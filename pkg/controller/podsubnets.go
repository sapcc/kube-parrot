package controller

import (
	"net"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/util"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	reconciler "github.com/sapcc/kube-parrot/pkg/util"
)

type PodSubnetsController struct {
	routes     *bgp.NodePodSubnetRoutesStore
	nodes      cache.Store
	reconciler reconciler.DirtyReconcilerInterface
	hostIP     *net.IP
	podCIDR	   string
}

func NewPodSubnetsController(informers informer.SharedInformerFactory, hostIP *net.IP, podCIDR string,
	routes *bgp.NodePodSubnetRoutesStore) *PodSubnetsController {

	n := &PodSubnetsController{
		nodes:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		routes: routes,
		hostIP: hostIP,
		podCIDR: podCIDR,
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
	
	ip, err := util.GetNodeInternalIP(node)
	if err != nil {
		glog.Errorf("Node (%s) doesn't have an internal ip. Skipping.", node.Name)	
	}
	
	if ip != c.hostIP.String() {
		return
	}

	if c.podCIDR == "" {
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
		if err := c.routes.Add(node.(*v1.Node), c.podCIDR); err != nil {
			return err
		}
	}

	return nil
}
