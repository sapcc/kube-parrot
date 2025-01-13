package informer

import (
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	informers_v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodInformer is type of SharedIndexInformer which watches and lists all pods.
// Interface provides constructor for informer and lister for pods
type PodInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() *StoreToPodLister
}

type podInformer struct {
	*sharedInformerFactory
}

// Informer checks whether podInformer exists in sharedInformerFactory and if not, it creates new informer of type
// podInformer and connects it to sharedInformerFactory
func (f *podInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(&v1.Pod{})
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}
	informer = NewPodInformer(f.client, f.defaultResync)
	f.informers[informerType] = informer

	return informer
}

// Lister returns lister for podInformer
func (f *podInformer) Lister() *StoreToPodLister {
	informer := f.Informer()
	return &StoreToPodLister{Indexer: informer.GetIndexer()}
}

func NewPodInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return informers_v1.NewPodInformer(
		client,
		"",
		resyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
}

type EndpointInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() *StoreToEndpointsLister
}

type endpointInformer struct {
	*sharedInformerFactory
}

func (f *endpointInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(&v1.Endpoints{})
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}
	informer = NewEndpointInformer(f.client, f.defaultResync)
	f.informers[informerType] = informer

	return informer
}

func (f *endpointInformer) Lister() *StoreToEndpointsLister {
	informer := f.Informer()
	return &StoreToEndpointsLister{Store: informer.GetStore()}
}

func NewEndpointInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return informers_v1.NewEndpointsInformer(
		client,
		"",
		resyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
}

type ServiceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() *StoreToServiceLister
}

type serviceInformer struct {
	*sharedInformerFactory
}

func (f *serviceInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(&v1.Service{})
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}
	informer = NewServiceInformer(f.client, f.defaultResync)
	f.informers[informerType] = informer

	return informer
}

func (f *serviceInformer) Lister() *StoreToServiceLister {
	informer := f.Informer()
	return &StoreToServiceLister{Indexer: informer.GetIndexer()}
}

func NewServiceInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return informers_v1.NewServiceInformer(
		client,
		"",
		resyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
}

type NodeInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() *StoreToNodeLister
}

type nodeInformer struct {
	*sharedInformerFactory
}

// Informer checks whether nodeInformer exists in sharedInformerFactory and if not, it creates new informer of type
// nodeInformer and connects it to sharedInformerFactory
func (f *nodeInformer) Informer() cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(&v1.Node{})
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}
	informer = NewNodeInformer(f.client, f.defaultResync)
	f.informers[informerType] = informer

	return informer
}

// Lister returns lister for nodeInformer
func (f *nodeInformer) Lister() *StoreToNodeLister {
	informer := f.Informer()
	return &StoreToNodeLister{Store: informer.GetStore()}
}

func NewNodeInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return informers_v1.NewNodeInformer(
		client,
		resyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
}
