package bridge

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// Bridge represents a Linux bridge interface
type Bridge struct {
	Name string
	Link netlink.Link
}

// Create creates a new bridge or returns existing one
func Create(name string) (*Bridge, error) {
	// Check if bridge already exists
	existing, err := netlink.LinkByName(name)
	if err == nil {
		// Bridge exists, return it
		return &Bridge{Name: name, Link: existing}, nil
	}

	// Create new bridge
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		return nil, fmt.Errorf("failed to create bridge %s: %w", name, err)
	}

	// Get the created bridge
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get created bridge: %w", err)
	}

	return &Bridge{Name: name, Link: link}, nil
}

// EnsureExists creates bridge if it doesn't exist, returns existing otherwise
func EnsureExists(name string) (*Bridge, error) {
	return Create(name) // Create is already idempotent
}

// SetIP assigns an IP address to the bridge (gateway IP)
func (b *Bridge) SetIP(cidr string) error {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Check if IP is already assigned
	addrs, err := netlink.AddrList(b.Link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			return nil // Already has this IP, nothing to do
		}
	}

	// Add the IP address
	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: ipNet.Mask,
		},
	}

	if err := netlink.AddrAdd(b.Link, addr); err != nil {
		return fmt.Errorf("failed to add IP to bridge: %w", err)
	}

	return nil
}


// SetUp brings the bridge interface up
func (b *Bridge) SetUp() error {
	if err := netlink.LinkSetUp(b.Link); err != nil {
		return fmt.Errorf("failed to bring bridge up: %w", err)
	}
	return nil
}

// AttachInterface attaches a network interface to the bridge
func (b *Bridge) AttachInterface(ifName string) error {
	// Get the interface to attach
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifName, err)
	}

	// Check if already attached to this bridge
	if link.Attrs().MasterIndex == b.Link.Attrs().Index {
		return nil // Already attached
	}

	// Attach to bridge
	if err := netlink.LinkSetMaster(link, b.Link); err != nil {
		return fmt.Errorf("failed to attach %s to bridge: %w", ifName, err)
	}

	return nil
}

// DetachInterface removes an interface from the bridge
func (b *Bridge) DetachInterface(ifName string) error {
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return nil // Interface doesn't exist, nothing to detach
	}

	if err := netlink.LinkSetNoMaster(link); err != nil {
		return fmt.Errorf("failed to detach %s from bridge: %w", ifName, err)
	}

	return nil
}

// Delete removes the bridge interface
func (b *Bridge) Delete() error {
	if err := netlink.LinkDel(b.Link); err != nil {
		return fmt.Errorf("failed to delete bridge: %w", err)
	}
	return nil
}

// GetAttachedInterfaces returns names of all interfaces attached to this bridge
func (b *Bridge) GetAttachedInterfaces() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}

	var attached []string
	bridgeIndex := b.Link.Attrs().Index

	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attached = append(attached, link.Attrs().Name)
		}
	}

	return attached, nil
}


// FDBEntry represents a forwarding database entry
type FDBEntry struct {
	MAC      string
	PortName string
	PortIdx  int
}

// GetFDB returns the bridge's forwarding database entries
// Note: For detailed FDB inspection, use: bridge fdb show dev <bridge-name>
func (b *Bridge) GetFDB() ([]FDBEntry, error) {
	attached, err := b.GetAttachedInterfaces()
	if err != nil {
		return nil, err
	}

	var entries []FDBEntry
	for _, ifName := range attached {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			continue
		}
		// Add the interface's MAC as an FDB entry
		if link.Attrs().HardwareAddr != nil {
			entries = append(entries, FDBEntry{
				MAC:      link.Attrs().HardwareAddr.String(),
				PortName: ifName,
				PortIdx:  link.Attrs().Index,
			})
		}
	}

	return entries, nil
}

// GetInfo returns detailed information about the bridge
func (b *Bridge) GetInfo() (map[string]interface{}, error) {
	addrs, _ := netlink.AddrList(b.Link, netlink.FAMILY_V4)
	attached, _ := b.GetAttachedInterfaces()

	var addrStrings []string
	for _, addr := range addrs {
		addrStrings = append(addrStrings, addr.IPNet.String())
	}

	return map[string]interface{}{
		"name":      b.Name,
		"mac":       b.Link.Attrs().HardwareAddr.String(),
		"mtu":       b.Link.Attrs().MTU,
		"state":     b.Link.Attrs().OperState.String(),
		"addresses": addrStrings,
		"attached":  attached,
	}, nil
}

// Exists checks if a bridge with the given name exists
func Exists(name string) bool {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false
	}
	return link.Type() == "bridge"
}

