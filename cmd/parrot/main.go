package main

import (
	"fmt"
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
	flag.IntVar(&opts.As, "as", 65000, "global AS")
	flag.IPVar(&opts.LocalAddress, "local_address", net.ParseIP("127.0.0.1"), "local IP address")
	flag.Var(&neighbors, "neighbor", "IP address of a neighbor. Can be specified multiple times...")
}

func main() {
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	opts.Neighbors = neighbors
	parrot := parrot.New(opts)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		parrot.Run(stop, wg)
	}()

	go func() {
		<-sigs
		glog.V(2).Infof("Got signal. Shuting down")
		close(stop)
	}()

	wg.Wait()
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
