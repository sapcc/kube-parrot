package main

import (
	goflag "flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/sapcc/go-traceroute/traceroute"
	"github.com/sapcc/kube-parrot/pkg/metrics"
	"github.com/sapcc/kube-parrot/pkg/parrot"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/context"
)

type Neighbors []*net.IP

var opts parrot.Options
var neighbors Neighbors

func init() {
	flag.IntVar(&opts.As, "as", 65000, "local BGP ASN")
	flag.IntVar(&opts.RemoteAs, "remote-as", 0, "remote BGP ASN. Default to local ASN (iBGP)")
	flag.StringVar(&opts.NodeName, "nodename", "", "Name of the node this pod is running on")
	flag.IPVar(&opts.HostIP, "hostip", net.ParseIP("127.0.0.1"), "IP")
	flag.IntVar(&opts.MetricsPort, "metric-port", 30039, "Port for Prometheus metrics")
	flag.Var(&neighbors, "neighbor", "IP address of a neighbor. Can be specified multiple times...")
	flag.IntVar(&opts.TraceCount, "traceroute-count", 10, "Amount of traceroute packets to send with ttl of 1 for dynamic neighbor discovery")
	flag.IntVar(&opts.NeighborCount, "neighbor-count", 2, "Amount of expected BGP neighbors. Used with dynamic neighbor discovery")
	flag.BoolVar(&opts.PodSubnet, "podsubnet", true, "Announce node podCIDR")
}

func main() {
	goflag.CommandLine.Parse([]string{})
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	if opts.RemoteAs == 0 {
		opts.RemoteAs = opts.As
	}

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if neighbors != nil {
		opts.Neighbors = neighbors
	} else {
		opts.Neighbors = getNeighbors()
	}
	opts.GrpcPort = 12345
	parrot := parrot.New(opts)

	wg := &sync.WaitGroup{}
	parrot.Run(opts, stop, wg)

	go metrics.ServeMetrics(opts.HostIP, opts.MetricsPort, wg, stop)

	<-sigs      // Wait for signals
	close(stop) // Stop all goroutines
	wg.Wait()   // Wait for all to be stopped

	glog.V(2).Infof("Shutdown Completed. Bye!")
}

func (f *Neighbors) String() string {
	return fmt.Sprintf("%v", *f)
}

func (i *Neighbors) Set(value string) error {
	ip := net.ParseIP(value)
	if ip == nil {
		return fmt.Errorf("%v is not a valid IP address", value)
	}

	*i = append(*i, &ip)
	return nil
}

func (s *Neighbors) Type() string {
	return "neighborSlice"
}

// getNeighbors discovers next-hops by sending traceroute packets with ttl=1
func getNeighbors() []*net.IP {
	t := &traceroute.Tracer{
		Config: traceroute.Config{
			Delay:    50 * time.Millisecond,
			Timeout:  time.Second,
			MaxHops:  1,
			Count:    1,
			Networks: []string{"ip4:icmp", "ip4:ip"},
		},
	}
	defer t.Close()

	h := make(map[string]struct{})
	for i := 0; i < opts.TraceCount; i++ {
		dst := fmt.Sprintf("1.1.1.%v", i)
		err := t.Trace(context.Background(), net.ParseIP(dst), func(reply *traceroute.Reply) {

			h[reply.IP.String()] = struct{}{}
		})
		if err != nil {
			glog.Fatal(err)
		}
	}

	var neigh []*net.IP
	for k, _ := range h {
		ip := net.ParseIP(k)
		neigh = append(neigh, &ip)
	}
	if len(neigh) != opts.NeighborCount {
		glog.Fatalf("Discovered %d neighbors: %v, but was expecting %d neighbors. Exiting", len(neigh), neigh, opts.NeighborCount)
	}
	return neigh
}
