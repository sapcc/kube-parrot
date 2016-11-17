package bgp

import (
	"fmt"
	"net"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"
)

type RoutesStore struct {
	cache.Store
	server *Server
}

type NodePodSubnetRoutesStore struct {
	store RoutesStore
}

type NodeServiceSubnetRoutesStore struct {
	store RoutesStore
}

type ExternalIPRoutesStore struct {
	store RoutesStore
}

type APIServerRoutesStore struct {
	store    RoutesStore
	masterIP net.IP
}

func RouteKeyFunc(obj interface{}) (string, error) {
	route := obj.(RouteInterface)
	prefix, length := route.Source()
	return fmt.Sprintf("%s/%s->%s", prefix, length, route.NextHop().To4().String()), nil
}

func newNodePodSubnetRoutesStore(bgp *Server) *NodePodSubnetRoutesStore {
	return &NodePodSubnetRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc), bgp}}
}

func newNodeServiceSubnetRoutesStore(bgp *Server) *NodeServiceSubnetRoutesStore {
	return &NodeServiceSubnetRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc), bgp}}
}

func newExternalIPRoutesStore(bgp *Server) *ExternalIPRoutesStore {
	return &ExternalIPRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc), bgp}}
}

func newAPIServerRoutesStore(bgp *Server, masterIP net.IP) *APIServerRoutesStore {
	return &APIServerRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc), bgp}, masterIP}
}

func (s *RoutesStore) Add(route RouteInterface) error {
	if _, exists, _ := s.Store.Get(route); !exists {
		glog.Infof("Announcing  %s\n", Route{route})

		if _, err := s.server.bgp.AddPath("", []*table.Path{Route{route}.Path(false)}); err != nil {
			return fmt.Errorf("Oops. Something went wrong adding path: %s", err)
		}

		return s.Store.Add(route)
	}

	return nil
}

func (s *RoutesStore) Delete(route RouteInterface) error {
	if _, exists, _ := s.Store.Get(route); exists {
		glog.Infof("Withdrawing %s\n", Route{route})

		if err := s.server.bgp.DeletePath(nil, bgp.RF_IPv4_UC, "", []*table.Path{Route{route}.Path(true)}); err != nil {
			return fmt.Errorf("Oops. Something went wrong deleting route: %s", err)
		}

		return s.Store.Delete(route)
	}

	return nil
}

func (s *ExternalIPRoutesStore) List() (routes []ExternalIPRoute) {
	for _, m := range s.store.List() {
		routes = append(routes, m.(ExternalIPRoute))
	}
	return routes
}

func (s *ExternalIPRoutesStore) Add(service *v1.Service, proxy *v1.Pod) error {
	return s.store.Add(NewExternalIPRoute(service, proxy))
}

func (s *ExternalIPRoutesStore) Delete(route ExternalIPRoute) error {
	return s.store.Delete(route)
}

func (s *NodePodSubnetRoutesStore) List() (routes []NodePodSubnetRoute) {
	for _, m := range s.store.List() {
		routes = append(routes, m.(NodePodSubnetRoute))
	}
	return routes
}

func (s *NodePodSubnetRoutesStore) Add(node *v1.Node) error {
	return s.store.Add(NewNodePodSubnetRoute(node))
}

func (s *NodePodSubnetRoutesStore) Delete(route NodePodSubnetRoute) error {
	return s.store.Delete(route)
}

func (s *NodeServiceSubnetRoutesStore) List() (routes []NodeServiceSubnetRoute) {
	for _, m := range s.store.List() {
		routes = append(routes, m.(NodeServiceSubnetRoute))
	}
	return routes
}

func (s *NodeServiceSubnetRoutesStore) Add(pod *v1.Pod, subnet net.IPNet) error {
	return s.store.Add(NewNodeServiceSubnetRoute(pod, subnet))
}

func (s *NodeServiceSubnetRoutesStore) Delete(route NodeServiceSubnetRoute) error {
	return s.store.Delete(route)
}

func (s *APIServerRoutesStore) List() (routes []APIServerRoute) {
	for _, m := range s.store.List() {
		routes = append(routes, m.(APIServerRoute))
	}
	return routes
}

func (s *APIServerRoutesStore) Add(apiserver *v1.Pod) error {
	return s.store.Add(NewAPIServerRoute(apiserver, s.masterIP))
}

func (s *APIServerRoutesStore) Delete(route APIServerRoute) error {
	return s.store.Delete(route)
}
