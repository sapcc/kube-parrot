package controller

//import (
//  "sync"
//  "time"

//  "github.com/golang/glog"
//  "k8s.io/client-go/1.5/pkg/api/v1"
//  "k8s.io/client-go/1.5/tools/cache"
//  "k8s.io/kubernetes/pkg/types"
//  "k8s.io/kubernetes/pkg/util/wait"

//  "github.com/sapcc/kube-parrot/pkg/bgp"
//  "github.com/sapcc/kube-parrot/pkg/forked/informer"
//  "github.com/sapcc/kube-parrot/pkg/forked/workqueue"
//)

//type PodSubnetsController struct {
//  queue workqueue.RateLimitingInterface

//  nodeStore       *informer.StoreToNodeLister
//  nodeStoreSynced cache.InformerSynced

//  bgp *bgp.Server
//}

//func NewPodSubnetsController(nodeInformer informer.NodeInformer, bgp *bgp.Server) *PodSubnetsController {
//  n := &PodSubnetsController{
//    nodeStore:       nodeInformer.Lister(),
//    nodeStoreSynced: nodeInformer.Informer().HasSynced,
//    bgp:             bgp,
//  }

//  nodeInformer.Informer().AddEventHandler(
//    cache.ResourceEventHandlerFuncs{
//      AddFunc:    n.addNode,
//      DeleteFunc: n.deleteNode,
//    },
//  )

//  return n
//}

//func (n *PodSubnetsController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
//  defer wg.Done()
//  defer c.queue.ShutDown()
//  wg.Add(1)

//  if !cache.WaitForCacheSync(stopCh, c.nodeStoreSynced) {
//    return
//  }

//  go wait.Until(c.worker, time.Second, stopCh)

//  <-stopCh
//}

//func (c *PodSubnetsController) worker() {
//  for c.processNextWorkItem() {
//  }
//}

//func (c *PodSubnetsController) processNextWorkItem() bool {
//  // pull the next work item from queue.
//  obj, quit := c.queue.Get()
//  if quit {
//    return false
//  }
//  cmd := obj.(Command)

//  // you always have to indicate to the queue that you've completed a piece of work
//  defer c.queue.Done(cmd)

//  // do your work on the key.  This method will contains your "do stuff" logic"
//  err := c.executeCommand(cmd)

//  // there was a failure so be sure to report it.  This method allows for pluggable error handling
//  // which can be used for things like cluster-monitoring
//  if err == nil {
//    c.queue.Forget(cmd)
//    return true
//  }

//  glog.Errorf("Failed to execute command %s: %v", cmd, err)
//  c.queue.AddRateLimited(cmd)

//  return true
//}

//func (c *PodSubnetsController) executeCommand(command Command) error {
//  switch command.resource.(type) {
//  case *v1.Node:
//    node := command.resource.(*v1.Node)
//    switch command.Op {
//    case ADD:
//      c.routes.OnNodeAdd(node)
//    case DEL:
//      c.routes.OnNodeDelete(node)
//    }
//  }

//  return c.reconcile()
//}

//type PodSubnetRoutesConfig struct {
//  bgp    *bgp.Server
//  routes map[PodSubnetRoute]PodSubnetRoute
//  nodes  map[types.NamespacedName]*v1.Node
//}

//type PodSubnetRoute struct {
//  Node   string
//  HostIP string
//  Subnet string
//}

//func NewPodSubnetRoutesConfig(bgp *bgp.Server) *PodSubnetRoutesConfig {
//  return &PodSubnetRoutesConfig{
//    bgp:    bgp,
//    routes: map[PodSubnetRoutesConfig]PodSubnetRoutesConfig{},
//    nodes:  map[types.NamespacedName]*v1.Node{},
//  }
//}

//func (n *PodSubnetRoutesConfig) OnNodeAdd(node *v1.Node) {
//  n.nodes[node.Name] = node
//  //route, err := getPodSubnetRoute(node)
//  //if err != nil {
//  //  glog.Warningf("Couldn't add pod subnet for %s: %s", node.GetName(), err)
//  //  return
//  //}

//  //fmt.Printf("Adding %s\n", route)
//  //n.bgp.AddPath(route)
//}

//func (n *PodSubnetRoutesConfig) OnNodeDelete(node *v1.Node) {
//  delete(n.nodes, node.Name)
//}

//func (c *PodSubnetRoutesConfig) reconcile() error {
//  for _, route := range c.routes {
//    if _, ok := c.nodes; !ok {
//      if err := c.deleteRoute(route); err != nil {
//        return err
//      }
//    }
//  }

//  for _, node := range c.nodes {
//    if err := c.addRoute(route); err != nil {
//      return err
//    }
//  }
//}

//func (c *PodSubnetRoutesConfig) deleteRoute(route PodSubnetRoute) error {
//  fmt.Printf("Withdrawing PodSubnet of %s: %s --> %s\n", route.Node, route.Subnet, route.HostIP)
//  if err := c.bgp.DeleteRoute(route.externalIP, route.nextHop); err != nil {
//    return err
//  }
//  delete(c.routes, route)
//  return nil
//}


