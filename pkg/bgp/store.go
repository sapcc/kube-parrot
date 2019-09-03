package bgp

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type RoutesStore struct {
	cache.Store
	server *Server
}

type ExternalIPRoutesStore struct {
	store RoutesStore
}

func RouteKeyFunc(obj interface{}) (string, error) {
	route := obj.(RouteInterface)
	prefix, length := route.Source()
	return fmt.Sprintf("%s/%s->%s", prefix, length, route.NextHop().To4().String()), nil
}

func newExternalIPRoutesStore(bgp *Server) *ExternalIPRoutesStore {
	return &ExternalIPRoutesStore{RoutesStore{cache.NewStore(RouteKeyFunc), bgp}}
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
