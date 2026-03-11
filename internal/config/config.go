package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed original_user_agents.json
var embeddedUserAgents []byte

// Config represents the application configuration
type Config struct {
	RootURLs          []string         `json:"root_urls"`
	BlacklistedURLs   []string         `json:"blacklisted_urls"`
	UserAgents        []string         `json:"user_agents"`
	MaxDepth          int              `json:"max_depth"`
	MinSleep          int              `json:"min_sleep"`
	MaxSleep          int              `json:"max_sleep"`
	Timeout           int              `json:"timeout"`
	NetworkSimulation NetworkSimConfig `json:"network_simulation"`
}

// NetworkSimConfig represents network simulation configuration
type NetworkSimConfig struct {
	Enabled        bool                       `json:"enabled"`
	AutoDiscover   bool                       `json:"auto_discover"`
	Interfaces     map[string]InterfaceConfig `json:"interfaces"`
	VirtualDevices int                        `json:"virtual_devices"`
	MACPrefix      string                     `json:"mac_prefix"`
	IPConfig       string                     `json:"ip_config"`
	CleanupOnStart bool                       `json:"cleanup_on_start"`
	DHCPRetries    int                        `json:"dhcp_retries"`
	DHCPRetryDelay int                        `json:"dhcp_retry_delay"`
}

// InterfaceConfig represents per-interface configuration
type InterfaceConfig struct {
	Enabled        bool   `json:"enabled"`
	VirtualDevices int    `json:"virtual_devices"`
	IPConfig       string `json:"ip_config"`
}

// defaultConfig returns the embedded default configuration
func defaultConfig() *Config {
	// Load embedded user agents
	var userAgents []string
	if err := json.Unmarshal(embeddedUserAgents, &userAgents); err != nil {
		// Fallback to basic user agents if parsing fails
		userAgents = []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		}
	}

	return &Config{
		RootURLs: []string{
			"https://www.reddit.com",
			"https://www.yahoo.com",
			"https://www.cnn.com",
			"https://www.ebay.com",
			"https://wikipedia.org",
			"https://youtube.com",
			"https://github.com",
			"https://medium.com",
			"https://stackoverflow.com",
			"https://www.bbc.com",
			"https://www.nytimes.com",
			"https://www.theguardian.com",
			"https://www.reuters.com",
			"https://www.bloomberg.com",
			"https://www.amazon.com",
			"https://www.linkedin.com",
			"https://www.microsoft.com",
			"https://www.apple.com",
			"https://news.ycombinator.com",
			"https://www.techcrunch.com",
			"https://www.wired.com",
			"https://arstechnica.com",
			"https://www.theverge.com",
		},
		BlacklistedURLs: []string{
			"https://t.co",
			"t.umblr.com",
			"messenger.com",
			"itunes.apple.com",
			"l.facebook.com",
			"bit.ly",
			"mediawiki",
			".css",
			".ico",
			".xml",
			"intent/tweet",
			"twitter.com/share",
			"dialog/feed?",
			".json",
			"zendesk",
			"clickserve",
			".png",
			".iso",
		},
		UserAgents: userAgents,
		MaxDepth:   25,
		MinSleep:   3,
		MaxSleep:   6,
		Timeout:    30,
		NetworkSimulation: NetworkSimConfig{
			Enabled:        true,
			AutoDiscover:   true,
			Interfaces:     make(map[string]InterfaceConfig),
			VirtualDevices: 3,
			MACPrefix:      "02:00:00",
			IPConfig:       "dhcp",
			CleanupOnStart: true,
			DHCPRetries:    5,
			DHCPRetryDelay: 10,
		},
	}
}

// Load loads configuration from file or returns embedded defaults
func Load(configFile string) (*Config, error) {
	cfg := defaultConfig()

	// If no config file specified, use embedded defaults
	if configFile == "" {
		return cfg, nil
	}

	// Try to load external config file
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// File doesn't exist, use defaults and optionally create it
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.WriteFile(configFile, data, 0644); err != nil {
			// Don't fail if we can't write the file, just use defaults
			return cfg, nil
		}

		return cfg, nil
	}

	// Load existing config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save saves configuration to file
func (c *Config) Save(configFile string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
