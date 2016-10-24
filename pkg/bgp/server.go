package bgp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"

	"github.com/golang/glog"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet/bgp"
	gobgp "github.com/osrg/gobgp/server"
	"github.com/osrg/gobgp/table"
)

type Server struct {
	bgp  *gobgp.BgpServer
	grpc *api.Server

	as           uint32
	routerId     string
	localAddress string

	Routes *StoreToRouteLister
}

type RouteCategory int

type Route struct {
	Category   RouteCategory
	SourceCIDR string
	NextHop    string
	Source     interface{}
	Target     interface{}
}

type StoreToRouteLister struct {
	cache.Store
}

const (
	EXTERNAL_IP RouteCategory = iota
	PODSUBNET
	APISERVER
)

func NewServer(localAddress net.IP, as int, port int) *Server {
	server := &Server{
		localAddress: localAddress.String(),
		routerId:     localAddress.String(),
		as:           uint32(as),
		Routes:       &StoreToRouteLister{cache.NewStore(RouteKeyFunc)},
	}

	server.bgp = gobgp.NewBgpServer()
	server.grpc = api.NewGrpcServer(
		server.bgp,
		fmt.Sprintf(":%v", port),
	)

	return server
}

func NewExternalIPRoute(service *v1.Service, proxy *v1.Pod) Route {
	return Route{
		Category:   EXTERNAL_IP,
		SourceCIDR: fmt.Sprintf("%s/32", service.Spec.ExternalIPs[0]),
		NextHop:    proxy.Status.HostIP,
		Source:     service,
		Target:     proxy,
	}
}

func (r Route) String() string {
	source, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(r.Source)
	target, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(r.Target)
	category := ""

	switch r.Category {
	case EXTERNAL_IP:
		category = "ExternalIP:"
	case PODSUBNET:
		category = "PodSubnet:"
	case APISERVER:
		category = "APIServer:"
	}
	return fmt.Sprintf("%18s -> %-15s (%s %s -> %s)", r.SourceCIDR, r.NextHop, category, source, target)
}

func (s *Server) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	// logrus.SetLevel(logrus.DebugLevel)

	go s.bgp.Serve()
	go s.grpc.Serve()

	time.Sleep(1 * time.Second)
	s.startServer()

	<-stopCh
	s.bgp.Stop()
	time.Sleep(1 * time.Second)
}

func (s *Server) startServer() {
	global := &config.Global{
		Config: config.GlobalConfig{
			As:       s.as,
			RouterId: s.routerId,
			Port:     -1,
		},
	}

	if err := s.bgp.Start(global); err != nil {
		glog.Errorf("Oops. Something went wrong starting bgp server: %s", err)
	}
}

func RouteKeyFunc(obj interface{}) (string, error) {
	route := obj.(Route)
	return fmt.Sprintf("%s/%s->%s", route.Category, route.SourceCIDR, route.NextHop), nil
}

func (s *Server) AddRoute(source, nextHop string) error {
	sourceIP := net.ParseIP(source)
	nextHopIP := net.ParseIP(nextHop)

	if sourceIP == nil {
		return fmt.Errorf("Error adding route. Source %s is not an IP,", source)
	}

	if nextHopIP == nil {
		return fmt.Errorf("Error adding route. NextHop %s is not an IP,", nextHop)
	}

	return s.AddPath(getExternalIPRoute(sourceIP, nextHopIP, false))
}

func (s *Server) DeleteRoute(source, nextHop string) error {
	sourceIP := net.ParseIP(source)
	nextHopIP := net.ParseIP(nextHop)

	if sourceIP == nil {
		return fmt.Errorf("Error adding route. Source %s is not an IP,", source)
	}

	if nextHopIP == nil {
		return fmt.Errorf("Error adding route. NextHop %s is not an IP,", nextHop)
	}

	return s.DeletePath(getExternalIPRoute(sourceIP, nextHopIP, true))
}

func (s *Server) AddPath(path *table.Path) error {
	glog.V(3).Infof("Adding Path: %s", path)
	if _, err := s.bgp.AddPath("", []*table.Path{path}); err != nil {
		return fmt.Errorf("Oops. Something went wrong adding path: %s", err)
	}

	s.debug()
	return nil
}

func (s *Server) DeletePath(path *table.Path) error {
	glog.V(3).Infof("Deleting Path: %s", path)
	if err := s.bgp.DeletePath(nil, bgp.RF_IPv4_UC, "", []*table.Path{path}); err != nil {
		return fmt.Errorf("Oops. Something went wrong deleting route: %s", err)
	}
	s.debug()
	return nil
}

func (s *Server) AddNeighbor(neighbor string) {
	glog.Infof("Adding Neighbor: %s", neighbor)
	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: neighbor,
			PeerAs:          s.as,
		},
	}

	if err := s.bgp.AddNeighbor(n); err != nil {
		glog.Errorf("Oops. Something went wrong adding neighbor: %s", err)
	}
}

func (s *Server) debug() {
	for _, route := range s.bgp.GetVrf() {
		glog.V(5).Infof("%s", route)
	}
}

func getExternalIPRoute(service, node net.IP, isWithdraw bool) *table.Path {
	nlri := bgp.NewIPAddrPrefix(uint8(32), service.String())

	pattr := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
		bgp.NewPathAttributeNextHop(node.String()),
	}

	return table.NewPath(nil, nlri, isWithdraw, pattr, time.Now(), false)
}

func (s *StoreToRouteLister) List(category RouteCategory) (routes []Route) {
	for _, m := range s.Store.List() {
		route := *(m.(*Route))
		if route.Category == category {
			routes = append(routes, route)
		}
	}
	return routes
}
