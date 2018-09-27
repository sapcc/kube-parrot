package controller

import (
	"net"
	"strings"
	"sync"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/forked/util"
	"github.com/sapcc/kube-parrot/pkg/types"
	"github.com/sapcc/kube-parrot/pkg/util"
)

type ServiceSubnetController struct {
	routes        *bgp.NodeServiceSubnetRoutesStore
	reconciler    reconciler.DirtyReconcilerInterface
	hostIP        net.IP
	serviceSubnet net.IPNet

	proxies cache.Store
}

func NewServiceSubnetController(informers informer.SharedInformerFactory,
	serviceSubnet net.IPNet, hostIP net.IP, routes *bgp.NodeServiceSubnetRoutesStore) *ServiceSubnetController {

	c := &ServiceSubnetController{
		proxies:       cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		routes:        routes,
		hostIP:        hostIP,
		serviceSubnet: serviceSubnet,
	}

	c.reconciler = reconciler.NewNamedDirtyReconciler("servicesubnet", c.reconcile)

	informers.Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.podAdd,
		UpdateFunc: c.podUpdate,
		DeleteFunc: c.podDelete,
	})

	return c
}

func (c *ServiceSubnetController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	c.reconciler.Run(stopCh)

	<-stopCh
}

func (c *ServiceSubnetController) podDelete(obj interface{}) {
	pod := obj.(*v1.Pod)
	if _, exists, _ := c.proxies.Get(pod); exists {
		glog.V(3).Infof("Deleting Proxy (%s)", pod.Name)
		c.proxies.Delete(pod)
		c.reconciler.Dirty()
	}
}

func (c *ServiceSubnetController) podAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	if !strings.HasPrefix(pod.Name, types.KubeProxyPrefix) ||
		pod.Namespace != types.KubeProxyNamespace {
		return
	}

	if pod.Status.HostIP != c.hostIP.To4().String() {
		return
	}

	if util.IsPodReady(pod) {
		glog.V(5).Infof("Proxy is ready (%s)", pod.Name)
		if _, exists, _ := c.proxies.Get(pod); !exists {
			glog.V(3).Infof("Adding Proxy (%s)", pod.Name)
			c.proxies.Add(pod)
			c.reconciler.Dirty()
		}
	} else {
		glog.V(5).Infof("Proxy is NOT ready (%s)", pod.Name)
		if _, exists, _ := c.proxies.Get(pod); exists {
			glog.V(3).Infof("Deleting Proxy (%s)", pod.Name)
			c.proxies.Delete(pod)
			c.reconciler.Dirty()
		}
	}
}

func (c *ServiceSubnetController) podUpdate(old, cur interface{}) {
	c.podAdd(cur)
}

func (c *ServiceSubnetController) reconcile() error {
	for _, route := range c.routes.List() {
		if _, ok, _ := c.proxies.Get(route.Proxy); !ok {
			if err := c.routes.Delete(route); err != nil {
				return err
			}
		}
	}

	for _, proxy := range c.proxies.List() {
		if err := c.routes.Add(proxy.(*v1.Pod), c.serviceSubnet); err != nil {
			return err
		}
	}

	return nil
}
