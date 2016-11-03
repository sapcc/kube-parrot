package bgp

import (
	"fmt"

	"github.com/sapcc/kube-parrot/pkg/types"

	"k8s.io/client-go/1.5/pkg/api/v1"
)

type RouteInterface interface {
	SourceCIDR() string
	NextHop() string
	Describe() string
}


type Route struct {
	RouteInterface
}

func (r Route) String() string {
	return fmt.Sprintf("%18s -> %-15s (%s)", r.SourceCIDR(), r.NextHop(), r.Describe())
}


type ExternalIPRoute struct {
	Service *v1.Service
	Proxy   *v1.Pod
}

func (r ExternalIPRoute) SourceCIDR() string {
	return fmt.Sprintf("%s/32", r.Service.Spec.ExternalIPs[0])
}

func (r ExternalIPRoute) NextHop() string {
	return r.Proxy.Status.HostIP
}

func (r ExternalIPRoute) Describe() string {
	return fmt.Sprintf("ExternalIP: %s/%s -> %s/%s", r.Service.Namespace, r.Service.Name, r.Proxy.Namespace, r.Proxy.Name)
}


type NodePodSubnetRoute struct {
	Node *v1.Node
}

func (r NodePodSubnetRoute) SourceCIDR() string {
	subnet, _ := GetNodePodSubnet(r.Node)
	return subnet
}

func (r NodePodSubnetRoute) NextHop() string {
	nexthop, _ := GetNodeInternalIP(r.Node)
	return nexthop
}

func (r NodePodSubnetRoute) Describe() string {
	return fmt.Sprintf("NodePodSubnet: %s -> %s", r.SourceCIDR(), r.Node.Name)
}


func NewNodePodSubnetRoute(node *v1.Node) RouteInterface {
	return Route{NodePodSubnetRoute{node}}
}

func NewExternalIPRoute(service *v1.Service, proxy *v1.Pod) RouteInterface {
	return Route{ExternalIPRoute{service, proxy}}
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
