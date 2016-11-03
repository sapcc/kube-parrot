package bgp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	gobgp "github.com/osrg/gobgp/server"
)

type Server struct {
	bgp  *gobgp.BgpServer
	grpc *api.Server

	as           uint32
	routerId     string
	localAddress string

	ExternalIPRoutes    *ExternalIPRoutesStore
	NodePodSubnetRoutes *NodePodSubnetRoutesStore
}

func NewServer(localAddress net.IP, as int, port int) *Server {
	server := &Server{
		localAddress:        localAddress.String(),
		routerId:            localAddress.String(),
		as:                  uint32(as),
		ExternalIPRoutes:    newExternalIPRoutesStore(),
		NodePodSubnetRoutes: newNodePodSubnetRoutesStore(),
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

//func (c *Server) AddPodSubnetRoute(node *v1.Node) error {
//  route := NewNodePodSubnetRoute(node)

//  if _, exists, _ := c.NodePodSubnetRoutes.Get(route); !exists {
//    fmt.Printf("Announcing %s\n", route)
//    return c.NodePodSubnetRoutes.Add(route)
//  }

//  return nil
//}

//func (c *Server) AddExternalIPRoute(service *v1.Service, proxy *v1.Pod) error {
//  route := NewExternalIPRoute(service, proxy)

//  if _, exists, _ := c.ExternalIPRoutes.Get(route); !exists {
//    fmt.Printf("Announcing %s\n", route)
//    return c.ExternalIPRoutes.Add(route)
//  }

//  return nil
//}

//func (c *Server) DeleteExternalIPRoute(route ExternalIPRoute) error {
//  if _, exists, _ := c.ExternalIPRoutes.Get(route); exists {
//    fmt.Printf("Withdrawing %s\n", route)
//    return c.ExternalIPRoutes.Delete(route)
//  }

//  return nil
//}

//func (c *Server) DeleteNodePodSubnetRoute(route NodePodSubnetRoute) error {
//  if _, exists, _ := c.NodePodSubnetRoutes.Get(route); exists {
//    fmt.Printf("Withdrawing %s\n", route)
//    return c.NodePodSubnetRoutes.Delete(route)
//  }

//  return nil
//}
