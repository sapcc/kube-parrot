package controller

import (
	"strings"
	"sync"

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

	pods       cache.Store
	apiservers cache.Store
}

func NewAPIServerController(informers informer.SharedInformerFactory,
	routes *bgp.APIServerRoutesStore) *APIServerController {

	c := &APIServerController{
		routes:     routes,
		pods:       cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		apiservers: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
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

	if util.IsPodReady(pod) {
		if _, exists, _ := c.apiservers.Get(pod); !exists {
			c.apiservers.Add(pod)
			c.reconciler.Dirty()
		}
	} else {
		if _, exists, _ := c.apiservers.Get(pod); exists {
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
