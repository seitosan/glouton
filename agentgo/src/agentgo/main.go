package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agentgo/api"
	"agentgo/collector"
	"agentgo/discovery"
	"agentgo/facts"
	"agentgo/inputs/cpu"
	"agentgo/inputs/disk"
	"agentgo/inputs/diskio"
	"agentgo/inputs/docker"
	"agentgo/inputs/mem"
	"agentgo/inputs/net"
	"agentgo/inputs/process"
	"agentgo/inputs/swap"
	"agentgo/inputs/system"
	"agentgo/store"
	"agentgo/version"

	"github.com/influxdata/telegraf"
)

func panicOnError(i telegraf.Input, err error) telegraf.Input {
	if err != nil {
		log.Fatalf("%v", err)
	}
	return i
}

func main() {
	log.Printf("Starting agent version %v (commit %v)", version.Version, version.BuildHash)

	apiBindAddress := os.Getenv("API_ADDRESS")
	if apiBindAddress == "" {
		apiBindAddress = ":8015"
	}

	db := store.New()
	dockerFact := facts.NewDocker()
	psFact := facts.NewProcess(dockerFact)
	netstat := &facts.NetstatProvider{}
	factProvider := facts.NewFacter(
		"",
		"/",
		"https://myip.bleemeo.com",
	)
	factProvider.AddCallback(dockerFact.DockerFact)
	factProvider.SetFact("installation_format", "golang")
	factProvider.SetFact("statsd_enabled", "false")
	api := api.New(db, dockerFact, psFact, factProvider, apiBindAddress)
	coll := collector.New(db.Accumulator())

	coll.AddInput(panicOnError(system.New()))
	coll.AddInput(panicOnError(process.New()))
	coll.AddInput(panicOnError(cpu.New()))
	coll.AddInput(panicOnError(mem.New()))
	coll.AddInput(panicOnError(swap.New()))
	coll.AddInput(panicOnError(net.New(
		[]string{
			"docker",
			"lo",
			"veth",
			"virbr",
			"vnet",
			"isatap",
		},
	)))
	coll.AddInput(panicOnError(disk.New("/", nil)))
	coll.AddInput(panicOnError(diskio.New(
		[]string{
			"sd?",
			"nvme.*",
		},
	)))
	coll.AddInput(panicOnError(docker.New()))

	disc := discovery.New(
		discovery.NewDynamic(psFact, netstat, dockerFact),
		coll,
		nil,
	)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		db.Run(ctx)
		db.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, err := disc.Discovery(ctx, 0)
			if err != nil {
				log.Printf("DBG: error during discovery: %v", err)
			}
			select {
			case <-time.After(60 * time.Second):
			case <-ctx.Done():
				return

			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dockerFact.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		coll.Run(ctx)
	}()

	go api.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c
	cancel()
	wg.Wait()
}
