package cluster

import (
	"fmt"
	"net"

	"github.com/poridhioss/poridhi-cni/pkg/bridge"
	"github.com/poridhioss/poridhi-cni/pkg/route"
	"github.com/poridhioss/poridhi-cni/pkg/vxlan"
	"github.com/vishvananda/netlink"
)

const (
	DefaultBridgeName = "cni0"
	DefaultVXLANName  = "vxlan0"
)

// NetworkManager manages multi-node network setup
type NetworkManager struct {
	config    *ClusterConfig
	localNode *NodeConfig
	br        *bridge.Bridge
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(config *ClusterConfig) (*NetworkManager, error) {
	localNode, err := config.GetLocalNode()
	if err != nil {
		return nil, err
	}

	return &NetworkManager{
		config:    config,
		localNode: localNode,
	}, nil
}

// Setup configures the complete multi-node network
func (m *NetworkManager) Setup() error {
	// 1. Create bridge
	if err := m.setupBridge(); err != nil {
		return fmt.Errorf("failed to setup bridge: %w", err)
	}

	// 2. Create VXLAN
	if err := m.setupVXLAN(); err != nil {
		return fmt.Errorf("failed to setup vxlan: %w", err)
	}

	// 3. Attach VXLAN to bridge
	if err := m.attachVXLANToBridge(); err != nil {
		return fmt.Errorf("failed to attach vxlan to bridge: %w", err)
	}

	// 4. Setup FDB entries for remote nodes
	if err := m.setupFDB(); err != nil {
		return fmt.Errorf("failed to setup FDB: %w", err)
	}

	// 5. Setup routes to remote pod CIDRs
	if err := m.setupRoutes(); err != nil {
		return fmt.Errorf("failed to setup routes: %w", err)
	}

	// 6. Enable IP forwarding
	if err := route.EnableIPForwarding(); err != nil {
		return fmt.Errorf("failed to enable ip forwarding: %w", err)
	}

	return nil
}

// setupBridge creates the CNI bridge with gateway IP
func (m *NetworkManager) setupBridge() error {
	_, podNet, err := net.ParseCIDR(m.localNode.PodCIDR)
	if err != nil {
		return err
	}

	// Calculate gateway IP (first IP in subnet)
	ip4 := podNet.IP.To4()
	gatewayIP := make(net.IP, len(ip4))
	copy(gatewayIP, ip4)
	gatewayIP[3] = 1 // .1 for gateway

	// Create bridge (idempotent - returns existing if already created)
	br, err := bridge.Create(DefaultBridgeName)
	if err != nil {
		return err
	}
	m.br = br

	// Add gateway IP using the bridge's SetIP method
	gatewayCIDR := fmt.Sprintf("%s/%d", gatewayIP.String(), maskBits(podNet.Mask))
	if err := br.SetIP(gatewayCIDR); err != nil {
		return err
	}

	return br.SetUp()
}

// maskBits returns the number of leading 1-bits in a subnet mask
func maskBits(mask net.IPMask) int {
	bits, _ := mask.Size()
	return bits
}

// setupVXLAN creates the VXLAN interface
func (m *NetworkManager) setupVXLAN() error {
	localIP := net.ParseIP(m.localNode.IP)

	config := &vxlan.Config{
		Name:    DefaultVXLANName,
		VNI:     m.config.VNI,
		LocalIP: localIP,
		MTU:     1450,
	}

	_, err := vxlan.Create(config)
	if err != nil {
		// Check if already exists
		if _, getErr := vxlan.GetVXLAN(DefaultVXLANName); getErr == nil {
			return nil // Already exists
		}
		return err
	}

	return vxlan.SetUp(DefaultVXLANName)
}

// attachVXLANToBridge attaches VXLAN interface to the bridge
func (m *NetworkManager) attachVXLANToBridge() error {
	return m.br.AttachInterface(DefaultVXLANName)
}

// setupFDB adds FDB entries for all remote nodes
func (m *NetworkManager) setupFDB() error {
	remoteNodes := m.config.GetRemoteNodes(m.localNode)

	for _, node := range remoteNodes {
		remoteIP := net.ParseIP(node.IP)

		// Add broadcast entry for BUM traffic
		if err := vxlan.AddBroadcastEntry(DefaultVXLANName, remoteIP); err != nil {
			// Ignore duplicate errors
			continue
		}
	}

	return nil
}

// setupRoutes adds routes to remote pod CIDRs
func (m *NetworkManager) setupRoutes() error {
	remoteNodes := m.config.GetRemoteNodes(m.localNode)

	for _, node := range remoteNodes {
		_, podNet, err := net.ParseCIDR(node.PodCIDR)
		if err != nil {
			continue
		}

		rt := &netlink.Route{
			Dst:       podNet,
			LinkIndex: m.br.Link.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
		}

		if err := netlink.RouteAdd(rt); err != nil {
			// Ignore if route exists
			continue
		}
	}

	return nil
}

// Teardown removes multi-node network configuration
func (m *NetworkManager) Teardown() error {
	vxlan.Delete(DefaultVXLANName)
	if m.br != nil {
		m.br.Delete()
	}
	return nil
}

// GetLocalNode returns the local node configuration
func (m *NetworkManager) GetLocalNode() *NodeConfig {
	return m.localNode
}

// GetPodCIDR returns the pod CIDR for the local node
func (m *NetworkManager) GetPodCIDR() string {
	return m.localNode.PodCIDR
}
