package bgp

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/gobgp/cmd"
	"github.com/osrg/gobgp/packet/bgp"
	gobgp "github.com/osrg/gobgp/server"
)

type Server struct {
	bgp  *gobgp.BgpServer
	grpc *gobgp.Server

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
	server.grpc = gobgp.NewGrpcServer(
		fmt.Sprintf(":%v", port),
		server.bgp.GrpcReqCh,
	)

	return server
}

func (s *Server) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	go s.bgp.Serve()
	go s.grpc.Serve()

	time.Sleep(1 * time.Second)
	s.startServer()

	<-stopCh
	s.bgp.Stop()
}

func (s *Server) startServer() {
	req := gobgp.NewGrpcRequest(gobgp.REQ_START_SERVER, "", bgp.RouteFamily(0), &gobgpapi.StartServerRequest{
		Global: &gobgpapi.Global{
			As:       s.as,
			RouterId: s.routerId,
		},
	})
	s.bgp.GrpcReqCh <- req
	res := <-req.ResponseCh
	if err := res.Err(); err != nil {
		glog.Errorf("Oops. Something went wrong starting bgp server: %s", err)
	}
}

func (s *Server) AddRoute(route []string) {
	glog.Infof("Adding Route: %s", strings.Join(route, " "))
	path, _ := cmd.ParsePath(bgp.RF_IPv4_UC, route)
	req := gobgp.NewGrpcRequest(gobgp.REQ_ADD_PATH, "", bgp.RouteFamily(0), &gobgpapi.AddPathRequest{
		Resource: gobgpapi.Resource_GLOBAL,
		Path:     path,
	})

	s.bgp.GrpcReqCh <- req
	res := <-req.ResponseCh
	if err := res.Err(); err != nil {
		glog.Errorf("Oops. Something went wrong adding route: %s", err)
	}
}

func (s *Server) DeleteRoute(route []string) {
	glog.Infof("Deleting Route: %s", strings.Join(route, " "))
	path, _ := cmd.ParsePath(bgp.RF_IPv4_UC, route)
	req := gobgp.NewGrpcRequest(gobgp.REQ_ADD_PATH, "", bgp.RouteFamily(0), &gobgpapi.DeletePathRequest{
		Resource: gobgpapi.Resource_GLOBAL,
		Path:     path,
	})

	s.bgp.GrpcReqCh <- req
	res := <-req.ResponseCh
	if err := res.Err(); err != nil {
		glog.Errorf("Oops. Something went wrong deleting route: %s", err)
	}
}

func (s *Server) AddNeighbor(neighbor string) {
	glog.Infof("Adding Neighbor: %s", neighbor)
	req := gobgp.NewGrpcRequest(gobgp.REQ_GRPC_ADD_NEIGHBOR, "", bgp.RouteFamily(0), &gobgpapi.AddNeighborRequest{
		Peer: &gobgpapi.Peer{
			Conf: &gobgpapi.PeerConf{
				NeighborAddress: neighbor,
				PeerAs:          s.as,
			},
			Transport: &gobgpapi.Transport{
				LocalAddress: s.localAddress,
			},
		},
	})

	s.bgp.GrpcReqCh <- req
	res := <-req.ResponseCh
	if err := res.Err(); err != nil {
		glog.Errorf("Oops. Something went wrong adding neighbor: %s", err)
	}
}
