package parrot

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/tools/cache"
)

var (
	VERSION = "0.0.0.dev"
)

type Options struct {
	GrpcPort      int
	As            int
	LocalAddress  net.IP
	MasterAddress net.IP
	Neighbors     []*net.IP
}

type Parrot struct {
	Options

	client *kubernetes.Clientset
	bgp    *bgp.Server

	informers informer.SharedInformerFactory

	podSubnets      *controller.PodSubnetsController
	externalSevices *controller.ExternalServicesController
	apiservers      *controller.APIServerController
}

func New(opts Options) *Parrot {
	p := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(opts.LocalAddress, opts.As, opts.GrpcPort, opts.MasterAddress),
		client:  NewClient(),
	}

	p.informers = informer.NewSharedInformerFactory(p.client, 5*time.Minute)
	p.podSubnets = controller.NewPodSubnetsController(p.informers, p.bgp.NodePodSubnetRoutes)
	p.externalSevices = controller.NewExternalServicesController(p.informers, p.bgp.ExternalIPRoutes)
	p.apiservers = controller.NewAPIServerController(p.informers, p.bgp.APIServerRoutes)

	return p
}

func (p *Parrot) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)

	go p.bgp.Run(stopCh, wg)
	go p.informers.Start(stopCh)

	// Wait for BGP main loop
	time.Sleep(1 * time.Second)

	for _, neighbor := range p.Neighbors {
		p.bgp.AddNeighbor(neighbor.String())
	}

	cache.WaitForCacheSync(
		stopCh,
		p.informers.Endpoints().Informer().HasSynced,
		p.informers.Nodes().Informer().HasSynced,
		p.informers.Pods().Informer().HasSynced,
		p.informers.Services().Informer().HasSynced,
	)

	go p.podSubnets.Run(stopCh, wg)
	go p.externalSevices.Run(stopCh, wg)
	go p.apiservers.Run(stopCh, wg)
}
