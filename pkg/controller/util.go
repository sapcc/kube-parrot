package controller

import (
	"fmt"
	"net"

	"k8s.io/kubernetes/pkg/api"
)

func getPodSubnetRoute(node *api.Node) ([]string, error) {
	nodeIP, err := getNodeIP(node)
	if err != nil {
		return nil, err
	}

	podSubnet, err := getPodSubnet(node)
	if err != nil {
		return nil, err
	}

	return []string{podSubnet.String(), "nexthop", nodeIP.String()}, nil
}

func getNodeIP(node *api.Node) (net.IP, error) {
	var nodeIP net.IP
	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeInternalIP {
			nodeIP = net.ParseIP(address.Address)
		}
	}

	if nodeIP == nil {
		return nil, fmt.Errorf("Couldn't get internalIP for %s", node.GetName())
	}

	return nodeIP, nil
}

func getPodSubnet(node *api.Node) (*net.IPNet, error) {
	nodeIP, err := getNodeIP(node)
	if err != nil {
		return nil, err
	}

	_, net, err := net.ParseCIDR(fmt.Sprintf("10.%d.0.0/24", nodeIP.To4()[3]))
	return net, err
}
