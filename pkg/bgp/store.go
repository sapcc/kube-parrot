package bgp

import (
	"fmt"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"
)

type RoutesStore struct {
	cache.Store
}

type NodePodSubnetRoutesStore struct {
	store RoutesStore
}

type ExternalIPRoutesStore struct {
	store RoutesStore
}

func RouteKeyFunc(obj interface{}) (string, error) {
	route := obj.(RouteInterface)
	return fmt.Sprintf("%s->%s", route.SourceCIDR(), route.NextHop()), nil
}

func newNodePodSubnetRoutesStore() *NodePodSubnetRoutesStore {
	return &NodePodSubnetRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc)}}
}

func newExternalIPRoutesStore() *ExternalIPRoutesStore {
	return &ExternalIPRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc)}}
}

func (s *RoutesStore) Add(route RouteInterface) error {
	if _, exists, _ := s.Get(route); !exists {
		fmt.Printf("Announcing %s\n", route)
		return s.Add(route)
	}

	return nil
}

func (s *RoutesStore) Delete(route RouteInterface) error {
	if _, exists, _ := s.Get(route); exists {
		fmt.Printf("Withdrawing %s\n", route)
		return s.Delete(route)
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
