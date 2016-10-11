package controller

import (
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/1.4/pkg/api"
	
)

type ExternalServicesController struct {
	client *clientset.Clientset
	queue  *workqueue.Type
	synced func() bool

	services           cache.StoreToServiceLister
	servicesController *framework.Controller
	nodes              cache.StoreToNodeLister
	nodesController    *framework.Controller
}

var (
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc
)

const (
	StoreSyncedPollPeriod = 100 * time.Millisecond
)

func NewExternalServicesController(client *clientset.Clientset) *ExternalServicesController {
	c := &ExternalServicesController{
		client: client,
		queue:  workqueue.New(),
	}

	c.services.Store, c.servicesController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return c.client.Core().Services(api.NamespaceAll).List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return c.client.Core().Services(api.NamespaceAll).Watch(options)
			},
		},
		&api.Service{},
		controller.NoResyncPeriodFunc(),
		framework.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueService,
			UpdateFunc: func(old, cur interface{}) {
				c.enqueueService(cur)
			},
			DeleteFunc: c.enqueueService,
		},
	)

	c.nodes.Store, c.nodesController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return c.client.Core().Nodes().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return c.client.Core().Nodes().Watch(options)
			},
		},
		&api.Node{},
		controller.NoResyncPeriodFunc(),
		framework.ResourceEventHandlerFuncs{
			AddFunc:    c.reconcileServices,
			DeleteFunc: c.reconcileServices,
		},
	)

	c.synced = func() bool {
		return c.servicesController.HasSynced() && c.nodesController.HasSynced()
	}

	return c
}

func (c *ExternalServicesController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	go c.servicesController.Run(stopCh)
	go c.nodesController.Run(stopCh)

	for i := 0; i < 2; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	c.queue.ShutDown()
}

func (c *ExternalServicesController) enqueueService(obj interface{}) {
	if service, ok := obj.(*api.Service); ok {
		if len(service.Spec.ExternalIPs) == 0 {
			return
		}
	} else {
		glog.Errorf("Couldn't get service from object: %+v", obj)
		return
	}

	key, err := keyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	glog.V(3).Infof("Service queued: %s", key)
	c.queue.Add(key)
}

func (c *ExternalServicesController) worker() {
	glog.V(3).Infof("Worker Started")
	for {
		func() {
			key, quit := c.queue.Get()
			if quit {
				return
			}
			defer c.queue.Done(key)
			c.syncService(key.(string))
		}()
	}
}

func (c *ExternalServicesController) syncService(key string) {
	startTime := time.Now()

	if !c.synced() {
		glog.V(3).Infof("Waiting for controllers to sync, requeuing service %v", key)
		time.Sleep(StoreSyncedPollPeriod)
		c.queue.Add(key)
		return
	}

	obj, exists, err := c.services.GetByKey(key)
	if err != nil || !exists {
		// Delete routes, as the service has been deleted.
		glog.V(3).Infof("Deleting Routes for Service %s", key)
		return
	}

	service := obj.(*api.Service)
	glog.V(3).Infof("Creating Routes for Service %s", service.GetName())

	glog.V(3).Infof("Finished syncing service %q (%v)", key, time.Now().Sub(startTime))
}

func (c *ExternalServicesController) reconcileServices(obj interface{}) {
	serviceList, err := c.services.List()
	if err != nil {
		glog.Errorf("Couldn't reconcile services: %s", err)
		return
	}

	for _, service := range serviceList.Items {
		c.enqueueService(&service)
	}
}
