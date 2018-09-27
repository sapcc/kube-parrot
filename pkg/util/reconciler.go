package reconciler

import (
	"time"

	"github.com/sapcc/kube-parrot/pkg/forked/workqueue"

	"k8s.io/apimachinery/pkg/util/wait"
)

type Interface interface {
	Reconcile() error
	Run(stopCh <-chan struct{})
}

type Type struct {
	queue     workqueue.RateLimitingInterface
	reconcile func() error
}

func (c *Type) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	go wait.Until(c.worker, time.Second, stopCh)

	<-stopCh
}

func (c *Type) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Type) processNextWorkItem() bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(obj)

	if c.Reconcile() == nil {
		c.queue.Forget(obj)
		return true
	}

	c.queue.AddRateLimited(obj)
	return true
}

func (c *Type) Reconcile() error {
	return c.reconcile()
}
