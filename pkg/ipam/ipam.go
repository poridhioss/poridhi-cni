package ipam

import (
	"fmt"
	"net"
)

// Config holds IPAM configuration
type Config struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	DataDir string `json:"dataDir"`
}

// IPAM manages IP address allocation
type IPAM struct {
	config  *Config
	store   *Store
	subnet  *net.IPNet
	gateway net.IP
	firstIP net.IP
	lastIP  net.IP
}

// New creates a new IPAM instance
func New(config *Config, network string) (*IPAM, error) {
	subnet, firstIP, lastIP, err := ParseCIDR(config.Subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet: %w", err)
	}

	gateway := net.ParseIP(config.Gateway)
	if gateway == nil {
		// Default to first IP as gateway
		gateway = firstIP
		firstIP = NextIP(firstIP)
	}

	dataDir := config.DataDir
	if dataDir == "" {
		dataDir = "/var/lib/cni/networks"
	}

	return &IPAM{
		config:  config,
		store:   NewStore(dataDir, network),
		subnet:  subnet,
		gateway: gateway,
		firstIP: firstIP,
		lastIP:  lastIP,
	}, nil
}

// Allocate assigns an IP to a container
func (i *IPAM) Allocate(containerID string) (net.IP, error) {
	// Acquire lock
	lock, err := i.store.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer i.store.Unlock(lock)

	// Get last allocated IP
	lastIP, err := i.store.GetLastIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get last IP: %w", err)
	}

	// Start from first usable IP if no previous allocation
	startIP := i.firstIP
	if lastIP != nil {
		startIP = NextIP(lastIP)
	}

	// Find next available IP
	ip := startIP
	for {
		// Wrap around if we've gone past the end
		if !IPInRange(ip, i.subnet) || IPToUint32(ip) > IPToUint32(i.lastIP) {
			ip = i.firstIP
		}

		// Skip gateway
		if ip.Equal(i.gateway) {
			ip = NextIP(ip)
			continue
		}

		// Check if IP is available
		if !i.store.IsAllocated(ip) {
			break
		}

		ip = NextIP(ip)

		// Check if we've searched the entire range
		if ip.Equal(startIP) {
			return nil, fmt.Errorf("no available IPs in subnet %s", i.config.Subnet)
		}
	}

	// Reserve the IP
	if err := i.store.Reserve(ip, containerID); err != nil {
		return nil, fmt.Errorf("failed to reserve IP: %w", err)
	}

	// Update last allocated IP
	if err := i.store.SetLastIP(ip); err != nil {
		// Rollback reservation on failure
		i.store.Release(ip)
		return nil, fmt.Errorf("failed to update last IP: %w", err)
	}

	return ip, nil
}

// Release frees an IP allocation
func (i *IPAM) Release(containerID string, ip net.IP) error {
	lock, err := i.store.Lock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer i.store.Unlock(lock)

	// Verify the container owns this IP
	owner, err := i.store.GetContainerByIP(ip)
	if err != nil {
		return nil // IP not allocated, nothing to do
	}

	if owner != containerID {
		return fmt.Errorf("container %s does not own IP %s", containerID, ip)
	}

	return i.store.Release(ip)
}

// Gateway returns the gateway IP
func (i *IPAM) Gateway() net.IP {
	return i.gateway
}

// Subnet returns the subnet
func (i *IPAM) Subnet() *net.IPNet {
	return i.subnet
}