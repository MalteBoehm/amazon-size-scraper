package proxy

import (
	"bufio"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
)

// ProxyConfig represents a single proxy configuration
type ProxyConfig struct {
	Username string
	Password string
	Host     string
	Port     string
	URL      string // Full proxy URL
}

// ProxyManager manages proxy rotation and health checking
type ProxyManager struct {
	proxies     []ProxyConfig
	currentIdx  int
	mu          sync.RWMutex
	logger      *slog.Logger
	failedCount map[int]int // Track failures per proxy
	maxFailures int         // Max failures before blacklisting
}

// NewProxyManager creates a new proxy manager from a file
func NewProxyManager(filename string, logger *slog.Logger) (*ProxyManager, error) {
	proxies, err := loadProxiesFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load proxies: %w", err)
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no valid proxies found in file %s", filename)
	}

	// Shuffle proxies for better distribution
	rand.Shuffle(len(proxies), func(i, j int) {
		proxies[i], proxies[j] = proxies[j], proxies[i]
	})

	return &ProxyManager{
		proxies:     proxies,
		currentIdx:  0,
		logger:      logger.With("component", "proxy-manager"),
		failedCount: make(map[int]int),
		maxFailures: 3, // Blacklist after 3 failures
	}, nil
}

// GetNextProxy returns the next proxy in rotation
func (pm *ProxyManager) GetNextProxy() (*ProxyConfig, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// Find next healthy proxy
	startIdx := pm.currentIdx
	for {
		proxy := &pm.proxies[pm.currentIdx]
		
		// Check if proxy is not blacklisted
		if pm.failedCount[pm.currentIdx] < pm.maxFailures {
			pm.currentIdx = (pm.currentIdx + 1) % len(pm.proxies)
			pm.logger.Debug("selected proxy", "index", pm.currentIdx-1, "host", proxy.Host)
			return proxy, nil
		}

		pm.currentIdx = (pm.currentIdx + 1) % len(pm.proxies)
		
		// If we've cycled through all proxies and all are blacklisted
		if pm.currentIdx == startIdx {
			// Reset failure counts to give proxies another chance
			pm.logger.Warn("all proxies blacklisted, resetting failure counts")
			pm.failedCount = make(map[int]int)
			proxy := &pm.proxies[pm.currentIdx]
			pm.currentIdx = (pm.currentIdx + 1) % len(pm.proxies)
			return proxy, nil
		}
	}
}

// GetRandomProxy returns a random proxy (useful for initial distribution)
func (pm *ProxyManager) GetRandomProxy() (*ProxyConfig, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.proxies) == 0 {
		return nil, fmt.Errorf("no proxies available")
	}

	// Get healthy proxies
	var healthyProxies []int
	for i := range pm.proxies {
		if pm.failedCount[i] < pm.maxFailures {
			healthyProxies = append(healthyProxies, i)
		}
	}

	if len(healthyProxies) == 0 {
		// All proxies are blacklisted, reset and use any
		pm.logger.Warn("all proxies blacklisted, using random proxy anyway")
		idx := rand.Intn(len(pm.proxies))
		return &pm.proxies[idx], nil
	}

	idx := healthyProxies[rand.Intn(len(healthyProxies))]
	return &pm.proxies[idx], nil
}

// MarkProxyFailed marks a proxy as failed
func (pm *ProxyManager) MarkProxyFailed(proxy *ProxyConfig) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Find proxy index
	for i, p := range pm.proxies {
		if p.URL == proxy.URL {
			pm.failedCount[i]++
			pm.logger.Warn("proxy marked as failed",
				"host", proxy.Host,
				"failures", pm.failedCount[i],
				"blacklisted", pm.failedCount[i] >= pm.maxFailures)
			break
		}
	}
}

// GetProxyCount returns total number of proxies
func (pm *ProxyManager) GetProxyCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.proxies)
}

// GetHealthyProxyCount returns number of healthy (non-blacklisted) proxies
func (pm *ProxyManager) GetHealthyProxyCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	healthy := 0
	for i := range pm.proxies {
		if pm.failedCount[i] < pm.maxFailures {
			healthy++
		}
	}
	return healthy
}

// loadProxiesFromFile loads proxies from file in Packetstream format
// Format: letsgrowdigital:2dc91789a1ca3824_country-Germany_session-X:proxy.packetstream.io:31111
func loadProxiesFromFile(filename string) ([]ProxyConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open proxy file: %w", err)
	}
	defer file.Close()

	var proxies []ProxyConfig
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		proxy, err := parseProxyLine(line)
		if err != nil {
			// Log warning but continue processing
			slog.Warn("invalid proxy line", "line", lineNum, "content", line, "error", err)
			continue
		}

		proxies = append(proxies, proxy)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading proxy file: %w", err)
	}

	return proxies, nil
}

// parseProxyLine parses a single proxy line in Packetstream format
func parseProxyLine(line string) (ProxyConfig, error) {
	// Expected format: username:password@host:port or username:password:host:port
	var username, password, host, port string

	// Try format with @ separator first
	if strings.Contains(line, "@") {
		parts := strings.Split(line, "@")
		if len(parts) != 2 {
			return ProxyConfig{}, fmt.Errorf("invalid format: expected username:password@host:port")
		}

		// Parse credentials
		credParts := strings.Split(parts[0], ":")
		if len(credParts) != 2 {
			return ProxyConfig{}, fmt.Errorf("invalid credentials format")
		}
		username, password = credParts[0], credParts[1]

		// Parse host:port
		hostParts := strings.Split(parts[1], ":")
		if len(hostParts) != 2 {
			return ProxyConfig{}, fmt.Errorf("invalid host:port format")
		}
		host, port = hostParts[0], hostParts[1]
	} else {
		// Try colon-separated format: username:password:host:port
		parts := strings.Split(line, ":")
		if len(parts) != 4 {
			return ProxyConfig{}, fmt.Errorf("invalid format: expected username:password:host:port or username:password@host:port")
		}
		username, password, host, port = parts[0], parts[1], parts[2], parts[3]
	}

	// Validate required fields
	if username == "" || password == "" || host == "" || port == "" {
		return ProxyConfig{}, fmt.Errorf("missing required fields")
	}

	return ProxyConfig{
		Username: username,
		Password: password,
		Host:     host,
		Port:     port,
		URL:      fmt.Sprintf("http://%s:%s@%s:%s", username, password, host, port),
	}, nil
}