package controller

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/forked/util"
	"github.com/sapcc/kube-parrot/pkg/forked/workqueue"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/tools/cache"
)

const (
	AnnotationBGPAnnouncement = "parrot.sap.cc/announce"
	KubeProxyPrefix           = "kube-proxy"
)

type ExternalServicesController struct {
	queue workqueue.RateLimitingInterface

	bgp *bgp.Server

	services  cache.Store
	endpoints cache.Store
	proxies   cache.Store

	waitForCacheSync func(stopCh <-chan struct{})
}

func NewExternalServicesController(endpointInformer informer.EndpointInformer,
	serviceInformer informer.ServiceInformer, podInformer informer.PodInformer,
	bgp *bgp.Server) *ExternalServicesController {

	c := &ExternalServicesController{
		bgp: bgp,

		services:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		endpoints: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		proxies:   cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "externalips"),
	}

	c.waitForCacheSync = func(stopCh <-chan struct{}) {
		cache.WaitForCacheSync(stopCh, serviceInformer.Informer().HasSynced,
			endpointInformer.Informer().HasSynced, podInformer.Informer().HasSynced)
	}

	endpointInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.endpointsAdd,
		UpdateFunc: c.endpointsUpdate,
		DeleteFunc: c.endpointsDelete,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.podAdd,
		UpdateFunc: c.podUpdate,
		DeleteFunc: c.podDelete,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.serviceAdd,
		UpdateFunc: c.serviceUpdate,
		DeleteFunc: c.serviceDelete,
	})

	return c
}

func (c *ExternalServicesController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer c.queue.ShutDown()
	wg.Add(1)

	c.waitForCacheSync(stopCh)

	go wait.Until(c.worker, time.Second, stopCh)

	<-stopCh
}

func (c *ExternalServicesController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *ExternalServicesController) processNextWorkItem() bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(obj)

	if c.reconcile() == nil {
		c.queue.Forget(obj)
		return true
	}

	c.queue.AddRateLimited(obj)
	return true
}

func (c *ExternalServicesController) setDirty() {
	c.queue.AddRateLimited("dirty")
}

func (c *ExternalServicesController) deleteRoute(route bgp.Route) error {
	if _, exists, _ := c.bgp.Routes.Get(route); exists {
		fmt.Printf("Withdrawing %s\n", route)
		return c.bgp.Routes.Delete(route)
	}

	return nil
}

func (c *ExternalServicesController) addRoute(service *v1.Service, proxy *v1.Pod) error {
	route := bgp.NewExternalIPRoute(service, proxy)

	if _, exists, _ := c.bgp.Routes.Get(route); !exists {
		fmt.Printf("Announcing %s\n", route)
		return c.bgp.Routes.Add(route)
	}

	return nil
}

func (c *ExternalServicesController) podDelete(pod interface{}) {
	if _, exists, _ := c.proxies.Get(pod); exists {
		c.proxies.Delete(pod)
		c.setDirty()
	}
}

func (c *ExternalServicesController) podAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	if !strings.HasPrefix(pod.Name, KubeProxyPrefix) {
		return
	}

	if util.IsPodReady(pod) {
		if _, exists, _ := c.proxies.Get(pod); !exists {
			c.proxies.Add(pod)
			c.setDirty()
		}
	} else {
		if _, exists, _ := c.proxies.Get(pod); exists {
			c.proxies.Delete(pod)
			c.setDirty()
		}
	}

}

func (c *ExternalServicesController) podUpdate(old, cur interface{}) {
	c.podAdd(cur)
}

func (c *ExternalServicesController) serviceDelete(service interface{}) {
	c.services.Delete(service)
	c.setDirty()
}

func (c *ExternalServicesController) serviceAdd(obj interface{}) {
	service := obj.(*v1.Service)
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

	if _, exists, _ := c.services.Get(service); !exists {
		c.services.Add(service)
		c.setDirty()
	}
}

func (c *ExternalServicesController) serviceUpdate(old, cur interface{}) {
	c.serviceAdd(cur)
}

func (c *ExternalServicesController) endpointsDelete(endpoints interface{}) {
	if _, exists, _ := c.endpoints.Get(endpoints); exists {
		c.endpoints.Delete(endpoints)
		c.setDirty()
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
		if _, exists, _ := c.endpoints.Get(endpoints); !exists {
			c.endpoints.Add(endpoints)
			c.setDirty()
		}
	} else {
		if _, exists, _ := c.endpoints.Get(endpoints); exists {
			c.endpoints.Delete(endpoints)
			c.setDirty()
		}
	}
}

func (c *ExternalServicesController) endpointsUpdate(old, cur interface{}) {
	c.endpointsAdd(cur)
}

func (c *ExternalServicesController) reconcile() error {
	for _, route := range c.bgp.Routes.List(bgp.EXTERNAL_IP) {
		if _, ok, _ := c.proxies.Get(route.Target); !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}

		if _, ok, _ := c.services.Get(route.Source); !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}

		if _, ok, _ := c.endpoints.Get(route.Source); !ok {
			if err := c.deleteRoute(route); err != nil {
				return err
			}
		}
	}

	for _, proxy := range c.proxies.List() {
		for _, service := range c.services.List() {
			if _, ok, _ := c.endpoints.Get(service); ok {
				if err := c.addRoute(service.(*v1.Service), proxy.(*v1.Pod)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
