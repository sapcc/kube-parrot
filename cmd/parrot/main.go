package main

import (
	goflag "flag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/glog"
	"github.com/sapcc/kube-parrot/pkg/metrics"
	"github.com/sapcc/kube-parrot/pkg/parrot"
	flag "github.com/spf13/pflag"
)

var opts parrot.Options

func init() {
	flag.IntVar(&opts.As, "as", 65000, "global AS")
	flag.StringVar(&opts.NodeName, "nodename", "", "Name of the node this pod is running on")
	flag.IPVar(&opts.HostIP, "hostip", net.ParseIP("127.0.0.1"), "IP")
	flag.IntVar(&opts.MetricsPort, "metric-port", 30039, "Port for Prometheus metrics")
}

func main() {
	goflag.CommandLine.Parse([]string{})
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	opts.Neighbors = getNeighbors(opts.HostIP.To4())
	opts.GrpcPort = 12345
	parrot := parrot.New(opts)

	wg := &sync.WaitGroup{}
	parrot.Run(stop, wg)

	go metrics.ServeMetrics(opts.HostIP, opts.MetricsPort, wg, stop)

	<-sigs      // Wait for signals
	close(stop) // Stop all goroutines
	wg.Wait()   // Wait for all to be stopped

	glog.V(2).Infof("Shutdown Completed. Bye!")
}

func getNeighbors(local net.IP) []*net.IP {
	n1 := make(net.IP, len(local))
	n2 := make(net.IP, len(local))
	copy(n1, local)
	copy(n2, local)

	n1[3] = n1[3] - 1
	n2[3] = n2[3] - 2

	return []*net.IP{&n1, &n2}
}
