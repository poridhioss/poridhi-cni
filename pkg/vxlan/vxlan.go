package vxlan

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

const (
	DefaultVXLANPort = 4789
	DefaultVNI       = 100
)

// Config holds VXLAN configuration
type Config struct {
	Name    string // Interface name (e.g., "vxlan0")
	VNI     int    // VXLAN Network Identifier
	Port    int    // UDP port (default 4789)
	LocalIP net.IP // Local VTEP IP (underlay)
	Dev     string // Physical device for underlay (e.g., "eth0")
	MTU     int    // MTU (usually 1450 for VXLAN overhead)
}

// Create creates a VXLAN interface
func Create(config *Config) (*netlink.Vxlan, error) {
	if config.Port == 0 {
		config.Port = DefaultVXLANPort
	}
	if config.VNI == 0 {
		config.VNI = DefaultVNI
	}
	if config.MTU == 0 {
		config.MTU = 1450 // Account for VXLAN overhead (50 bytes)
	}

	// Get the physical device index if specified
	var srcIfIndex int
	if config.Dev != "" {
		link, err := netlink.LinkByName(config.Dev)
		if err != nil {
			return nil, fmt.Errorf("failed to find device %s: %w", config.Dev, err)
		}
		srcIfIndex = link.Attrs().Index
	}

	// Create VXLAN interface
	vxlan := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name: config.Name,
			MTU:  config.MTU,
		},
		VxlanId:      config.VNI,
		Port:         config.Port,
		Learning:     false, // We'll manage FDB manually
		VtepDevIndex: srcIfIndex,
	}

	// Set local IP (VTEP IP)
	if config.LocalIP != nil {
		vxlan.SrcAddr = config.LocalIP
	}

	// Create the interface
	if err := netlink.LinkAdd(vxlan); err != nil {
		return nil, fmt.Errorf("failed to create vxlan: %w", err)
	}

	// Get the created interface
	link, err := netlink.LinkByName(config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get created vxlan: %w", err)
	}

	return link.(*netlink.Vxlan), nil
}

// Delete removes a VXLAN interface
func Delete(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil // Already deleted
	}

	return netlink.LinkDel(link)
}

// SetUp brings the VXLAN interface up
func SetUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(link)
}

// AddIP adds an IP address to the VXLAN interface
func AddIP(name string, ipNet *net.IPNet) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(link, addr)
}

// GetVXLAN retrieves VXLAN interface details
func GetVXLAN(name string) (*netlink.Vxlan, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	vxlan, ok := link.(*netlink.Vxlan)
	if !ok {
		return nil, fmt.Errorf("%s is not a vxlan interface", name)
	}

	return vxlan, nil
}

// Info returns human-readable VXLAN information
type Info struct {
	Name      string
	VNI       int
	Port      int
	LocalIP   string
	MTU       int
	State     string
	Addresses []string
}

func GetInfo(name string) (*Info, error) {
	vxlan, err := GetVXLAN(name)
	if err != nil {
		return nil, err
	}

	info := &Info{
		Name:  vxlan.Attrs().Name,
		VNI:   vxlan.VxlanId,
		Port:  vxlan.Port,
		MTU:   vxlan.Attrs().MTU,
		State: vxlan.Attrs().OperState.String(),
	}

	if vxlan.SrcAddr != nil {
		info.LocalIP = vxlan.SrcAddr.String()
	}

	// Get addresses
	addrs, _ := netlink.AddrList(vxlan, netlink.FAMILY_V4)
	for _, addr := range addrs {
		info.Addresses = append(info.Addresses, addr.IPNet.String())
	}

	return info, nil
}