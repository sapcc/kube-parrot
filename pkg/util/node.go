package util

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

const (
	AnnotationNodePodSubnet = "parrot.sap.cc/podsubnet"
)

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
