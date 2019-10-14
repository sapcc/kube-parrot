package main

import (
	utiljson "encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/glog"

	"github.com/sapcc/kube-parrot/pkg/parrot"
)

var opts parrot.Options

func main() {
	if err := config.mergeConfig(); err != nil {
		glog.Fatalf("Couldn't read config file: %s", err)
	}

	config.mergeFlags()
	json, _ := utiljson.Marshal(config)
	fmt.Println("using config: %s", string(json))

	if err := config.validate(); err != nil {
		glog.Fatalf("Couldn't validate config: %s", err)
	}

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	opts.PodCIDR = config.PodCIDR
	opts.Neighbors = getNeighbors(opts.HostIP.To4())
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
	n1 := make(net.IP, len(local))
	n2 := make(net.IP, len(local))
	copy(n1, local)
	copy(n2, local)

	n1[3] = n1[3] - 1
	n2[3] = n2[3] - 2

	return []*net.IP{&n1, &n2}
}
