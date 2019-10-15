package util

import (
	"fmt"
	"io/ioutil"
	"os"

	utiljson "encoding/json"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	AnnotationNodePodSubnet = "parrot.sap.cc/podsubnet"
	ConfigPath              = "/etc/kube-parrot/config"
)

var config *Config

func GetNodeInternalIP(node *v1.Node) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			return address.Address, nil
		}
	}

	return "", fmt.Errorf("Node must have an InternalIP: %s", node.Name)
}

func GetNodePodSubnet(node *v1.Node) (string, error) {
	if c, err := getNodePodSubnetFromConfig(); err == nil {
		return c, err
	}

	if l, ok := node.Annotations[AnnotationNodePodSubnet]; ok {
		return l, nil
	}

	return "", fmt.Errorf("Node must be annotated with %s", AnnotationNodePodSubnet)
}

func getNodePodSubnetFromConfig() (string, error) {
	if config != nil {
		return config.PodCIDR, nil
	}

	c := &Config{}
	if err := c.load(); err != nil {
		return "", err
	}
	config = c

	return c.PodCIDR, nil
}

type Config struct {
	PodCIDR string `json:"podCIDR"`
}

func (c *Config) load() error {
	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("no config file found at %q", ConfigPath)
	}
	glog.V(2).Infof("config file found at %q", ConfigPath)

	yaml, err := ioutil.ReadFile(ConfigPath)
	if err != nil {
		return fmt.Errorf("couldn't read config file %q: %s", ConfigPath, err)
	}

	json, err := utilyaml.ToJSON(yaml)
	if err != nil {
		return fmt.Errorf("couldn't parse config file %q: %s", ConfigPath, err)
	}

	if err = utiljson.Unmarshal(json, c); err != nil {
		return fmt.Errorf("couldn't unmarshal config file %q: %s", ConfigPath, err)
	}

	return nil
}
