package parrot

import (
	"fmt"
	"net"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	"github.com/sapcc/kube-parrot/pkg/kubernetes"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/wait"
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

func (p *Parrot) Start() {
	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)

	p.bgp.Run(wait.NeverStop)
	go p.podSubnets.Run(wait.NeverStop)

	for _, neighbor := range p.Neighbors {
		p.bgp.AddNeighbor(neighbor.String())
	}

}

func (p *Parrot) Stop() {
}
