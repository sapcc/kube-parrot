package informer

import (
	"reflect"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})

	Services() ServiceInformer
	Nodes() NodeInformer
	Endpoints() EndpointInformer
	Pods() PodInformer
}

type sharedInformerFactory struct {
	client        kubernetes.Interface
	lock          sync.Mutex
	defaultResync time.Duration

	informers        map[reflect.Type]cache.SharedIndexInformer
	startedInformers map[reflect.Type]bool
}

func NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory {
	return &sharedInformerFactory{
		client:           client,
		defaultResync:    defaultResync,
		informers:        make(map[reflect.Type]cache.SharedIndexInformer),
		startedInformers: make(map[reflect.Type]bool),
	}
}

func (s *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for informerType, informer := range s.informers {
		if !s.startedInformers[informerType] {
			go informer.Run(stopCh)
			s.startedInformers[informerType] = true
		}
	}
}

func (f *sharedInformerFactory) Pods() PodInformer {
	return &podInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Services() ServiceInformer {
	return &serviceInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Nodes() NodeInformer {
	return &nodeInformer{sharedInformerFactory: f}
}

func (f *sharedInformerFactory) Endpoints() EndpointInformer {
	return &endpointInformer{sharedInformerFactory: f}
}
