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
	"github.com/sapcc/kube-parrot/pkg/metrics"
	"github.com/sapcc/kube-parrot/pkg/parrot"
	"github.com/sapcc/go-traceroute/traceroute"
	"golang.org/x/net/context"
	flag "github.com/spf13/pflag"
)

type Neighbors []*net.IP

var opts parrot.Options
var TraceCount int
var neighbors Neighbors

func init() {
	flag.IntVar(&opts.As, "as", 65000, "global AS")
	flag.StringVar(&opts.NodeName, "nodename", "", "Name of the node this pod is running on")
	flag.IPVar(&opts.HostIP, "hostip", net.ParseIP("127.0.0.1"), "IP")
	flag.IntVar(&opts.MetricsPort, "metric-port", 30039, "Port for Prometheus metrics")
	flag.Var(&neighbors, "neighbor", "IP address of a neighbor. Can be specified multiple times...")
	flag.IntVar(&TraceCount, "traceroute-count", 10, "Amount of traceroute packets to send with ttl of 1 for dynamic neighbor discovery")
}

func main() {
	goflag.CommandLine.Parse([]string{})
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

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
	parrot.Run(stop, wg)

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

	var h []string
	for i := 0; i < TraceCount; i++ {
		dst := fmt.Sprintf("1.1.1.%v", i)
		err := t.Trace(context.Background(), net.ParseIP(dst), func(reply *traceroute.Reply) {
			hop := reply.IP

			h = append(h, hop.String())

		})
		if err != nil {
			glog.Fatal(err)
		}
	}
	hops := unique(h)

	var neigh []*net.IP
	for _, n := range hops {
		ip := net.ParseIP(n)
		neigh = append(neigh, &ip)
	}
	return neigh
}


// unique removes duplicate values from slice of strings
func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
