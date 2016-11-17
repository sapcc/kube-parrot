package controller

import (
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/forked/util"
	"github.com/sapcc/kube-parrot/pkg/util"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"
)

const (
	KubeApiserverNamespace = "kube-system"
	KubeApiserverPrefix    = "kubernetes-master"
)

type APIServerController struct {
	routes     *bgp.APIServerRoutesStore
	reconciler reconciler.DirtyReconcilerInterface
	hostIP     net.IP

	pods       cache.Store
	apiservers cache.Store
}

func NewAPIServerController(informers informer.SharedInformerFactory, hostIP net.IP,
	routes *bgp.APIServerRoutesStore) *APIServerController {

	c := &APIServerController{
		routes:     routes,
		pods:       cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		apiservers: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		hostIP:     hostIP,
	}

	c.reconciler = reconciler.NewNamedDirtyReconciler("apiserver", c.reconcile)

	informers.Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.podAdd,
		UpdateFunc: c.podUpdate,
		DeleteFunc: c.podDelete,
	})

	return c
}

func (c *APIServerController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	c.reconciler.Run(stopCh)

	<-stopCh
}

func (c *APIServerController) podDelete(obj interface{}) {
	pod := obj.(*v1.Pod)

	if _, exists, _ := c.apiservers.Get(pod); exists {
		glog.V(3).Infof("Deleting APIServer (%s)", pod.Name)
		c.apiservers.Delete(pod)
		c.reconciler.Dirty()
	}
}

func (c *APIServerController) podAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	if !strings.HasPrefix(pod.Name, KubeApiserverPrefix) ||
		pod.Namespace != KubeApiserverNamespace {
		return
	}

	if pod.Status.HostIP != c.hostIP.To4().String() {
		return
	}

	if util.IsPodReady(pod) {
		glog.V(5).Infof("APIServer is ready (%s)", pod.Name)
		if _, exists, _ := c.apiservers.Get(pod); !exists {
			glog.V(3).Infof("Adding APIServer (%s)", pod.Name)
			c.apiservers.Add(pod)
			c.reconciler.Dirty()
		}
	} else {
		glog.V(5).Infof("APIServer is NOT ready (%s)", pod.Name)
		if _, exists, _ := c.apiservers.Get(pod); exists {
			glog.V(3).Infof("Deleting APIServer (%s)", pod.Name)
			c.apiservers.Delete(pod)
			c.reconciler.Dirty()
		}
	}

}

func (c *APIServerController) podUpdate(old, cur interface{}) {
	c.podAdd(cur)
}

func (c *APIServerController) reconcile() error {
	for _, route := range c.routes.List() {
		if _, exists, _ := c.apiservers.Get(route.APIServer); !exists {
			if err := c.routes.Delete(route); err != nil {
				return err
			}
		}
	}

	for _, apiserver := range c.apiservers.List() {
		if err := c.routes.Add(apiserver.(*v1.Pod)); err != nil {
			return err
		}
	}

	return nil
}
