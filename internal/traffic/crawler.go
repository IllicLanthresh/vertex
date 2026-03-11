package traffic

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/IllicLanthresh/vertex/internal/config"
	"github.com/IllicLanthresh/vertex/internal/interfaces"
)

// Crawler generates HTTP traffic from a specific virtual device
type Crawler struct {
	config      *config.Config
	device      *interfaces.VirtualDevice
	id          string
	client      *http.Client
	visitedURLs map[string]bool
	links       []string
	rand        *rand.Rand
}

// NewCrawler creates a new crawler for a virtual device
func NewCrawler(cfg *config.Config, device *interfaces.VirtualDevice, id string) *Crawler {
	// Create HTTP client bound to the virtual device's IP
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			localAddr := &net.TCPAddr{
				IP: net.ParseIP(device.IP),
			}

			dialer := &net.Dialer{
				LocalAddr: localAddr,
				Timeout:   time.Duration(cfg.Timeout) * time.Second,
			}

			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		DisableKeepAlives:  false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}

	return &Crawler{
		config:      cfg,
		device:      device,
		id:          id,
		client:      client,
		visitedURLs: make(map[string]bool),
		links:       make([]string, 0),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run starts the crawler loop
func (c *Crawler) Run(ctx context.Context, stopChan <-chan struct{}) {
	log.Printf("Starting crawler %s on device %s (%s)", c.id, c.device.Name, c.device.IP)

	// Initialize with root URLs
	c.links = append(c.links, c.config.RootURLs...)

	ticker := time.NewTicker(c.getRandomSleepDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Crawler %s stopped by context", c.id)
			return
		case <-stopChan:
			log.Printf("Crawler %s stopped by stop channel", c.id)
			return
		case <-ticker.C:
			c.crawlNext()
			ticker.Reset(c.getRandomSleepDuration())
		}
	}
}

// crawlNext crawls the next URL in the queue
func (c *Crawler) crawlNext() {
	if len(c.links) == 0 {
		// Reset with root URLs if we run out of links
		c.links = append(c.links, c.config.RootURLs...)
		c.visitedURLs = make(map[string]bool) // Reset visited URLs
	}

	// Pick a random URL
	idx := c.rand.Intn(len(c.links))
	targetURL := c.links[idx]

	// Remove the URL from queue
	c.links = append(c.links[:idx], c.links[idx+1:]...)

	// Check if we've visited this URL recently
	if c.visitedURLs[targetURL] {
		return
	}

	// Mark as visited
	c.visitedURLs[targetURL] = true

	// Clean up visited URLs if too many (prevent memory leak)
	if len(c.visitedURLs) > 1000 {
		c.visitedURLs = make(map[string]bool)
	}

	// Crawl the URL
	c.crawlURL(targetURL, 0)
}

// crawlURL crawls a single URL and extracts links
func (c *Crawler) crawlURL(targetURL string, depth int) {
	if depth > c.config.MaxDepth {
		return
	}

	// Check if URL is blacklisted
	for _, blacklisted := range c.config.BlacklistedURLs {
		if strings.Contains(targetURL, blacklisted) {
			log.Printf("Skipping blacklisted URL: %s", targetURL)
			return
		}
	}

	log.Printf("Crawler %s: Fetching %s (depth: %d)", c.id, targetURL, depth)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Printf("Crawler %s: Failed to create request for %s: %v", c.id, targetURL, err)
		return
	}

	// Set random user agent
	if len(c.config.UserAgents) > 0 {
		userAgent := c.config.UserAgents[c.rand.Intn(len(c.config.UserAgents))]
		req.Header.Set("User-Agent", userAgent)
	}

	// Set common headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")

	start := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Crawler %s: Failed to fetch %s: %v", c.id, targetURL, err)
		return
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	log.Printf("Crawler %s: Fetched %s in %v (status: %d, size: %d bytes)",
		c.id, targetURL, duration, resp.StatusCode, resp.ContentLength)

	// Only process successful responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}

	const maxBodySize = 10 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		log.Printf("Crawler %s: Failed to read body from %s: %v", c.id, targetURL, err)
		return
	}

	// Extract links for further crawling
	newLinks := c.extractLinks(string(body), targetURL)

	// Add new links to queue (limited to prevent memory explosion)
	for i, link := range newLinks {
		if i >= 10 { // Limit to 10 new links per page
			break
		}
		if !c.visitedURLs[link] {
			c.links = append(c.links, link)
		}
	}

	log.Printf("Crawler %s: Extracted %d new links from %s", c.id, len(newLinks), targetURL)

	// Randomly crawl some of the extracted links immediately (depth-first)
	if depth < c.config.MaxDepth && len(newLinks) > 0 {
		// Crawl 1-2 random links immediately
		numToCrawl := 1 + c.rand.Intn(2)
		if numToCrawl > len(newLinks) {
			numToCrawl = len(newLinks)
		}

		for i := 0; i < numToCrawl; i++ {
			link := newLinks[c.rand.Intn(len(newLinks))]
			time.Sleep(time.Duration(c.rand.Intn(2)+1) * time.Second) // Brief delay
			c.crawlURL(link, depth+1)
		}
	}
}

// extractLinks extracts HTTP/HTTPS links from HTML content
func (c *Crawler) extractLinks(body, baseURL string) []string {
	var links []string

	// Parse base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return links
	}

	// Regular expressions for finding links
	linkRegexes := []*regexp.Regexp{
		regexp.MustCompile(`href=["']([^"']+)["']`),
		regexp.MustCompile(`src=["']([^"']+)["']`),
		regexp.MustCompile(`action=["']([^"']+)["']`),
	}

	seenLinks := make(map[string]bool)

	for _, regex := range linkRegexes {
		matches := regex.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			rawURL := match[1]

			// Skip non-HTTP links
			if strings.HasPrefix(rawURL, "javascript:") ||
				strings.HasPrefix(rawURL, "mailto:") ||
				strings.HasPrefix(rawURL, "tel:") ||
				strings.HasPrefix(rawURL, "#") {
				continue
			}

			// Parse and resolve URL
			parsed, err := url.Parse(rawURL)
			if err != nil {
				continue
			}

			resolvedURL := base.ResolveReference(parsed).String()

			// Only include HTTP/HTTPS URLs
			if !strings.HasPrefix(resolvedURL, "http://") &&
				!strings.HasPrefix(resolvedURL, "https://") {
				continue
			}

			// Avoid duplicates
			if seenLinks[resolvedURL] {
				continue
			}
			seenLinks[resolvedURL] = true

			links = append(links, resolvedURL)
		}
	}

	return links
}

// getRandomSleepDuration returns a random sleep duration within configured range
func (c *Crawler) getRandomSleepDuration() time.Duration {
	min := c.config.MinSleep
	max := c.config.MaxSleep

	if min >= max {
		return time.Duration(min) * time.Second
	}

	sleepSeconds := min + c.rand.Intn(max-min+1)
	return time.Duration(sleepSeconds) * time.Second
}
