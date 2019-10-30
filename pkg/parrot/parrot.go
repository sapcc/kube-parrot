package parrot

import (
	"fmt"
	"net"
	"sync"
	"time"

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
	GrpcPort  int
	As        int
	NodeName  string
	HostIP    net.IP
	Neighbors []*net.IP
	MetricsPort int
}

type Parrot struct {
	Options

	client *kubernetes.Clientset
	bgp    *bgp.Server

	informers       informer.SharedInformerFactory
	externalSevices *controller.ExternalServicesController
	podSubnets      *controller.PodSubnetsController
}

func New(opts Options) *Parrot {
	p := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(&opts.HostIP, opts.As, opts.GrpcPort),
		client:  NewClient(),
	}

	// Register parrot prometheus metrics collector.
	RegisterCollector(p.NodeName, opts.Neighbors, p.bgp)

	p.informers = informer.NewSharedInformerFactory(p.client, 5*time.Minute)
	p.externalSevices = controller.NewExternalServicesController(p.informers, &opts.HostIP, opts.NodeName, p.bgp.ExternalIPRoutes)
	p.podSubnets = controller.NewPodSubnetsController(p.informers, &opts.HostIP, p.bgp.NodePodSubnetRoutes)

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
		p.informers.Nodes().Informer().HasSynced,
		p.informers.Services().Informer().HasSynced,
	)

	go p.externalSevices.Run(stopCh, wg)
	go p.podSubnets.Run(stopCh, wg)
}
