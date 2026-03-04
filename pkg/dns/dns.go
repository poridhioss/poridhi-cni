package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	netnsDir       = "/etc/netns"
	resolvConfName = "resolv.conf"
	defaultDNS1    = "8.8.8.8"
	defaultDNS2    = "8.8.4.4"
)

// Config represents DNS configuration
type Config struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// DefaultConfig returns default DNS configuration
func DefaultConfig() *Config {
	return &Config{
		Nameservers: []string{defaultDNS1, defaultDNS2},
	}
}

// ConfigureForNamespace sets up DNS for a named namespace
func ConfigureForNamespace(nsName string, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	// Create namespace directory: /etc/netns/<nsName>/
	nsDir := filepath.Join(netnsDir, nsName)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("failed to create netns dir: %w", err)
	}

	// Generate resolv.conf content
	content := generateResolvConf(config)

	// Write resolv.conf: /etc/netns/<nsName>/resolv.conf
	resolvPath := filepath.Join(nsDir, resolvConfName)
	if err := os.WriteFile(resolvPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write resolv.conf: %w", err)
	}

	return nil
}

// generateResolvConf creates resolv.conf content from config
func generateResolvConf(config *Config) string {
	var lines []string

	for _, ns := range config.Nameservers {
		lines = append(lines, fmt.Sprintf("nameserver %s", ns))
	}

	if len(config.Search) > 0 {
		lines = append(lines, fmt.Sprintf("search %s", strings.Join(config.Search, " ")))
	}

	if len(config.Options) > 0 {
		lines = append(lines, fmt.Sprintf("options %s", strings.Join(config.Options, " ")))
	}

	return strings.Join(lines, "\n") + "\n"
}

// CleanupNamespace removes DNS configuration for a namespace
func CleanupNamespace(nsName string) error {
	nsDir := filepath.Join(netnsDir, nsName)
	return os.RemoveAll(nsDir)
}

// GetNamespaceConfig reads DNS configuration from a namespace
func GetNamespaceConfig(nsName string) (*Config, error) {
	resolvPath := filepath.Join(netnsDir, nsName, resolvConfName)

	data, err := os.ReadFile(resolvPath)
	if err != nil {
		return nil, err
	}

	return parseResolvConf(string(data))
}

// parseResolvConf parses resolv.conf content into a Config
func parseResolvConf(content string) (*Config, error) {
	config := &Config{}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "nameserver":
			config.Nameservers = append(config.Nameservers, parts[1])
		case "search":
			config.Search = append(config.Search, parts[1:]...)
		case "options":
			config.Options = append(config.Options, parts[1:]...)
		}
	}

	return config, nil
}


