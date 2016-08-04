package controller

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/sapcc/kube-parrot/pkg/bgp"
)

type PodSubnetsController struct {
	client *clientset.Clientset

	store      cache.Store
	controller *framework.Controller
	bgp        *bgp.Server
}

func NewPodSubnetsController(client *clientset.Clientset, bgp *bgp.Server) *PodSubnetsController {
	n := &PodSubnetsController{
		client: client,
		bgp:    bgp,
	}

	n.store, n.controller = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return n.client.Core().Nodes().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return n.client.Core().Nodes().Watch(options)
			},
		},
		&api.Node{},
		controller.NoResyncPeriodFunc(),
		framework.ResourceEventHandlerFuncs{
			AddFunc:    n.addNode,
			DeleteFunc: n.deleteNode,
		},
	)

	return n
}

func (n *PodSubnetsController) Run(stopCh <-chan struct{}) {
	n.controller.Run(stopCh)
}

func (n *PodSubnetsController) addNode(obj interface{}) {
	node := obj.(*api.Node)
	glog.V(3).Infof("Node created: %s", node.GetName())

	route, err := getPodSubnetRoute(node)
	if err != nil {
		glog.Warningf("Couldn't add pod subnet for %s: %s", node.GetName(), err)
		return
	}

	n.bgp.AddRoute(route)
}

func (n *PodSubnetsController) deleteNode(obj interface{}) {
	node := obj.(*api.Node)
	glog.V(3).Infof("Node deleted: %s", node.GetName())

	route, err := getPodSubnetRoute(node)
	if err != nil {
		glog.Warningf("Couldn't add pod subnet for %s: %s", node.GetName(), err)
		return
	}

	n.bgp.DeleteRoute(route)
}
