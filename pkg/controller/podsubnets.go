package controller

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/tools/cache"

	"github.com/sapcc/kube-parrot/pkg/bgp"
)

type PodSubnetsController struct {
	client *kubernetes.Clientset

	store      cache.Store
	controller *cache.Controller
	bgp        *bgp.Server
}

func NewPodSubnetsController(client *kubernetes.Clientset, bgp *bgp.Server) *PodSubnetsController {
	n := &PodSubnetsController{
		client: client,
		bgp:    bgp,
	}

	n.store, n.controller = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return n.client.Core().Nodes().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return n.client.Core().Nodes().Watch(options)
			},
		},
		&v1.Node{},
		NoResyncPeriodFunc(),
		cache.ResourceEventHandlerFuncs{
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
	node := obj.(*v1.Node)
	glog.Infof("Node created: %s", node.GetName())

	route, err := getPodSubnetRoute(node)
	if err != nil {
		glog.Warningf("Couldn't add pod subnet for %s: %s", node.GetName(), err)
		return
	}

	fmt.Printf("Adding %s\n", route)
	n.bgp.AddPath(route)
}

func (n *PodSubnetsController) deleteNode(obj interface{}) {
	node := obj.(*v1.Node)
	glog.Infof("Node deleted: %s", node.GetName())

	route, err := getPodSubnetRoute(node)
	if err != nil {
		glog.Warningf("Couldn't add pod subnet for %s: %s", node.GetName(), err)
		return
	}

	fmt.Printf("Deleting %s\n", route)
	n.bgp.DeletePath(route)
}
