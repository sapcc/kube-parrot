package parrot

import (
	"fmt"
	"net"
	"strings"

	"github.com/golang/glog"
	"github.com/osrg/gobgp/gobgp/cmd"
	"github.com/osrg/gobgp/packet/bgp"
	"k8s.io/kubernetes/pkg/api"

	gobgpapi "github.com/osrg/gobgp/api"
	gobgp "github.com/osrg/gobgp/server"
)

func (feeder *Parrot) handleNodeCreate(obj interface{}) {
	node, ok := obj.(*api.Node)
	if !ok {
		glog.Errorf("Aborting. Expected to receive a node object. Apparently not.")
		return
	} else {
		glog.V(2).Infof("Node created: %s (%s)", node.GetName(), node.Status.Phase)
	}

	var internalIP net.IP = nil
	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeInternalIP {
			glog.V(2).Infof("Found internalIP %s for node %s", address.Address, node.GetName())
			internalIP = net.ParseIP(string(address.Address))
		}
	}

	if internalIP == nil {
		glog.Errorf("Aborting. No internalIP found for node: %s", node.GetName())
		return
	}

	podSubnetCidr := fmt.Sprintf("10.%d.0.0/24", internalIP.To4()[3])
	route := []string{podSubnetCidr, "nexthop", internalIP.String()}

	glog.Infof("Adding Route: %-42s # Pods for %s", strings.Join(route, " "), node.GetName())

	path, _ := cmd.ParsePath(bgp.RF_IPv4_UC, route)
	req := gobgp.NewGrpcRequest(gobgp.REQ_ADD_PATH, "", bgp.RouteFamily(0), &gobgpapi.AddPathRequest{
		Resource: gobgpapi.Resource_GLOBAL,
		Path:     path,
	})
	feeder.bgpServer.GrpcReqCh <- req
	res := <-req.ResponseCh
	if err := res.Err(); err != nil {
		glog.Errorf("Oops. Something went wrong adding the route: %s", err)
	}
}

func (feeder *Parrot) handleServiceCreate(obj interface{}) {
	if e, ok := obj.(*api.Node); ok {
		glog.Infof("Service created: %s (%s)", e.GetName(), e.Status.Phase)
	}
}

func (feeder *Parrot) handleEndpointCreate(obj interface{}) {
	if e, ok := obj.(*api.Node); ok {
		glog.Infof("Endpoint created: %s (%s)", e.GetName(), e.Status.Phase)
	}
}

func (feeder *Parrot) handleNodeUpdate(old interface{}, new interface{}) {
	oldPod, okOld := old.(*api.Node)
	newPod, okNew := new.(*api.Node)

	if okOld && okNew {
		glog.V(3).Infof("Node updated: %s", newPod.GetName())
	} else if okNew {
		feeder.handleNodeCreate(newPod)
	} else if okOld {
		feeder.handleNodeDelete(oldPod)
	}
}

func (feeder *Parrot) handleServiceUpdate(old interface{}, new interface{}) {
	oldPod, okOld := old.(*api.Node)
	newPod, okNew := new.(*api.Node)

	if okOld && okNew {
		glog.V(3).Infof("Service updated: %s", newPod.GetName())
	} else if okNew {
		feeder.handleServiceCreate(newPod)
	} else if okOld {
		feeder.handleServiceDelete(oldPod)
	}
}

func (feeder *Parrot) handleEndpointUpdate(old interface{}, new interface{}) {
	oldPod, okOld := old.(*api.Node)
	newPod, okNew := new.(*api.Node)

	if okOld && okNew {
		glog.V(3).Infof("Endpoint updated: %s", newPod.GetName())
	} else if okNew {
		feeder.handleServiceCreate(newPod)
	} else if okOld {
		feeder.handleServiceDelete(oldPod)
	}
}

func (feeder *Parrot) handleServiceDelete(obj interface{}) {
	if e, ok := obj.(*api.Node); ok {
		glog.V(3).Infof("Service deleted: %s", e)
	}
}

func (feeder *Parrot) handleEndpointDelete(obj interface{}) {
	if e, ok := obj.(*api.Node); ok {
		glog.V(3).Infof("Endpoint deleted: %s", e)
	}
}

func (feeder *Parrot) handleNodeDelete(obj interface{}) {
	if e, ok := obj.(*api.Node); ok {
		glog.V(3).Infof("Node deleted: %s", e)
	}
}
