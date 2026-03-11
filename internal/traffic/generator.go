package traffic

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/IllicLanthresh/vertex/internal/config"
	"github.com/IllicLanthresh/vertex/internal/interfaces"
)

// Generator manages traffic generation across multiple interfaces
type Generator struct {
	mu           sync.RWMutex
	config       *config.Config
	ifaceManager *interfaces.Manager
	crawlers     map[string][]*Crawler
	running      bool
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewGenerator creates a new traffic generator
func NewGenerator(cfg *config.Config) *Generator {
	return &Generator{
		config:       cfg,
		ifaceManager: interfaces.NewManager(),
		crawlers:     make(map[string][]*Crawler),
	}
}

// Start begins traffic generation
func (g *Generator) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return fmt.Errorf("traffic generator is already running")
	}

	log.Println("Starting traffic generator...")

	// Discover physical interfaces
	if err := g.ifaceManager.DiscoverPhysicalInterfaces(); err != nil {
		return fmt.Errorf("failed to discover interfaces: %w", err)
	}

	physicalIfaces := g.ifaceManager.GetPhysicalInterfaces()
	if len(physicalIfaces) == 0 {
		return fmt.Errorf("no physical interfaces found")
	}

	// Create virtual devices for enabled interfaces
	g.stopChan = make(chan struct{})

	for _, iface := range physicalIfaces {
		ifaceConfig, exists := g.config.NetworkSimulation.Interfaces[iface]
		if !exists {
			// Use default configuration
			ifaceConfig = config.InterfaceConfig{
				Enabled:        true,
				VirtualDevices: g.config.NetworkSimulation.VirtualDevices,
				IPConfig:       g.config.NetworkSimulation.IPConfig,
			}
		}

		if !ifaceConfig.Enabled {
			log.Printf("Skipping disabled interface: %s", iface)
			continue
		}

		// Create virtual devices for this interface
		devices, err := g.ifaceManager.CreateVirtualDevices(
			iface,
			ifaceConfig.VirtualDevices,
			g.config.NetworkSimulation.MACPrefix,
			g.config.NetworkSimulation.DHCPRetries,
			g.config.NetworkSimulation.DHCPRetryDelay,
		)
		if err != nil {
			log.Printf("Failed to create virtual devices for %s: %v", iface, err)
			continue
		}

		if len(devices) == 0 {
			log.Printf("No virtual devices created for interface %s", iface)
			continue
		}

		// Create crawlers for each virtual device
		var crawlers []*Crawler
		for i, device := range devices {
			crawler := NewCrawler(g.config, device, fmt.Sprintf("%s-%d", iface, i))
			crawlers = append(crawlers, crawler)
		}

		g.crawlers[iface] = crawlers

		// Start crawlers for this interface
		for _, crawler := range crawlers {
			g.wg.Add(1)
			go func(c *Crawler) {
				defer g.wg.Done()
				c.Run(ctx, g.stopChan)
			}(crawler)
		}

		log.Printf("Started %d crawlers for interface %s", len(crawlers), iface)
	}

	g.running = true
	log.Printf("Traffic generator started with %d interfaces", len(g.crawlers))
	return nil
}

// Stop stops traffic generation
func (g *Generator) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return fmt.Errorf("traffic generator is not running")
	}

	log.Println("Stopping traffic generator...")

	// Signal all crawlers to stop
	close(g.stopChan)

	// Wait for all crawlers to finish
	g.wg.Wait()

	// Clean up virtual devices
	g.ifaceManager.Cleanup()

	g.crawlers = make(map[string][]*Crawler)
	g.running = false

	log.Println("Traffic generator stopped")
	return nil
}

// IsRunning returns whether the generator is currently running
func (g *Generator) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

// GetStats returns current statistics
func (g *Generator) GetStats() *Stats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := &Stats{
		Running:    g.running,
		Interfaces: make(map[string]*InterfaceStats),
	}

	for ifaceName := range g.crawlers {
		ifaceStats, err := g.ifaceManager.GetInterfaceStats(ifaceName)
		if err != nil {
			log.Printf("Failed to get stats for %s: %v", ifaceName, err)
			continue
		}

		stats.Interfaces[ifaceName] = &InterfaceStats{
			InterfaceStats: *ifaceStats,
			VirtualDevices: len(g.ifaceManager.GetVirtualDevices(ifaceName)),
		}
	}

	return stats
}

// GetInterfaceManager returns the interface manager
func (g *Generator) GetInterfaceManager() *interfaces.Manager {
	return g.ifaceManager
}

// UpdateConfig updates the generator configuration
func (g *Generator) UpdateConfig(newConfig *config.Config) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config = newConfig
}

// Stats represents traffic generation statistics
type Stats struct {
	Running    bool                       `json:"running"`
	Interfaces map[string]*InterfaceStats `json:"interfaces"`
}

type InterfaceStats struct {
	interfaces.InterfaceStats
	VirtualDevices int `json:"virtual_devices"`
}
