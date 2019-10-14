package bgp

import (
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/config"
	gobgp "github.com/osrg/gobgp/server"
)

type Server struct {
	bgp *gobgp.BgpServer

	as           uint32
	routerId     string
	localAddress string

	ExternalIPRoutes    *ExternalIPRoutesStore
	NodePodSubnetRoutes *NodePodSubnetRoutesStore
}

func NewServer(localAddress *net.IP, as int, port int) *Server {
	server := &Server{
		localAddress: localAddress.String(),
		routerId:     localAddress.String(),
		as:           uint32(as),
	}

	server.ExternalIPRoutes = newExternalIPRoutesStore(server)
	server.NodePodSubnetRoutes = newNodePodSubnetRoutesStore(server)

	server.bgp = gobgp.NewBgpServer()
	return server
}

func (s *Server) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	// logrus.SetLevel(logrus.DebugLevel)

	go s.bgp.Serve()

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
