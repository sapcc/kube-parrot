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

	bgpNeighborsTotalMetric,
	bgpNeighborsSessionStatusMetric *prometheus.Desc
}

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
		bgpNeighborsSessionStatusMetric: prometheus.NewDesc(
			"kube_parrot_bgp_neighbor_session_status",
			"Session status of BGP neighbors.",
			[]string{"node", "neighbor", "status"},
			nil,
		),
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.bgpNeighborsSessionStatusMetric
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	for _, neighbor := range c.neighbors {
		sessionStatus, err := c.bgpServer.GetNeighborBgpState(neighbor.String())
		if err != nil {
			glog.Infof("failed to get session status for BGP neighbor: %v", err)
			continue
		}

		for _, status := range sessionStati {
			ch <- prometheus.MustNewConstMetric(
				c.bgpNeighborsSessionStatusMetric,
				prometheus.GaugeValue,
				boolToFloat64(sessionStatus == status),
				c.nodeName,
				neighbor.String(),
				status,
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
