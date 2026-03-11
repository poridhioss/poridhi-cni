package cluster

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// NodeConfig represents a single node in the cluster
type NodeConfig struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`      // Node IP (underlay)
	PodCIDR string `json:"podCIDR"` // Pod subnet for this node
}

// ClusterConfig represents the entire cluster configuration
type ClusterConfig struct {
	ClusterCIDR string       `json:"clusterCIDR"` // Overall cluster CIDR
	VNI         int          `json:"vni"`          // VXLAN Network ID
	Nodes       []NodeConfig `json:"nodes"`
}

// LoadConfig loads cluster configuration from a file
func LoadConfig(path string) (*ClusterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config ClusterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.VNI == 0 {
		config.VNI = 100
	}

	return &config, nil
}

// GetLocalNode identifies the current node by matching local IPs
func (c *ClusterConfig) GetLocalNode() (*NodeConfig, error) {
	localIPs, err := getLocalIPs()
	if err != nil {
		return nil, err
	}

	for _, node := range c.Nodes {
		nodeIP := net.ParseIP(node.IP)
		for _, localIP := range localIPs {
			if localIP.Equal(nodeIP) {
				return &node, nil
			}
		}
	}

	return nil, fmt.Errorf("current node not found in cluster config")
}

// GetRemoteNodes returns all nodes except the local one
func (c *ClusterConfig) GetRemoteNodes(localNode *NodeConfig) []NodeConfig {
	var remotes []NodeConfig
	for _, node := range c.Nodes {
		if node.Name != localNode.Name {
			remotes = append(remotes, node)
		}
	}
	return remotes
}

// getLocalIPs returns all IPv4 addresses on this machine
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP)
				}
			}
		}
	}

	return ips, nil
}

// Validate checks if the cluster configuration is valid
func (c *ClusterConfig) Validate() error {
	if len(c.Nodes) == 0 {
		return fmt.Errorf("no nodes configured")
	}

	seen := make(map[string]bool)
	for _, node := range c.Nodes {
		if node.Name == "" {
			return fmt.Errorf("node name is required")
		}
		if node.IP == "" {
			return fmt.Errorf("node %s: IP is required", node.Name)
		}
		if node.PodCIDR == "" {
			return fmt.Errorf("node %s: podCIDR is required", node.Name)
		}

		// Check for IP conflicts
		if seen[node.IP] {
			return fmt.Errorf("duplicate node IP: %s", node.IP)
		}
		seen[node.IP] = true

		// Check for CIDR conflicts
		if seen[node.PodCIDR] {
			return fmt.Errorf("duplicate podCIDR: %s", node.PodCIDR)
		}
		seen[node.PodCIDR] = true
	}

	return nil
}
