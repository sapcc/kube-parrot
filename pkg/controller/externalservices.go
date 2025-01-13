package controller

import (
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	reconciler "github.com/sapcc/kube-parrot/pkg/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type ExternalServicesController struct {
	routes     *bgp.ExternalIPRoutesStore
	reconciler reconciler.DirtyReconcilerInterface
	hostIP     *net.IP
	nodeName   string

	services  cache.Store
	endpoints cache.Store
}

func NewExternalServicesController(informers informer.SharedInformerFactory,
	hostIP *net.IP, nodeName string, routes *bgp.ExternalIPRoutesStore) *ExternalServicesController {

	c := &ExternalServicesController{
		routes:    routes,
		hostIP:    hostIP,
		nodeName:  nodeName,
		services:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		endpoints: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
	}

	c.reconciler = reconciler.NewNamedDirtyReconciler("externalips", c.reconcile)

	_, err := informers.Endpoints().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.endpointsAdd,
		UpdateFunc: c.endpointsUpdate,
		DeleteFunc: c.endpointsDelete,
	})
	if err != nil {
		glog.V(3).Infof("adding endpoints event handler failed (%w), continuing anyway", err)
	}

	_, err = informers.Services().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.serviceAdd,
		UpdateFunc: c.serviceUpdate,
		DeleteFunc: c.serviceDelete,
	})
	if err != nil {
		glog.V(3).Infof("adding services event handler failed (%w), continuing anyway", err)
	}

	return c
}

func (c *ExternalServicesController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	c.reconciler.Run(stopCh)

	<-stopCh
}

func (c *ExternalServicesController) serviceDelete(obj interface{}) {
	service := obj.(*v1.Service)
	glog.V(3).Infof("Deleting Service (%s)", service.Name)
	err := c.services.Delete(service)
	if err != nil {
		glog.V(3).Infof("deleting service %s failed (%w), continuing anyway", service.Name, err)
	}
	c.reconciler.Dirty()
}

func (c *ExternalServicesController) serviceAdd(obj interface{}) {
	service := obj.(*v1.Service)
	if len(service.Spec.ExternalIPs) == 0 {
		glog.V(3).Infof("Skipping service %v. No externalIP defined...", service.GetName())
		return
	}

	_, exists, err := c.services.Get(service)
	if err != nil {
		glog.V(3).Infof("getting services failed (%w), continuing anyway", err)
	}
	if !exists {
		glog.V(3).Infof("Adding Service (%s)", service.Name)
		err := c.services.Add(service)
		if err != nil {
			glog.V(3).Infof("adding service %s failed (%w), continuing anyway", service.Name, err)
		}
	} else {
		err := c.services.Update(service) // update service object in cache
		if err != nil {
			glog.V(3).Infof("updating service %s failed (%w), continuing anyway", service.Name, err)
		}
	}
	c.reconciler.Dirty()
}

func (c *ExternalServicesController) serviceUpdate(old, cur interface{}) {
	c.serviceAdd(cur)
}

func (c *ExternalServicesController) endpointsDelete(obj interface{}) {
	endpoints := obj.(*v1.Endpoints)

	_, exists, err := c.endpoints.Get(endpoints)
	if err != nil {
		glog.V(3).Infof("getting nodes failed (%w), continuing anyway", err)
	}
	if exists {
		glog.V(3).Infof("deleting Endpoints (%s/%s)", endpoints.Namespace, endpoints.Name)
		err := c.endpoints.Delete(endpoints)
		if err != nil {
			glog.V(3).Infof("deleting endpoint %s/%s failed (%w), continuing anyway", endpoints.Namespace, endpoints.Name, err)
		}
		c.reconciler.Dirty()
	}
}

func (c *ExternalServicesController) endpointsAdd(obj interface{}) {
	endpoints := obj.(*v1.Endpoints)

	ready := false
	for _, v := range endpoints.Subsets {
		if len(v.Addresses) > 0 {
			ready = true
			break
		}
	}

	if ready {
		glog.V(5).Infof("Endpoint is ready (%s)", endpoints.Name)
		_, exists, err := c.endpoints.Get(endpoints)
		if err != nil {
			glog.V(3).Infof("getting endpoints failed (%w), continuing anyway", err)
		}
		if !exists {
			glog.V(3).Infof("Adding Endpoints (%s/%s)", endpoints.Namespace, endpoints.Name)
			err := c.endpoints.Add(endpoints)
			if err != nil {
				glog.V(3).Infof("adding endpoint %s/%s failed (%w), continuing anyway", endpoints.Namespace, endpoints.Name, err)
			}
			c.reconciler.Dirty()
		} else {
			err := c.endpoints.Update(endpoints) // update the endpoints object in the cache
			if err != nil {
				glog.V(3).Infof("deleting endpoint %s/%s failed (%w), continuing anyway", endpoints.Namespace, endpoints.Name, err)
			}
		}
	} else {
		if !strings.HasSuffix(endpoints.Name, "kube-scheduler") &&
			!strings.HasSuffix(endpoints.Name, "kube-controller-manager") {
			glog.V(5).Infof("Endpoint is NOT ready (%s)", endpoints.Name)
		}
		_, exists, err := c.endpoints.Get(endpoints)
		if err != nil {
			glog.V(3).Infof("getting endpoints failed (%w), continuing anyway", err)
		}
		if exists {
			glog.V(3).Infof("Deleting Endpoints (%s/%s)", endpoints.Namespace, endpoints.Name)
			err := c.endpoints.Delete(endpoints)
			if err != nil {
				glog.V(3).Infof("deleting endpoint %s/%s failed (%w), continuing anyway", endpoints.Namespace, endpoints.Name, err)
			}
			c.reconciler.Dirty()
		}
	}
}

func (c *ExternalServicesController) endpointsUpdate(old, cur interface{}) {
	c.endpointsAdd(cur)
	c.reconciler.Dirty()
}

func (c *ExternalServicesController) reconcile() error {
	for _, route := range c.routes.List() {
		obj, svcFound, err := c.services.Get(route.Service)
		if err != nil {
			glog.V(3).Infof("getting services failed (%w), continuing anyway", err)
		}
		if !svcFound {
			if err := c.routes.Delete(route); err != nil {
				return err
			}
			continue
		}
		svc := obj.(*v1.Service)

		eps, ok, err := c.endpoints.Get(svc)
		if err != nil {
			glog.V(3).Infof("getting endpoints failed (%w), continuing anyway", err)
		}
		if !ok {
			if err := c.routes.Delete(route); err != nil {
				return err
			}
		} else if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
			if !hasEndpointOnNode(c.nodeName, eps.(*v1.Endpoints)) {
				if err := c.routes.Delete(route); err != nil {
					return err
				}
			}
		}
	}

	for _, service := range c.services.List() {
		eps, ok, err := c.endpoints.Get(service)
		if err != nil {
			glog.V(3).Infof("getting endpoints failed (%w), continuing anyway", err)
		}
		if ok {
			svc := service.(*v1.Service)

			if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
				if hasEndpointOnNode(c.nodeName, eps.(*v1.Endpoints)) {
					if err := c.routes.Add(svc, c.hostIP); err != nil {
						return err
					}
				}
			} else {
				if err := c.routes.Add(svc, c.hostIP); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func hasEndpointOnNode(nodeName string, eps *v1.Endpoints) bool {
	for _, subset := range eps.Subsets {
		for _, address := range subset.Addresses {
			if *address.NodeName == nodeName {
				return true
			}
		}
	}
	return false
}
