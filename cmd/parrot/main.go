package main

import (
	"net"

	flag "github.com/spf13/pflag"

	"os"
	"os/signal"
	"syscall"

	"github.com/sapcc/routing-controller/pkg/parrot"
)

var opts parrot.Options

func init() {
	flag.IntVar(&opts.As, "as", 65000, "global AS")
	flag.IPVar(&opts.LocalAddress, "local_address", net.ParseIP("127.0.0.1"), "local IP address")
	flag.Var(&opts.Neighbors, "neighbor", "IP address of a neighbor. Can be specified multiple times...")
}

func main() {
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

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
