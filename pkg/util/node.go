package util

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func GetNodeInternalIP(node *v1.Node) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			return address.Address, nil
		}
	}

	return "", fmt.Errorf("Node must have an InternalIP: %s", node.Name)
}