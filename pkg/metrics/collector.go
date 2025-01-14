// Copyright 2025 SAP SE
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sapcc/kube-parrot/pkg/bgp"
	"net"
)

var sessionStati = []string{"idle", "connect", "active", "opensent", "openconfirm", "established"}

type collector struct {
	nodeName  string
	neighbors []*net.IP
	bgpServer *bgp.Server

	bgpServerErrorsTotal,
	bgpNeighborsSessionStatusMetric,
	bgpNeighborAdvertisedRouteCountTotalMetric *prometheus.Desc
}

// RegisterCollector registers a new Prometheus metrics collector.
func RegisterCollector(nodeName string, neighbors []*net.IP, bgpServer *bgp.Server) {
	prometheus.MustRegister(
		newCollector(nodeName, neighbors, bgpServer),
	)
}

func newCollector(nodeName string, neighbors []*net.IP, bgpServer *bgp.Server) *collector {
	return &collector{
		nodeName:  nodeName,
		neighbors: neighbors,
		bgpServer: bgpServer,
		bgpServerErrorsTotal: prometheus.NewDesc(
			"kube_parrot_bgp_server_errors_total",
			"Counter for BGP server errors.",
			[]string{"node"},
			nil,
		),
		bgpNeighborsSessionStatusMetric: prometheus.NewDesc(
			"kube_parrot_bgp_neighbor_session_status",
			"Session status of BGP neighbors.",
			[]string{"node", "neighbor", "status"},
			nil,
		),
		bgpNeighborAdvertisedRouteCountTotalMetric: prometheus.NewDesc(
			"kube_parrot_bgp_neighbor_advertised_route_count_total",
			"Total count of advertised routes to BGP neighbor.",
			[]string{"node", "neighbor"},
			nil,
		),
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.bgpServerErrorsTotal
	ch <- c.bgpNeighborsSessionStatusMetric
	ch <- c.bgpNeighborAdvertisedRouteCountTotalMetric
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	for _, neighbor := range c.neighbors {
		neighborList, err := c.bgpServer.GetNeighbor(neighbor.String())
		if err != nil {
			glog.Infof("failed to get session status for BGP neighbor: %v", err)
			ch <- prometheus.MustNewConstMetric(
				c.bgpServerErrorsTotal,
				prometheus.CounterValue,
				1,
				c.nodeName,
			)
			continue
		}

		for _, n := range neighborList {
			// Report BGP sessions status metrics.
			for _, status := range sessionStati {
				ch <- prometheus.MustNewConstMetric(
					c.bgpNeighborsSessionStatusMetric,
					prometheus.GaugeValue,
					boolToFloat64(n.GetInfo().GetBgpState() == status),
					c.nodeName,
					neighbor.String(),
					status,
				)
			}

			// Report count of advertised routes.
			ch <- prometheus.MustNewConstMetric(
				c.bgpNeighborAdvertisedRouteCountTotalMetric,
				prometheus.GaugeValue,
				float64(n.GetInfo().GetAdvertised()),
				c.nodeName,
				neighbor.String(),
			)

		}
	}
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
