// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package parrot

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println("kube-parrot can now only run in-cluster as a sidecar to kube-proxy. over and out.")
		os.Exit(-1)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	glog.V(3).Infof("Using Kubernetes Api at %s", config.Host)
	return client
}
