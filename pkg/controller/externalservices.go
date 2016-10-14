package controller

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/tools/cache"
)

type ExternalServicesController struct {
	client *kubernetes.Clientset
	bgp    *bgp.Server

	queue  *cache.FIFO
	synced func() bool

	endpoints           cache.Store
	endpointsController *cache.Controller
	services            cache.Store
	servicesController  *cache.Controller
	nodes               cache.Store
	nodesController     *cache.Controller
}

const (
	StoreSyncedPollPeriod     = 100 * time.Millisecond
	AnnotationBGPAnnouncement = "parrot.sap.cc/announce"
)

func NewExternalServicesController(client *kubernetes.Clientset, bgp *bgp.Server) *ExternalServicesController {
	c := &ExternalServicesController{
		client: client,
		bgp:    bgp,
		queue: cache.NewFIFO(func(obj interface{}) (string, error) {
			return obj.(string), nil
		}),
	}

	handlerFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(old, cur interface{}) {
			c.enqueue(cur)
		},
		DeleteFunc: c.enqueue,
	}

	c.endpoints, c.endpointsController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return c.client.Endpoints(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return c.client.Endpoints(api.NamespaceAll).Watch(options)
			},
		},
		&v1.Endpoints{},
		NoResyncPeriodFunc(),
		handlerFuncs,
	)

	c.services, c.servicesController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return c.client.Services(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return c.client.Services(api.NamespaceAll).Watch(options)
			},
		},
		&v1.Service{},
		NoResyncPeriodFunc(),
		handlerFuncs,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	c.nodes, c.nodesController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labels.SelectorFromSet(labels.Set{"zone": "farm"})
				return c.client.Nodes().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labels.SelectorFromSet(labels.Set{"zone": "farm"})
				return c.client.Nodes().Watch(options)
			},
		},
		&v1.Node{},
		NoResyncPeriodFunc(),
		cache.ResourceEventHandlerFuncs{},
	)

	c.synced = func() bool {
		return c.servicesController.HasSynced() &&
			c.nodesController.HasSynced() &&
			c.endpointsController.HasSynced()
	}

	return c
}

// addEndpoint          --> for proxies { announce(endpoint.getService().ExternalIP, proxy) }
// deleteEndpoint       --> for proxies { withdraw(endpoint.getService().ExternalIP, proxy) }
// updateEndpoint       --> addEndpoint(endpoint)
//
// addService           --> for proxies { announce(service.ExternalIP, proxy) }
// deleteService        --> for proxies { withdraw(service.ExternalIP, proxy) }
// updateService        --> addService(service)
//
// addProxyEndpoint     --> for services { announce(service.ExternalIP, proxy) }
// deleteProxyEndpoint  --> for services { withdraw(service.ExternalIP, proxy) }
// updateProxyEndpoint  --> addProxyEndpoint(proxy)


func (c *ExternalServicesController) enqueue(obj interface{}) {
	if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
		c.queue.Add(key)
	}
}

func (c *ExternalServicesController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	go c.servicesController.Run(stopCh)
	go c.nodesController.Run(stopCh)
	go c.endpointsController.Run(stopCh)
	go wait.Until(c.worker, time.Second, stopCh)

	<-stopCh
}

func (c *ExternalServicesController) worker() {
	for !c.synced() {
		time.Sleep(StoreSyncedPollPeriod)
		glog.V(3).Infof("Waiting for controllers to be synced")
	}

	for {
		if r, err := c.queue.Pop(c.syncService); err != nil {
			c.queue.AddIfNotPresent(r)
		}
	}

}

func (c *ExternalServicesController) syncService(obj interface{}) error {
	key := obj.(string)
	startTime := time.Now()

	serviceObj, exists, err := c.services.GetByKey(key)
	if err != nil || !exists {
		glog.V(3).Infof("Service does not exist anymore. Doing nothing.: %v", key)
		// Could this be a problem when a service is deleted?
		// We could possibly have an announcement in the bgp session but don't know its
		// IP anymore here, so we can't delete it without keeping some state.
		return nil
	}
	service := serviceObj.(*v1.Service)

	if l, ok := service.Annotations[AnnotationBGPAnnouncement]; ok {
		announcementRequested, err := strconv.ParseBool(l)
		if err != nil {
			glog.Errorf("Failed to parse annotation %v: %v", AnnotationBGPAnnouncement, err)
			return nil
		}

		if !announcementRequested {
			glog.V(3).Infof("Skipping service %v. Annotation is set but not true. Huh?", key)
			return nil
		}
	} else {
		glog.V(3).Infof("Skipping service %v. No announce annotation defined...", key)
		return nil
	}

	if len(service.Spec.ExternalIPs) == 0 {
		glog.V(3).Infof("Skipping service %v. No externalIP defined...", key)
		return nil
	}

	endpointsObj, exists, err := c.endpoints.GetByKey(key)
	if err != nil || !exists {
		glog.Infof("Service without endpoints: %s", key)
		c.deleteRoute(service)
		return nil
	}

	endpoints := endpointsObj.(*v1.Endpoints)
	ready := false
	for _, v := range endpoints.Subsets {
		if len(v.Addresses) > 0 {
			ready = true
			break
		}
	}
	if ready {
		glog.Infof("Service up: %s", service.GetName())
		c.addRoute(service)
	} else {
		glog.Infof("Service down: %s", service.GetName())
		c.deleteRoute(service)
	}

	glog.V(3).Infof("Finished syncing service %q (%v)", key, time.Now().Sub(startTime))
	return nil
}

func (c *ExternalServicesController) reconcileServices(obj interface{}) {
	for _, service := range c.services.List() {
		c.enqueue(&service)
	}
}

func (c *ExternalServicesController) addRoute(service *v1.Service) {
	nodes := c.nodes.List()

	for _, ip := range service.Spec.ExternalIPs {
		for _, node := range nodes {
			var nodeIP net.IP
			for _, address := range node.(*v1.Node).Status.Addresses {
				if address.Type == v1.NodeInternalIP {
					nodeIP = net.ParseIP(address.Address)
				}
			}

			if nodeIP == nil {
				glog.Errorf("Couldn't get IP for node: %s", node)
			}

			route := getExternalIPRoute(net.ParseIP(ip), nodeIP, false)
			fmt.Printf("Adding %s\n", route)
			c.bgp.AddPath(route)
		}
	}
}

func (c *ExternalServicesController) deleteRoute(service *v1.Service) {
	nodes := c.nodes.List()

	for _, ip := range service.Spec.ExternalIPs {
		for _, node := range nodes {
			var nodeIP net.IP
			for _, address := range node.(*v1.Node).Status.Addresses {
				if address.Type == v1.NodeInternalIP {
					nodeIP = net.ParseIP(address.Address)
				}
			}

			if nodeIP == nil {
				glog.Errorf("Couldn't get IP for node: %s", node)
			}

			route := getExternalIPRoute(net.ParseIP(ip), nodeIP, true)
			fmt.Printf("Deleting %s\n", route)
			c.bgp.DeletePath(route)
		}
	}
}
