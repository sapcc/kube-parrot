package parrot

import (
	"fmt"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/packet/bgp"
	gobgp "github.com/osrg/gobgp/server"
)

func (p *Parrot) createpBGPServer() {
	p.bgpServer = gobgp.NewBgpServer()
	p.grpcServer = gobgp.NewGrpcServer(
		fmt.Sprintf(":%v", p.grpcPort),
		p.bgpServer.GrpcReqCh,
	)

	go p.bgpServer.Serve()
	go p.grpcServer.Serve()
}

func (p *Parrot) startBGPServer() {
	// global configuration
	req := gobgp.NewGrpcRequest(gobgp.REQ_START_SERVER, "", bgp.RouteFamily(0), &api.StartServerRequest{
		Global: &api.Global{
			As:       uint32(p.As),
			RouterId: p.LocalAddress.String(),
		},
	})
	p.bgpServer.GrpcReqCh <- req
	res := <-req.ResponseCh
	p.handleError(res.Err())
}

func (p *Parrot) addBGPNeighbors() {
	for _, neighbor := range p.Neighbors {
		req := gobgp.NewGrpcRequest(gobgp.REQ_GRPC_ADD_NEIGHBOR, "", bgp.RouteFamily(0), &api.AddNeighborRequest{
			Peer: &api.Peer{
				Conf: &api.PeerConf{
					NeighborAddress: neighbor.String(),
					PeerAs:          uint32(p.As),
				},
				Transport: &api.Transport{
					LocalAddress: p.LocalAddress.String(),
				},
			},
		})

		p.bgpServer.GrpcReqCh <- req
		res := <-req.ResponseCh
		p.handleError(res.Err())
	}
}
