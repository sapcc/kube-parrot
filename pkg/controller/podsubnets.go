// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"net"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/util"
)

type PodSubnetsController struct {
	routes     *bgp.NodePodSubnetRoutesStore
	nodes      cache.Store
	reconciler util.DirtyReconcilerInterface
	hostIP     *net.IP
}

func NewPodSubnetsController(informers informer.SharedInformerFactory, hostIP *net.IP,
	routes *bgp.NodePodSubnetRoutesStore) *PodSubnetsController {

	n := &PodSubnetsController{
		nodes:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		routes: routes,
		hostIP: hostIP,
	}

	n.reconciler = util.NewNamedDirtyReconciler("podsubnets", n.reconcile)

	_, err := informers.Nodes().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    n.nodeAdd,
			UpdateFunc: n.nodeUpdate,
			DeleteFunc: n.nodeDelete,
		},
	)
	if err != nil {
		glog.V(3).Infof("adding node event handler failed (%w), continuing anyway", err)
	}

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

	if _, err := util.GetNodePodSubnet(node); err != nil {
		_, exists, err := c.nodes.Get(node)
		if err != nil {
			glog.V(3).Infof("getting nodes failed (%w), continuing anyway", err)
		}
		if exists {
			glog.V(3).Infof("Deleting Node (%s)", node.Name)
			err = c.nodes.Delete(node)
			if err != nil {
				glog.V(3).Infof("deleting node %s failed (%w), continuing anyway", node.Name, err)
			}
			c.reconciler.Dirty()
		}
		return
	}

	_, exists, err := c.nodes.Get(node)
	if err != nil {
		glog.V(3).Infof("getting nodes failed (%w), continuing anyway", err)
	}
	if !exists {
		glog.V(3).Infof("Adding Node (%s)", node.Name)
		err = c.nodes.Add(node)
		if err != nil {
			glog.V(3).Infof("adding node %s failed (%w), continuing anyway", node.Name, err)
		}
		c.reconciler.Dirty()
	}
}

func (c *PodSubnetsController) nodeUpdate(old, cur interface{}) {
	c.nodeAdd(cur.(*v1.Node))
}

func (c *PodSubnetsController) nodeDelete(obj interface{}) {
	node := obj.(*v1.Node)
	_, exists, err := c.nodes.Get(node)
	if err != nil {
		glog.V(3).Infof("getting nodes failed (%w), continuing anyway", err)
	}
	if exists {
		err := c.nodes.Delete(node)
		if err != nil {
			glog.V(3).Infof("deleting node %s failed (%w), continuing anyway", node.Name, err)
		}
		c.reconciler.Dirty()
	}
}

func (c *PodSubnetsController) reconcile() error {
	for _, route := range c.routes.List() {
		_, ok, err := c.nodes.Get(route.Node)
		if err != nil {
			glog.V(3).Infof("getting nodes failed (%w), continuing anyway", err)
		}
		if !ok {
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
