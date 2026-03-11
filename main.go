package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/IllicLanthresh/vertex/internal/config"
	"github.com/IllicLanthresh/vertex/internal/traffic"
	"github.com/IllicLanthresh/vertex/internal/tui"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	var (
		headless   = flag.Bool("headless", false, "Run without TUI (auto-starts traffic with embedded config)")
		configFile = flag.String("config", "", "External config file (optional)")
		version    = flag.Bool("version", false, "Show version information")
		devices    = flag.Int("devices", 0, "Virtual devices per interface (overrides config)")
		depth      = flag.Int("depth", 0, "Max crawl depth (overrides config)")
		minSleep   = flag.Int("min-sleep", 0, "Min seconds between fetches (overrides config)")
		maxSleep   = flag.Int("max-sleep", 0, "Max seconds between fetches (overrides config)")
		timeout    = flag.Int("timeout", 0, "HTTP request timeout in seconds (overrides config)")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Vertex %s (built %s)\n", Version, BuildTime)
		return
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if *devices > 0 {
		cfg.NetworkSimulation.VirtualDevices = *devices
	}
	if *depth > 0 {
		cfg.MaxDepth = *depth
	}
	if *minSleep > 0 {
		cfg.MinSleep = *minSleep
	}
	if *maxSleep > 0 {
		cfg.MaxSleep = *maxSleep
	}
	if *timeout > 0 {
		cfg.Timeout = *timeout
	}

	generator := traffic.NewGenerator(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if *headless {
		log.Printf("Starting Vertex %s in headless mode", Version)

		if err := generator.Start(ctx); err != nil {
			log.Fatalf("Failed to start traffic generator: %v", err)
		}

		<-sigChan
		log.Println("Shutting down...")
		cancel()
		generator.Stop()
		return
	}

	go func() {
		<-sigChan
		cancel()
	}()

	app := tui.New(cfg, generator)
	if err := app.Run(ctx); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
