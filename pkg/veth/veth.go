package veth

import (
	"crypto/rand"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// VethPair represents a virtual ethernet pair
type VethPair struct {
	HostEnd       netlink.Link
	ContainerEnd  netlink.Link
	HostName      string
	ContainerName string
}

// Create creates a new veth pair with specified names
func Create(hostName, containerName string, mtu int) (*VethPair, error) {
	// Define the veth pair
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostName,
			MTU:  mtu,
		},
		PeerName: containerName,
	}

	// Create the veth pair
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Get references to both ends
	hostLink, err := netlink.LinkByName(hostName)
	if err != nil {
		netlink.LinkDel(veth)
		return nil, fmt.Errorf("failed to get host veth: %w", err)
	}

	containerLink, err := netlink.LinkByName(containerName)
	if err != nil {
		netlink.LinkDel(veth)
		return nil, fmt.Errorf("failed to get container veth: %w", err)
	}

	return &VethPair{
		HostEnd:       hostLink,
		ContainerEnd:  containerLink,
		HostName:      hostName,
		ContainerName: containerName,
	}, nil
}

// CreateWithRandomNames creates a veth pair with generated names
func CreateWithRandomNames(containerID string, mtu int) (*VethPair, error) {
	// Generate host-side name: veth + first 8 chars of container ID
	hostName := fmt.Sprintf("veth%.8s", containerID)

	// Container side will be renamed to eth0 later, use temp name
	containerName := fmt.Sprintf("eth%.8s", containerID)

	// Ensure name doesn't exceed 15 characters (Linux limit)
	if len(hostName) > 15 {
		hostName = hostName[:15]
	}
	if len(containerName) > 15 {
		containerName = containerName[:15]
	}

	return Create(hostName, containerName, mtu)
}

// MoveToNetNS moves the container end to a different network namespace
func (v *VethPair) MoveToNetNS(nsPath string) error {
	// Open the target namespace
	nsFd, err := unix.Open(nsPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to open namespace %s: %w", nsPath, err)
	}
	defer unix.Close(nsFd)

	// Move the container end to the target namespace
	if err := netlink.LinkSetNsFd(v.ContainerEnd, nsFd); err != nil {
		return fmt.Errorf("failed to move veth to namespace: %w", err)
	}

	return nil
}

// SetUp brings an interface up
func SetUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to set link up: %w", err)
	}

	return nil
}

// SetDown brings an interface down
func SetDown(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to set link down: %w", err)
	}

	return nil
}

// SetMAC sets the MAC address of an interface
func SetMAC(name string, mac net.HardwareAddr) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err := netlink.LinkSetHardwareAddr(link, mac); err != nil {
		return fmt.Errorf("failed to set MAC address: %w", err)
	}

	return nil
}

// GenerateMAC generates a random locally-administered MAC address
func GenerateMAC() (net.HardwareAddr, error) {
	mac := make([]byte, 6)
	if _, err := rand.Read(mac); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Set the locally-administered bit and clear multicast bit
	// This ensures the MAC is valid for a local interface
	mac[0] = (mac[0] | 0x02) & 0xfe

	return net.HardwareAddr(mac), nil
}

// SetMTU sets the MTU of an interface
func SetMTU(name string, mtu int) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	return nil
}

// AddIP adds an IP address to an interface
func AddIP(name string, ipNet *net.IPNet) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", name, err)
	}

	addr := &netlink.Addr{IPNet: ipNet}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %w", err)
	}

	return nil
}

// Rename changes the name of an interface
func Rename(oldName, newName string) error {
	link, err := netlink.LinkByName(oldName)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", oldName, err)
	}

	// Interface must be down to rename
	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to set link down: %w", err)
	}

	if err := netlink.LinkSetName(link, newName); err != nil {
		return fmt.Errorf("failed to rename link: %w", err)
	}

	return nil
}

// Delete removes a veth pair (deleting one end removes both)
func (v *VethPair) Delete() error {
	if err := netlink.LinkDel(v.HostEnd); err != nil {
		return fmt.Errorf("failed to delete veth pair: %w", err)
	}
	return nil
}

// DeleteByName deletes a veth interface by name
func DeleteByName(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		// Interface doesn't exist, nothing to delete
		return nil
	}

	return netlink.LinkDel(link)
}

// GetLinkInfo returns information about a network interface
func GetLinkInfo(name string) (map[string]interface{}, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get link %s: %w", name, err)
	}

	attrs := link.Attrs()
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_ALL)

	var addrStrings []string
	for _, addr := range addrs {
		addrStrings = append(addrStrings, addr.IPNet.String())
	}

	return map[string]interface{}{
		"name":  attrs.Name,
		"mac":   attrs.HardwareAddr.String(),
		"mtu":   attrs.MTU,
		"state": attrs.OperState.String(),
		"index": attrs.Index,
		"addrs": addrStrings,
	}, nil
}