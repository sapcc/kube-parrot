package main

import (
	utiljson "encoding/json"
	goflag "flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	configPath = "/etc/kube-parrot/config"
)

var (
	config = NewConfig()
)

func init() {
	flag.BoolVar(&config.AutoNeighbors, "auto-neighbors", true, `Calculate BGP neighbors`)
	flag.StringVar(&config.LocalAddress, "local-address", "127.0.0.1", `Local interface address`)
	flag.StringVar(&config.NodeName, "node-name", "", `Name of the node which podCIDR will be announced`)
	flag.StringVar(&config.PodCIDR, "node-pod-cidr", "", `Overwrites the PodCIDR to announce`)
	flag.IntVar(&config.AS, "bgp-as", 65000, `BGP AS`)
	flag.StringVar(&config.RouterID, "bgp-router-id", "127.0.0.1", `BGP Router ID. Defaults to local-address`)
	flag.StringSliceVar(&config.Neighbors, "bgp-neighbor", []string{}, `Manually configured BGP neighbors`)
}

type Config struct {
	AutoNeighbors bool     `json:"autoNeighbors`
	LocalAddress  string   `json:"localAddress`
	NodeName      string   `json:"nodeName"`
	PodCIDR       string   `json:"podCIDR"`
	AS            int      `json:"as`
	RouterID      string   `json:"routerID"`
	Neighbors     []string `json:"neighbors`
}

func NewConfig() *Config {
	return &Config{
		AutoNeighbors: true,
		LocalAddress:  "127.0.0.1",
		NodeName:      "",
		PodCIDR:       "",
		AS:            65000,
		RouterID:      "127.0.0.1",
		Neighbors:     []string{},
	}
}

func (c *Config) mergeConfig() error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		glog.V(2).Infof("no config file found at %q", configPath)
		return nil
	}
	glog.V(2).Infof("config file found at %q", configPath)

	// file exists, read and parse file
	yaml, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	json, err := utilyaml.ToJSON(yaml)
	if err != nil {
		return err
	}

	// Only overwrites fields provided in JSON
	if err = utiljson.Unmarshal(json, c); err != nil {
		return err
	}

	return nil
}

func (c *Config) mergeFlags() {
	goflag.CommandLine.Parse([]string{})
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
}

func (c *Config) validate() error {
	if err := validateCIDR(c.PodCIDR); err != nil {
		return err
	}
	return nil
}

const cidrParseErrFmt = "CIDR %q could not be parsed, %v"
const cidrAlignErrFmt = "CIDR %q is not aligned to a CIDR block, ip: %q network: %q"

func validateCIDR(cidr string) error {
	// parse test
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf(cidrParseErrFmt, cidr, err)
	}
	// alignment test
	if !ip.Equal(ipnet.IP) {
		return fmt.Errorf(cidrAlignErrFmt, cidr, ip, ipnet.String())
	}
	return nil
}
