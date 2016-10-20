package controller

import (
	"fmt"
	"net"
	"time"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/packet/bgp"
	"github.com/osrg/gobgp/table"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/cache"
)

func NoResyncPeriodFunc() time.Duration {
	return 0
}

func getKey(obj interface{}) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return "unkown"
	}

	return key
}

func getPodSubnetRoute(node *v1.Node) (*table.Path, error) {
	nodeIP, err := getNodeIP(node)
	if err != nil {
		return nil, err
	}

	podSubnet, err := getPodSubnet(node)
	if err != nil {
		return nil, err
	}

	prefix, _ := podSubnet.Mask.Size()
	nlri := bgp.NewIPAddrPrefix(uint8(prefix), podSubnet.IP.String())

	pattr := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
		bgp.NewPathAttributeNextHop(nodeIP.String()),
	}

	return table.NewPath(nil, nlri, false, pattr, time.Now(), false), nil
}



func getNodeIP(node *v1.Node) (net.IP, error) {
	var nodeIP net.IP
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			nodeIP = net.ParseIP(address.Address)
		}
	}

	if nodeIP == nil {
		return nil, fmt.Errorf("Couldn't get internalIP for %s", node.GetName())
	}

	return nodeIP, nil
}

func getPodSubnet(node *v1.Node) (*net.IPNet, error) {
	nodeIP, err := getNodeIP(node)
	if err != nil {
		return nil, err
	}

	_, net, err := net.ParseCIDR(fmt.Sprintf("10.%d.0.0/24", nodeIP.To4()[3]))
	return net, err
}
