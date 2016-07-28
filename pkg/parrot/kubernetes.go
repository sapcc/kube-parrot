package parrot

import (
	"time"

	client "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	resyncPeriod = 1 * time.Minute
)

func (p *Parrot) createKubernetesClient() {
	glog.V(2).Infof("Creating Client")
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	p.handleError(err)

	client, err := client.New(config)
	p.handleError(err)

	p.client = client
	glog.V(3).Infof("  using %s", config.Host)
}

func (feeder *Parrot) watchServices() cache.Store {
	lw := cache.NewListWatchFromClient(feeder.client, "services", api.NamespaceAll, fields.Everything())
	hf := framework.ResourceEventHandlerFuncs{
		AddFunc: feeder.handleServiceCreate,
		UpdateFunc: func(oldObj, newObj interface{}) {
			feeder.handleServiceUpdate(oldObj, newObj)
		},
		DeleteFunc: feeder.handleServiceDelete,
	}

	store, controller := framework.NewInformer(lw, &api.Service{}, resyncPeriod, hf)

	go controller.Run(wait.NeverStop)
	return store
}

func (feeder *Parrot) watchEndpoints() cache.Store {
	lw := cache.NewListWatchFromClient(feeder.client, "endpoints", api.NamespaceAll, fields.Everything())
	hf := framework.ResourceEventHandlerFuncs{
		AddFunc: feeder.handleEndpointCreate,
		UpdateFunc: func(oldObj, newObj interface{}) {
			feeder.handleEndpointUpdate(oldObj, newObj)
		},
		DeleteFunc: feeder.handleEndpointDelete,
	}

	store, controller := framework.NewInformer(lw, &api.Endpoints{}, resyncPeriod, hf)

	go controller.Run(wait.NeverStop)
	return store
}

func (feeder *Parrot) watchNodes() cache.Store {
	lw := cache.NewListWatchFromClient(feeder.client, "nodes", api.NamespaceAll, fields.Everything())
	hf := framework.ResourceEventHandlerFuncs{
		AddFunc: feeder.handleNodeCreate,
		UpdateFunc: func(oldObj, newObj interface{}) {
			feeder.handleNodeUpdate(oldObj, newObj)
		},
		DeleteFunc: feeder.handleNodeDelete,
	}

	store, controller := framework.NewInformer(lw, &api.Node{}, resyncPeriod, hf)

	go controller.Run(wait.NeverStop)
	return store
}
