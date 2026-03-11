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
