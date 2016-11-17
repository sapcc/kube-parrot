package bgp

import (
	"fmt"
	"net"
	"time"

	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"
	"github.com/sapcc/kube-parrot/pkg/types"

	"k8s.io/client-go/1.5/pkg/api/v1"
)

type RouteInterface interface {
	Source() (net.IP, uint8)
	NextHop() net.IP
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
	Proxy   *v1.Pod
}

func (r ExternalIPRoute) Source() (net.IP, uint8) {
	return net.ParseIP(r.Service.Spec.ExternalIPs[0]), uint8(32)
}

func (r ExternalIPRoute) NextHop() net.IP {
	return net.ParseIP(r.Proxy.Status.HostIP)
}

func (r ExternalIPRoute) Describe() string {
	return fmt.Sprintf("ExternalIP:    %s/%s -> %s/%s", r.Service.Namespace, r.Service.Name, r.Proxy.Namespace, r.Proxy.Name)
}

type NodePodSubnetRoute struct {
	Route
	Node *v1.Node
}

func (r NodePodSubnetRoute) Source() (net.IP, uint8) {
	subnet, _ := GetNodePodSubnet(r.Node)
	ip, ipnet, _ := net.ParseCIDR(subnet)
	prefixSize, _ := ipnet.Mask.Size()
	return ip, uint8(prefixSize)
}

func (r NodePodSubnetRoute) NextHop() net.IP {
	nexthop, _ := GetNodeInternalIP(r.Node)
	return net.ParseIP(nexthop)
}

func (r NodePodSubnetRoute) Describe() string {
	prefix, length := r.Source()
	return fmt.Sprintf("NodePodSubnet: %s/%v -> %s", prefix.To4().String(), length, r.Node.Name)
}

type NodeServiceSubnetRoute struct {
	Route
	Proxy         *v1.Pod
	ServiceSubnet net.IPNet
}

func (r NodeServiceSubnetRoute) Source() (net.IP, uint8) {
	prefixSize, _ := r.ServiceSubnet.Mask.Size()
	return r.ServiceSubnet.IP, uint8(prefixSize)
}

func (r NodeServiceSubnetRoute) NextHop() net.IP {
	return net.ParseIP(r.Proxy.Status.HostIP)
}

func (r NodeServiceSubnetRoute) Describe() string {
	prefix, length := r.Source()
	return fmt.Sprintf("NodeServiceSubnet: %s/%v -> %s", prefix.To4().String(), length, r.Proxy.Name)
}

type APIServerRoute struct {
	Route
	APIServer *v1.Pod
	masterIP  net.IP
}

func (r APIServerRoute) Source() (net.IP, uint8) {
	return r.masterIP, 32
}

func (r APIServerRoute) NextHop() net.IP {
	return net.ParseIP(r.APIServer.Status.HostIP)
}

func (r APIServerRoute) Describe() string {
	return fmt.Sprintf("APIServer:     %s/%s -> %s", r.APIServer.Namespace, r.APIServer.Name, r.masterIP)
}

func NewNodePodSubnetRoute(node *v1.Node) RouteInterface {
	return NodePodSubnetRoute{Route{}, node}
}

func NewNodeServiceSubnetRoute(proxy *v1.Pod, subnet net.IPNet) RouteInterface {
	return NodeServiceSubnetRoute{Route{}, proxy, subnet}
}

func NewExternalIPRoute(service *v1.Service, proxy *v1.Pod) RouteInterface {
	return ExternalIPRoute{Route{}, service, proxy}
}

func NewAPIServerRoute(apiserver *v1.Pod, masterIP net.IP) RouteInterface {
	return APIServerRoute{Route{}, apiserver, masterIP}
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
	if l, ok := node.Annotations[types.AnnotationNodePodSubnet]; ok {
		return l, nil
	}

	return "", fmt.Errorf("Node must be annotated with %s", types.AnnotationNodePodSubnet)
}
