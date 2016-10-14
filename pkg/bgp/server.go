package bgp

import (
	"fmt"
	"net"
	"sync"
	"time"

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
}

func NewServer(localAddress net.IP, as int, port int) *Server {
	server := &Server{
		localAddress: localAddress.String(),
		routerId:     localAddress.String(),
		as:           uint32(as),
	}

	server.bgp = gobgp.NewBgpServer()
	server.grpc = api.NewGrpcServer(
		server.bgp,
		fmt.Sprintf(":%v", port),
	)

	return server
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

func (s *Server) AddPath(path *table.Path) {
	glog.V(3).Infof("Adding Path: %s", path)
	if _, err := s.bgp.AddPath("", []*table.Path{path}); err != nil {
		glog.Errorf("Oops. Something went wrong adding path: %s", err)
	}

	s.debug()
}

func (s *Server) DeletePath(path *table.Path) {
	glog.V(3).Infof("Deleting Path: %s", path)
	if err := s.bgp.DeletePath(nil, bgp.RF_IPv4_UC, "", []*table.Path{path}); err != nil {
		glog.Errorf("Oops. Something went wrong deleting route: %s", err)
	}
	s.debug()
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
	for _, route := range s.bgp.GetRib() {
		glog.V(5).Infof("%s", route)
	}
}
