package bgp

import (
	"fmt"
	"net"
	"time"

	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	v1 "k8s.io/api/core/v1"
)

const (
	AnnotationNodePodSubnet = "parrot.sap.cc/podsubnet"
)

type RouteInterface interface {
	Source() (*net.IP, uint8)
	NextHop() *net.IP
	Describe() string
	Path(bool) *table.Path
}

type Route struct {
	RouteInterface
}

func (r Route) String() string {
	prefix, length := r.Source()

	return fmt.Sprintf("%16s/%v -> %-15s (%s)", prefix.To4().String(), length, r.NextHop().To4().String(), r.Describe())
}

func (r Route) Path(isWithdraw bool) *table.Path {
	prefix, length := r.Source()
	nlri := bgp.NewIPAddrPrefix(length, prefix.To4().String())

	pattr := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
		bgp.NewPathAttributeNextHop(r.NextHop().To4().String()),
	}

	return table.NewPath(nil, nlri, isWithdraw, pattr, time.Now(), false)
}

type ExternalIPRoute struct {
	Route
	Service *v1.Service
	HostIP  *net.IP
}

func (r ExternalIPRoute) Source() (*net.IP, uint8) {
	ip := net.ParseIP(r.Service.Spec.ExternalIPs[0])
	return &ip, uint8(32)
}

func (r ExternalIPRoute) NextHop() *net.IP {
	return r.HostIP
}

func (r ExternalIPRoute) Describe() string {
	return fmt.Sprintf("ExternalIP:    %s/%s -> %s", r.Service.Namespace, r.Service.Name, r.HostIP)
}

func NewExternalIPRoute(service *v1.Service, hostIP *net.IP) RouteInterface {
	return ExternalIPRoute{Route{}, service, hostIP}
}

type NodePodSubnetRoute struct {
	Route
	Node *v1.Node
}

func NewNodePodSubnetRoute(node *v1.Node) RouteInterface {
	return NodePodSubnetRoute{Route{}, node}
}

func (r NodePodSubnetRoute) Source() (*net.IP, uint8) {
	subnet, _ := GetNodePodSubnet(r.Node)
	ip, ipnet, _ := net.ParseCIDR(subnet)
	prefixSize, _ := ipnet.Mask.Size()
	return &ip, uint8(prefixSize)
}

func (r NodePodSubnetRoute) NextHop() *net.IP {
	nexthop, _ := GetNodeInternalIP(r.Node)
	ip := net.ParseIP(nexthop)
	return &ip
}

func (r NodePodSubnetRoute) Describe() string {
	prefix, length := r.Source()
	return fmt.Sprintf("NodePodSubnet: %s/%v -> %s", prefix.To4().String(), length, r.Node.Name)
}

func GetNodeInternalIP(node *v1.Node) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			return address.Address, nil
		}
	}

	return "", fmt.Errorf("Node must have an InternalIP: %s", node.Name)
}

func GetNodePodSubnet(node *v1.Node) (string, error) {
	if l, ok := node.Annotations[AnnotationNodePodSubnet]; ok {
		return l, nil
	}

	return "", fmt.Errorf("Node must be annotated with %s", AnnotationNodePodSubnet)
}
