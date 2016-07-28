package parrot

import (
	"fmt"
	"net"

	gobgp "github.com/osrg/gobgp/server"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

var (
	VERSION = "0.0.0.dev"
)

type Parrot struct {
	client     *client.Client
	bgpServer  *gobgp.BgpServer
	grpcServer *gobgp.Server
	Options
}

type neighbors []*net.IP

type Options struct {
	grpcPort     int
	As           int
	LocalAddress net.IP
	Neighbors    neighbors
}

func (f *neighbors) String() string {
	return fmt.Sprintf("%v", *f)
}

func (i *neighbors) Set(value string) error {
	ip := net.ParseIP(value)
	if ip == nil {
		return fmt.Errorf("%v is not a valid IP address", value)
	}

	*i = append(*i, &ip)
	return nil
}

func (s *neighbors) Type() string {
	return "neighborSlice"
}

func New(opts Options) *Parrot {
	return &Parrot{Options: opts}
}

func (p *Parrot) Start() {
	fmt.Printf("Welcome to Kubernetes Parrot %v\n", VERSION)
	p.createKubernetesClient()
	p.createpBGPServer()
	p.startBGPServer()
	p.addBGPNeighbors()
	p.watchNodes()
}

func (p *Parrot) Stop() {
	p.bgpServer.Shutdown()
}
