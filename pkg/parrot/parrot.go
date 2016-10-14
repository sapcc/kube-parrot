package parrot

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	client "github.com/sapcc/kube-parrot/pkg/kubernetes"
	"k8s.io/client-go/1.5/kubernetes"
)

var (
	VERSION = "0.0.0.dev"
)

type Options struct {
	GrpcPort     int
	As           int
	LocalAddress net.IP
	Neighbors    []*net.IP
}

type Parrot struct {
	Options

	client          *kubernetes.Clientset
	bgp             *bgp.Server
	podSubnets      *controller.PodSubnetsController
	externalSevices *controller.ExternalServicesController
}

func New(opts Options) *Parrot {
	parrot := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(opts.LocalAddress, opts.As, opts.GrpcPort),
		client:  client.NewClient(),
	}

	parrot.podSubnets = controller.NewPodSubnetsController(parrot.client, parrot.bgp)
	parrot.externalSevices = controller.NewExternalServicesController(parrot.client, parrot.bgp)

	return parrot
}

func (p *Parrot) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)

	go p.bgp.Run(stopCh, wg)

	// Wait for BGP main loop
	time.Sleep(1 * time.Second)

	for _, neighbor := range p.Neighbors {
		p.bgp.AddNeighbor(neighbor.String())
	}

	//go p.podSubnets.Run(stopCh)
	go p.externalSevices.Run(stopCh, wg)
}
