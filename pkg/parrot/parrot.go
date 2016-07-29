package parrot

import (
	"fmt"
	"net"
	"sync"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	"github.com/sapcc/kube-parrot/pkg/kubernetes"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
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

	client     *clientset.Clientset
	bgp        *bgp.Server
	podSubnets *controller.PodSubnetsController
}

func New(opts Options) *Parrot {
	parrot := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(opts.LocalAddress, opts.As, opts.GrpcPort),
		client:  kubernetes.NewClient(),
	}

	parrot.podSubnets = controller.NewPodSubnetsController(parrot.client, parrot.bgp)

	return parrot
}

func (p *Parrot) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)

	p.bgp.Run(stopCh, wg)
	p.podSubnets.Run(stopCh)

	for _, neighbor := range p.Neighbors {
		p.bgp.AddNeighbor(neighbor.String())
	}
}
