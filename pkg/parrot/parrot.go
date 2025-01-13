// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package parrot

import (
	"fmt"
	"net"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/sapcc/kube-parrot/pkg/bgp"
	"github.com/sapcc/kube-parrot/pkg/controller"
	"github.com/sapcc/kube-parrot/pkg/forked/informer"
	"github.com/sapcc/kube-parrot/pkg/metrics"
)

var (
	VERSION = "0.0.0.dev"
)

type Options struct {
	GrpcPort      int
	As            uint32
	RemoteAs      uint32
	NodeName      string
	HostIP        net.IP
	Neighbors     []*net.IP
	MetricsPort   int
	TraceCount    int
	NeighborCount int
	PodSubnet     bool
}

type Parrot struct {
	Options

	client *kubernetes.Clientset
	bgp    *bgp.Server

	informers        informer.SharedInformerFactory
	externalServices *controller.ExternalServicesController
	podSubnets       *controller.PodSubnetsController
}

func New(opts Options) *Parrot {
	p := &Parrot{
		Options: opts,
		bgp:     bgp.NewServer(&opts.HostIP, opts.As, opts.RemoteAs, opts.GrpcPort),
		client:  NewClient(),
	}

	// Register parrot prometheus metrics collector.
	metrics.RegisterCollector(p.NodeName, opts.Neighbors, p.bgp)

	p.informers = informer.NewSharedInformerFactory(p.client, 5*time.Minute)
	p.externalServices = controller.NewExternalServicesController(p.informers, &opts.HostIP, opts.NodeName, p.bgp.ExternalIPRoutes)
	p.podSubnets = controller.NewPodSubnetsController(p.informers, &opts.HostIP, p.bgp.NodePodSubnetRoutes)

	return p
}

func (p *Parrot) Run(opts Options, stopCh <-chan struct{}, wg *sync.WaitGroup) {
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

	go p.externalServices.Run(stopCh, wg)
	if opts.PodSubnet {
		go p.podSubnets.Run(stopCh, wg)
	}
}
