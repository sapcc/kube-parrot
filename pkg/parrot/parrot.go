package parrot

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	VERSION = "0.0.0.dev"
)

type Options struct {
	GrpcPort     int
	As           int
	LocalAddress net.IP
	Neighbors    []*net.IP
	Kubeconfig   string
}

type Parrot struct {
	Options

	client *kubernetes.Clientset
	bgp    *bgp.Server

	informers       informer.SharedInformerFactory
	externalSevices *controller.ExternalServicesController
}

func New(opts Options) *Parrot {
	p := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(opts.LocalAddress, opts.As, opts.GrpcPort),
		client:  NewClient(opts.Kubeconfig),
	}

	p.informers = informer.NewSharedInformerFactory(p.client, 5*time.Minute)
	p.externalSevices = controller.NewExternalServicesController(p.informers, opts.LocalAddress, p.bgp.ExternalIPRoutes)

	p.informers.Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    p.debugAdd,
		UpdateFunc: p.debugUpdate,
		DeleteFunc: p.debugDelete,
	})

	p.informers.Services().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    p.debugAdd,
		UpdateFunc: p.debugUpdate,
		DeleteFunc: p.debugDelete,
	})

	p.informers.Endpoints().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    p.debugAdd,
		UpdateFunc: p.debugUpdate,
		DeleteFunc: p.debugDelete,
	})

	return p
}

func (p *Parrot) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)

	go p.bgp.Run(stopCh, wg)
	go p.informers.Start(stopCh)

	// Wait for BGP main loop
	time.Sleep(2 * time.Second)

	for _, neighbor := range p.Neighbors {
		p.bgp.AddNeighbor(neighbor.String())
	}

	cache.WaitForCacheSync(
		stopCh,
		p.informers.Endpoints().Informer().HasSynced,
		p.informers.Pods().Informer().HasSynced,
		p.informers.Services().Informer().HasSynced,
	)

	go p.externalSevices.Run(stopCh, wg)
}

func (p *Parrot) debugAdd(obj interface{}) {
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	glog.V(5).Infof("ADD %s (%s)", reflect.TypeOf(obj), key)
}

func (p *Parrot) debugDelete(obj interface{}) {
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	glog.V(5).Infof("DELETE %s (%s)", reflect.TypeOf(obj), key)
}

func (p *Parrot) debugUpdate(cur, old interface{}) {
	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(cur)

	if strings.HasSuffix(key, "kube-scheduler") || strings.HasSuffix(key, "kube-controller-manager") {
		return
	}

	glog.V(5).Infof("UPDATE %s (%s)", reflect.TypeOf(cur), key)
}
