package controller

import (
	"sync"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/types"
	"github.com/sapcc/kube-parrot/pkg/util"
)

type PodSubnetsController struct {
	routes     *bgp.NodePodSubnetRoutesStore
	nodes      cache.Store
	reconciler reconciler.DirtyReconcilerInterface
}

func NewPodSubnetsController(informers informer.SharedInformerFactory,
	routes *bgp.NodePodSubnetRoutesStore) *PodSubnetsController {

	n := &PodSubnetsController{
		nodes:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		routes: routes,
	}

	n.reconciler = reconciler.NewNamedDirtyReconciler("podsubnets", n.reconcile)

	informers.Nodes().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    n.nodeAdd,
			UpdateFunc: n.nodeUpdate,
			DeleteFunc: n.nodeDelete,
		},
	)

	return n
}

func (c *PodSubnetsController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	c.reconciler.Run(stopCh)

	<-stopCh
}

func (c *PodSubnetsController) nodeAdd(obj interface{}) {
	node := obj.(*v1.Node)

	if _, ok := node.Annotations[types.AnnotationNodePodSubnet]; !ok {
		if _, exists, _ := c.nodes.Get(node); exists {
			glog.V(3).Infof("Deleting Node (%s)", node.Name)
			c.nodes.Delete(node)
			c.reconciler.Dirty()
		}
		return
	}

	if _, exists, _ := c.nodes.Get(node); !exists {
		glog.V(3).Infof("Adding Node (%s)", node.Name)
		c.nodes.Add(node)
		c.reconciler.Dirty()
	}
}

func (c *PodSubnetsController) nodeUpdate(old, cur interface{}) {
	c.nodeAdd(cur.(*v1.Node))
}

func (c *PodSubnetsController) nodeDelete(obj interface{}) {
	node := obj.(*v1.Node)
	if _, exists, _ := c.nodes.Get(node); exists {
		c.nodes.Delete(node)
		c.reconciler.Dirty()
	}
}

func (c *PodSubnetsController) reconcile() error {
	for _, route := range c.routes.List() {
		if _, ok, _ := c.nodes.Get(route.Node); !ok {
			if err := c.routes.Delete(route); err != nil {
				return err
			}
		}
	}

	for _, node := range c.nodes.List() {
		if err := c.routes.Add(node.(*v1.Node)); err != nil {
			return err
		}
	}

	return nil
}
