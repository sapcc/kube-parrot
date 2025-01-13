package bgp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/config"
	gobgp "github.com/osrg/gobgp/server"
	"github.com/sapcc/go-bits/must"
)

type Server struct {
	bgp  *gobgp.BgpServer
	grpc *api.Server

	as           uint32
	remoteAs     uint32
	routerID     string
	localAddress string

	ExternalIPRoutes    *ExternalIPRoutesStore
	NodePodSubnetRoutes *NodePodSubnetRoutesStore
}

func NewServer(localAddress *net.IP, as, remoteAs uint32, port int) *Server {
	server := &Server{
		localAddress: localAddress.String(),
		routerID:     localAddress.String(),
		as:           as,
		remoteAs:     remoteAs,
	}

	server.ExternalIPRoutes = newExternalIPRoutesStore(server)
	server.NodePodSubnetRoutes = newNodePodSubnetRoutesStore(server)

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
	go must.Succeed(s.grpc.Serve())

	time.Sleep(1 * time.Second)
	s.startServer()

	<-stopCh
	must.Succeed(s.bgp.Stop())
	time.Sleep(1 * time.Second)
}

func (s *Server) startServer() {
	global := &config.Global{
		Config: config.GlobalConfig{
			As:       s.as,
			RouterId: s.routerID,
			Port:     -1,
		},
	}

	if err := s.bgp.Start(global); err != nil {
		glog.Errorf("Oops. Something went wrong starting bgp server: %s", err)
	}
}

func (s *Server) AddNeighbor(neighbor string) {
	glog.Infof("Adding Neighbor: %s remote ASN %d", neighbor, s.remoteAs)
	n := &config.Neighbor{
		Config: config.NeighborConfig{
			NeighborAddress: neighbor,
			PeerAs:          s.remoteAs,
		},
	}

	if err := s.bgp.AddNeighbor(n); err != nil {
		glog.Errorf("Oops. Something went wrong adding neighbor: %s", err)
	}
}

func (s *Server) GetNeighbor(address string) ([]*api.Peer, error) {
	resp, err := s.grpc.GetNeighbor(context.Background(), &api.GetNeighborRequest{
		Address:          address,
		EnableAdvertised: true,
	})
	if err != nil {
		return nil, err
	}

	// GoBGP server didn't return any neighbors (aka peers) though there are some configured.
	if resp.GetPeers() == nil {
		return nil, errors.New("invalid reply from goBGP server")
	}
	if len(resp.GetPeers()) == 0 {
		return nil, errors.New("invalid reply from goBGP server")
	}

	return resp.GetPeers(), nil
}
