package controller

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/workqueue"
	"github.com/sapcc/kube-parrot/pkg/informer"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/tools/cache"
	"k8s.io/kubernetes/pkg/types"
)

const (
	AnnotationBGPAnnouncement = "parrot.sap.cc/announce"
)

type ExternalServicesController struct {
	routes *RoutesConfig
	queue  workqueue.RateLimitingInterface

	serviceStore  *informer.StoreToServiceLister
	endpointStore *informer.StoreToEndpointsLister
	nodeStore     *informer.StoreToNodeLister

	serviceStoreSynced  cache.InformerSynced
	endpointStoreSynced cache.InformerSynced
	nodeStoreSynced     cache.InformerSynced
}

type Operation int

const (
	ADD Operation = iota
	DEL
)

type Command struct {
	resource interface{}
	Op       Operation
}

func NewExternalServicesController(
	endpointInformer informer.EndpointInformer,
	serviceInformer informer.ServiceInformer,
	nodeInformer informer.NodeInformer,
	bgp *bgp.Server) *ExternalServicesController {

	c := &ExternalServicesController{
		routes: NewRoutesConfig(bgp),
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"externalips"),
		endpointStore:       endpointInformer.Lister(),
		endpointStoreSynced: endpointInformer.Informer().HasSynced,
		serviceStore:        serviceInformer.Lister(),
		serviceStoreSynced:  serviceInformer.Informer().HasSynced,
		nodeStore:           nodeInformer.Lister(),
		nodeStoreSynced:     nodeInformer.Informer().HasSynced,
	}

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			c.queue.Add(Command{cur, ADD})
		},
		UpdateFunc: func(old, cur interface{}) {
			c.queue.Add(Command{cur, ADD})
		},
		DeleteFunc: func(cur interface{}) {
			c.queue.Add(Command{cur, DEL})
		},
	}

	endpointInformer.Informer().AddEventHandler(handlers)
	serviceInformer.Informer().AddEventHandler(handlers)
	nodeInformer.Informer().AddEventHandler(handlers)

	return c
}

func (c *ExternalServicesController) findExternalIPsFromService(service *v1.Service) []net.IP {
	if l, ok := service.Annotations[AnnotationBGPAnnouncement]; ok {
		announcementRequested, err := strconv.ParseBool(l)
		if err != nil {
			glog.Errorf("Failed to parse annotation %v: %v", AnnotationBGPAnnouncement, err)
			return []net.IP{}
		}

		if !announcementRequested {
			glog.V(3).Infof("Skipping service %v. Annotation is set but not true. Huh?", service.GetName())
			return []net.IP{}
		}
	} else {
		glog.V(3).Infof("Skipping service %v. No announce annotation defined...", service.GetName())
		return []net.IP{}
	}

	result := []net.IP{}
	for _, ip := range service.Spec.ExternalIPs {
		result = append(result, net.ParseIP(ip))
	}

	return result
}

func (c *ExternalServicesController) findExternalIPsFromEndpoint(endpoint *v1.Endpoints) []net.IP {
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(endpoint)

	serviceObj, exists, err := c.serviceStore.Indexer.GetByKey(key)
	if err != nil || !exists {
		glog.V(3).Infof("Service does not exist anymore. Doing nothing.: %v", key)
		return []net.IP{}
	}
	service := serviceObj.(*v1.Service)

	return c.findExternalIPsFromService(service)
}

func (c *ExternalServicesController) findProxyIPs() []net.IP {
	nodes, _ := c.nodeStore.List()
	result := []net.IP{}

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeInternalIP {
				result = append(result, net.ParseIP(address.Address))
			}
		}
	}

	return result
}

func (c *ExternalServicesController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.queue.ShutDown()
	wg.Add(1)

	if !cache.WaitForCacheSync(stopCh, c.endpointStoreSynced, c.serviceStoreSynced, c.nodeStoreSynced) {
		return
	}
	go wait.Until(c.worker, time.Second, stopCh)

	<-stopCh
}

func (c *ExternalServicesController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *ExternalServicesController) processNextWorkItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup something in a cache
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	cmd := obj.(Command)

	// you always have to indicate to the queue that you've completed a piece of work
	defer c.queue.Done(cmd)

	// do your work on the key.  This method will contains your "do stuff" logic"
	err := c.executeCommand(cmd)

	// there was a failure so be sure to report it.  This method allows for pluggable error handling
	// which can be used for things like cluster-monitoring
	if err == nil {
		c.queue.Forget(cmd)
		return true
	}

	glog.Errorf("Failed to execute command %s: %v", cmd, err)
	c.queue.AddRateLimited(cmd)

	return true
}

func (c *ExternalServicesController) executeCommand(command Command) error {
	switch command.resource.(type) {
	case *v1.Node:
		node := command.resource.(*v1.Node)
		switch command.Op {
		case ADD:
			c.routes.OnNodeAdd(node)
		case DEL:
			c.routes.OnNodeDelete(node)
		}
	case *v1.Endpoints:
		endpoints := command.resource.(*v1.Endpoints)
		switch command.Op {
		case ADD:
			c.routes.OnEndpointsAdd(endpoints)
		case DEL:
			c.routes.OnEndpointsDelete(endpoints)
		}
	case *v1.Service:
		service := command.resource.(*v1.Service)
		switch command.Op {
		case ADD:
			c.routes.OnServiceAdd(service)
		case DEL:
			c.routes.OnServiceDelete(service)
		}
	}

	return c.routes.reconcile()
}

type RoutesConfig struct {
	bgp       *bgp.Server
	routes    map[Route]Route
	nodes     map[string]*v1.Node
	services  map[types.NamespacedName]*v1.Service
	endpoints map[types.NamespacedName]*v1.Endpoints
}

type Route struct {
	Service    types.NamespacedName
	Node       string
	externalIP string
	nextHop    string
}

func NewRoutesConfig(bgp *bgp.Server) *RoutesConfig {
	return &RoutesConfig{
		bgp:       bgp,
		routes:    map[Route]Route{},
		nodes:     map[string]*v1.Node{},
		services:  map[types.NamespacedName]*v1.Service{},
		endpoints: map[types.NamespacedName]*v1.Endpoints{},
	}
}

func (c *RoutesConfig) deleteRoute(route Route) error {
	fmt.Printf("Withdrawing %s on %s: %s --> %s\n", route.Service, route.Node, route.externalIP, route.nextHop)
	if err := c.bgp.DeleteRoute(route.externalIP, route.nextHop); err != nil {
		return fmt.Errorf("Failed to delte route %v -> %v. Withdrawal failed: %v", route.Service, route.Node, err)
	}
	delete(c.routes, route)
	return nil
}

func (c *RoutesConfig) addRoute(service *v1.Service, node *v1.Node) error {
	serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}

	nodeInternalIP := ""
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			nodeInternalIP = address.Address
		}
	}

	if nodeInternalIP == "" {
		return fmt.Errorf("Failed to add route %v -> %v: Couldn't get internal Node IP", serviceName, node.Name)
	}

	route := Route{serviceName, node.Name, service.Spec.ExternalIPs[0], nodeInternalIP}
	if _, ok := c.routes[route]; !ok {
		fmt.Printf("Announcing %s on %s: %s --> %s\n", route.Service, route.Node, route.externalIP, route.nextHop)
		if err := c.bgp.AddRoute(route.externalIP, route.nextHop); err != nil {
			return fmt.Errorf("Failed to add route %v -> %v. Announcement failed: %v", serviceName, node.Name, err)
		}
		c.routes[route] = route
	}

	return nil
}

func (c *RoutesConfig) OnNodeDelete(node *v1.Node) {
	delete(c.nodes, node.Name)
}

func (c *RoutesConfig) OnNodeAdd(node *v1.Node) {
	c.nodes[node.Name] = node
}

func (c *RoutesConfig) OnServiceDelete(service *v1.Service) {
	serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	delete(c.services, serviceName)
}

func (c *RoutesConfig) OnServiceAdd(service *v1.Service) {
	if l, ok := service.Annotations[AnnotationBGPAnnouncement]; ok {
		announcementRequested, err := strconv.ParseBool(l)
		if err != nil {
			glog.Errorf("Failed to parse annotation %v: %v", AnnotationBGPAnnouncement, err)
			return
		}

		if !announcementRequested {
			glog.V(3).Infof("Skipping service %v. Annotation is set but not true. Huh?", service.GetName())
			return
		}
	} else {
		glog.V(5).Infof("Skipping service %v. No announce annotation defined...", service.GetName())
		return
	}

	if len(service.Spec.ExternalIPs) == 0 {
		glog.V(3).Infof("Skipping service %v. No externalIP defined...", service.GetName())
		return
	}

	serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	c.services[serviceName] = service
}

func (c *RoutesConfig) OnEndpointsDelete(endpoints *v1.Endpoints) {
	endpointsName := types.NamespacedName{Namespace: endpoints.Namespace, Name: endpoints.Name}
	delete(c.endpoints, endpointsName)
}

func (c *RoutesConfig) OnEndpointsAdd(endpoints *v1.Endpoints) {
	endpointsName := types.NamespacedName{Namespace: endpoints.Namespace, Name: endpoints.Name}

	ready := false
	for _, v := range endpoints.Subsets {
		if len(v.Addresses) > 0 {
			ready = true
			break
		}
	}

	if ready {
		c.endpoints[endpointsName] = endpoints
	} else {
		delete(c.endpoints, endpointsName)
	}
}

func (c *RoutesConfig) reconcile() error {
	for _, route := range c.routes {
		if _, ok := c.nodes[route.Node]; !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}

		serviceName := types.NamespacedName{Namespace: route.Service.Namespace, Name: route.Service.Name}
		if _, ok := c.services[serviceName]; !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}

		if _, ok := c.endpoints[serviceName]; !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}
	}

	for _, node := range c.nodes {
		for _, service := range c.services {
			serviceName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
			if _, ok := c.endpoints[serviceName]; ok {
				if err := c.addRoute(service, node); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
