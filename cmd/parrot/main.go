package main

import (
	"net"
	"sync"

	goflag "flag"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"os"
	"os/signal"
	"syscall"

	"github.com/sapcc/kube-parrot/pkg/parrot"
)

type Neighbors []*net.IP

var opts parrot.Options
var neighbors Neighbors

func init() {
	flag.StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")
	flag.IntVar(&opts.As, "as", 65000, "global AS")
	flag.IPVar(&opts.LocalAddress, "local_address", net.ParseIP("127.0.0.1"), "local IP address")
}

func main() {
	goflag.CommandLine.Parse([]string{})
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	opts.Neighbors = getNeighbors(opts.LocalAddress)
	opts.GrpcPort = 12345
	parrot := parrot.New(opts)

	wg := &sync.WaitGroup{}
	parrot.Run(stop, wg)

	<-sigs      // Wait for signals
	close(stop) // Stop all goroutines
	wg.Wait()   // Wait for all to be stopped

	glog.V(2).Infof("Shutdown Completed. Bye!")
}

func getNeighbors(local net.IP) []*net.IP {
	n1 := local.To4()
	n2 := local.To4()

	n1[3] = n1[3] - 1
	n2[3] = n2[3] - 2

	return []*net.IP{&n1, &n2}
}
