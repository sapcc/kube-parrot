package main

import (
	"fmt"
	"net"

	goflag "flag"

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
	done := make(chan bool, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	opts.Neighbors = neighbors
	parrot := parrot.New(opts)

	go func() {
		parrot.Start()
	}()

	go func() {
		<-sigs
		parrot.Stop()
		done <- true
	}()

	<-done
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
