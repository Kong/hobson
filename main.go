package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

type RecordEntry struct {
	Addresses []string
	Service   string
}

var (
	version = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version number")
	configPath := flag.String("config", "", "Config file path")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *configPath == "" {
		log.Fatalln("-config must be set")
	}

	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalln("Error loading config:", err)
	}

	log.Printf("Hobson %v started", version)

	srv := &dns.Server{Addr: config.Bind, Net: "udp"}
	h := NewDNSHandler(config.Zone)
	srv.Handler = h

	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		defer wg.Done()
		log.Println("Answer queries for zone", config.Zone)
		log.Println("Starting DNS server on", config.Bind)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	svcs := config.Services
	notify := make(chan *RecordEntry)
	for _, svc := range svcs {
		go func(s string, n chan *RecordEntry) {
			ServiceMonitorRunning.WithLabelValues(s).Set(1)
			defer ServiceMonitorRunning.WithLabelValues(s).Set(0)
			monitor(s, n)
		}(svc, notify)

	}

	ms := NewMetricsServer(&MetricsServerConfig{
		ListenAddress: config.MetricsListen,
	})
	ms.RegisterMetrics()

	go func() {
		wg.Add(1)
		defer wg.Done()
		log.Println("Starting metrics server on", config.MetricsListen)
		if err := ms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start metrics server %s\n", err.Error())
		}
	}()

	log.Printf("Beginning monitoring of Consul services (%s)",
		strings.Join(config.Services, ","))

	go func() {
		for {
			a := <-notify
			t := a.Addresses

			if len(t) == 0 {
				log.Printf("No records for service %q", a.Service)
				continue
			}

			sort.Strings(t)
			h.UpdateRecord(a.Service, t)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	waitCh := make(chan struct{})

	go func() {
		if err := srv.ShutdownContext(ctx); err != nil {
			log.Println("Error shutting down DNS server:", err)
		}
	}()

	go func() {
		if err := ms.ShutdownContext(ctx); err != nil {
			log.Println("Error shutting down DNS server:", err)
		}
	}()

	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-ctx.Done():
		log.Fatalln("Timeout while shutting down server")
	case <-waitCh:
	}
}
