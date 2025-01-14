// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServeMetrics starts the Prometheus metrics collector.
func ServeMetrics(host net.IP, port int, wg *sync.WaitGroup, stop <-chan struct{}) {
	wg.Add(1)
	defer wg.Done()

	addr := fmt.Sprintf("%s:%d", host.String(), port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		glog.Errorf("Failed to serve Prometheus metrics: %v", err)
		return
	}
	defer l.Close()
	glog.Infof("Serving Prometheus metrics on %s", addr)

	go http.Serve(l, promhttp.Handler())
	<-stop
}
