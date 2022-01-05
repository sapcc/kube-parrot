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
	ConfigPath              = "/etc/kubernetes/kube-parrot/config"
)

var config *Config

type Config struct {
	PodCIDR string `json:"podCIDR"`
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
	if config == nil {
		c, err := loadConfig()
		if err != nil {
			glog.Errorf("Couldn't read config file: %s", err)
		}
		config = c
	}

	if config.PodCIDR != "" {
		return config.PodCIDR, nil
	}

	if l, ok := node.Annotations[AnnotationNodePodSubnet]; ok {
		return l, nil
	}

	return "", fmt.Errorf("Couldn't figure out nodes PodCIDR. Set annotation or configfile.")
}

func loadConfig() (*Config, error) {
	c := &Config{}

	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		return c, fmt.Errorf("to advertise node pod subnet, provide config at %q", ConfigPath)
	}
	glog.V(2).Infof("config file found at %q", ConfigPath)

	yaml, err := ioutil.ReadFile(ConfigPath)
	if err != nil {
		return c, fmt.Errorf("couldn't read config file %q: %s", ConfigPath, err)
	}

	json, err := utilyaml.ToJSON(yaml)
	if err != nil {
		return c, fmt.Errorf("couldn't parse config file %q: %s", ConfigPath, err)
	}

	if err = utiljson.Unmarshal(json, c); err != nil {
		return c, fmt.Errorf("couldn't unmarshal config file %q: %s", ConfigPath, err)
	}

	return c, nil
}
