package cni

import (
	"fmt"
	"net"

	"github.com/poridhioss/poridhi-cni/pkg/bridge"
	"github.com/poridhioss/poridhi-cni/pkg/ipam"
	"github.com/poridhioss/poridhi-cni/pkg/netns"
	"github.com/poridhioss/poridhi-cni/pkg/iptables"
	"github.com/poridhioss/poridhi-cni/pkg/veth"
	"github.com/poridhioss/poridhi-cni/pkg/route"
	"github.com/vishvananda/netlink"
)

// SetupNetwork performs complete network setup for a container
// This is the main CNI ADD implementation
func SetupNetwork(args *EnvArgs, conf *NetConf) (*Result, error) {
	var allocatedIP net.IP
	var vethPair *veth.VethPair
	var ipamManager *ipam.IPAM
	var err error

	// Deferred cleanup on failure
	defer func() {
		if err != nil {
			// Clean up veth if created
			if vethPair != nil {
				veth.DeleteByName(vethPair.HostName)
			}
			// Release IP if allocated
			if allocatedIP != nil && ipamManager != nil {
				ipamManager.Release(args.ContainerID, allocatedIP)
			}
		}
	}()

	// Step 0: Enable IP forwarding (required for routing packets from containers)
        if err := route.EnableIPForwarding(); err != nil {
            return nil, fmt.Errorf("failed to enable IP forwarding: %w", err)
        }

	// Step 1: Initialize IPAM and allocate IP
	ipamConfig := &ipam.Config{
		Subnet:  conf.IPAM.Subnet,
		Gateway: conf.IPAM.Gateway,
	}
	ipamManager, err = ipam.New(ipamConfig, conf.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IPAM: %w", err)
	}

	allocatedIP, err = ipamManager.Allocate(args.ContainerID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Step 2: Ensure bridge exists
	br, err := bridge.EnsureExists(conf.Bridge)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure bridge exists: %w", err)
	}

	// Step 3: Set bridge IP (gateway) - idempotent
	gatewayIP := ipamManager.Gateway()
	ones, _ := ipamManager.Subnet().Mask.Size()
	bridgeCIDR := fmt.Sprintf("%s/%d", gatewayIP.String(), ones)
	if err = br.SetIP(bridgeCIDR); err != nil {
		return nil, fmt.Errorf("failed to set bridge IP: %w", err)
	}

	// Step 4: Bring bridge up
	if err = br.SetUp(); err != nil {
		return nil, fmt.Errorf("failed to bring bridge up: %w", err)
	}

	// Step 5: Create veth pair
	mtu := 1500
	if conf.MTU > 0 {
		mtu = conf.MTU
	}
	vethPair, err = veth.CreateWithRandomNames(args.ContainerID, mtu)
	if err != nil {
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Step 6: Attach host end to bridge
	if err = br.AttachInterface(vethPair.HostName); err != nil {
		return nil, fmt.Errorf("failed to attach veth to bridge: %w", err)
	}

	// Step 7: Bring host end up
	if err = veth.SetUp(vethPair.HostName); err != nil {
		return nil, fmt.Errorf("failed to bring host veth up: %w", err)
	}

	// Step 8: Move container end to namespace
	if err = vethPair.MoveToNetNS(args.NetNS); err != nil {
		return nil, fmt.Errorf("failed to move veth to container namespace: %w", err)
	}

	// Step 9: Configure container namespace
	var containerMAC string
	err = netns.WithNetNS(args.NetNS, func() error {
		return configureContainerNetworking(
			vethPair.ContainerName,
			args.IfName,
			allocatedIP,
			ipamManager.Subnet().Mask,
			gatewayIP,
			&containerMAC,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to configure container networking: %w", err)
	}

	// Setup NAT for internet access
	iptMgr, err := iptables.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to init iptables: %w", err)
	}

	if err := iptMgr.SetupNAT(conf.IPAM.Subnet, conf.Bridge); err != nil {
		return nil, fmt.Errorf("failed to setup NAT: %w", err)
	}

	// Setup FORWARD rules for bridge traffic
	if err := iptMgr.SetupForward(conf.IPAM.Subnet, conf.Bridge); err != nil {
		return nil, fmt.Errorf("failed to setup FORWARD rules: %w", err)
	}

	// Build and return result
	result := &Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []Interface{
			{
				Name:    args.IfName,
				Mac:     containerMAC,
				Sandbox: args.NetNS,
			},
		},
		IPs: []IPConfig{
			{
				Address:   fmt.Sprintf("%s/%d", allocatedIP.String(), ones),
				Gateway:   gatewayIP.String(),
				Interface: 0,
			},
		},
		Routes: []Route{
			{
				Dst: "0.0.0.0/0",
				GW:  gatewayIP.String(),
			},
		},
	}

	// Clear err so deferred cleanup doesn't run
	err = nil
	return result, nil
}

// configureContainerNetworking sets up networking inside the container namespace
// This function runs INSIDE the container's network namespace
func configureContainerNetworking(
	currentIfName string,
	targetIfName string,
	ip net.IP,
	mask net.IPMask,
	gateway net.IP,
	macOut *string,
) error {
	// Get the interface (it was moved here from host)
	link, err := netlink.LinkByName(currentIfName)
	if err != nil {
		return fmt.Errorf("failed to find interface %s: %w", currentIfName, err)
	}

	// Rename to target name (usually eth0) if different
	if currentIfName != targetIfName {
		// Must be down to rename
		if err := netlink.LinkSetDown(link); err != nil {
			return fmt.Errorf("failed to bring interface down for rename: %w", err)
		}

		if err := netlink.LinkSetName(link, targetIfName); err != nil {
			return fmt.Errorf("failed to rename interface to %s: %w", targetIfName, err)
		}

		// Get the renamed link
		link, err = netlink.LinkByName(targetIfName)
		if err != nil {
			return fmt.Errorf("failed to find renamed interface: %w", err)
		}
	}

	// Bring interface up
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Add IP address
	ipNet := &net.IPNet{
		IP:   ip,
		Mask: mask,
	}
	addr := &netlink.Addr{IPNet: ipNet}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %w", err)
	}

	// Bring loopback up
	lo, err := netlink.LinkByName("lo")
	if err == nil {
		if err := netlink.LinkSetUp(lo); err != nil {
			return fmt.Errorf("failed to bring loopback up: %w", err)
		}
	}

	// Add default route via gateway
	defaultRoute := &netlink.Route{
		Gw: gateway,
	}
	if err := netlink.RouteAdd(defaultRoute); err != nil {
		return fmt.Errorf("failed to add default route: %w", err)
	}

	// Return MAC address for the result
	*macOut = link.Attrs().HardwareAddr.String()

	return nil
}

// TeardownNetwork removes network configuration for a container
// This is the CNI DEL implementation
func TeardownNetwork(args *EnvArgs, conf *NetConf) error {
	// Generate the expected veth name
	hostVethName := fmt.Sprintf("veth%.8s", args.ContainerID)

	// Delete the veth pair (deleting host end removes both)
	// This is idempotent - succeeds even if veth doesn't exist
	if err := veth.DeleteByName(hostVethName); err != nil {
		// Log but don't fail - DEL should be idempotent
		fmt.Printf("Warning: failed to delete veth %s: %v\n", hostVethName, err)
	}

	// Release IP from IPAM
	// Note: We need to know which IP was allocated to this container
	// In a production implementation, we'd either:
	// 1. Store the IP in prevResult (CNI 1.0+ feature)
	// 2. Look it up from IPAM store by container ID
	// For now, we scan the IPAM store to find and release the IP

	if conf.IPAM.Subnet != "" {
		ipamConfig := &ipam.Config{
			Subnet:  conf.IPAM.Subnet,
			Gateway: conf.IPAM.Gateway,
		}
		ipamManager, err := ipam.New(ipamConfig, conf.Name)
		if err == nil {
			// Try to find and release IP for this container
			releaseIPForContainer(ipamManager, args.ContainerID, conf.Name)
		}
	}

	return nil
}

// releaseIPForContainer finds and releases the IP allocated to a container
func releaseIPForContainer(ipamManager *ipam.IPAM, containerID, network string) {
	// The IPAM store files are named by IP and contain the container ID
	// We need to scan them to find which IP belongs to this container
	store := ipam.NewStore("/var/lib/cni/networks", network)

	// Get the subnet range to scan
	subnet := ipamManager.Subnet()
	gateway := ipamManager.Gateway()

	// Iterate through possible IPs
	ip := ipam.NextIP(subnet.IP.Mask(subnet.Mask))
	lastIP := ipam.LastIP(subnet)

	for ; ipam.IPToUint32(ip) <= ipam.IPToUint32(lastIP); ip = ipam.NextIP(ip) {
		// Skip gateway
		if ip.Equal(gateway) {
			continue
		}

		// Check if this IP is allocated to our container
		owner, err := store.GetContainerByIP(ip)
		if err != nil {
			continue
		}

		if owner == containerID {
			store.Release(ip)
			return
		}
	}
}

// CheckNetwork verifies the container's network configuration
// This is the CNI CHECK implementation
func CheckNetwork(args *EnvArgs, conf *NetConf) error {
	// Verify the veth exists on host side
	hostVethName := fmt.Sprintf("veth%.8s", args.ContainerID)
	_, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return fmt.Errorf("host veth %s not found: %w", hostVethName, err)
	}

	// Verify the interface exists in container namespace
	err = netns.WithNetNS(args.NetNS, func() error {
		link, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return fmt.Errorf("container interface %s not found: %w", args.IfName, err)
		}

		// Check interface is UP
		if link.Attrs().OperState != netlink.OperUp {
			return fmt.Errorf("interface %s is not UP", args.IfName)
		}

		// Check interface has an IP
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil || len(addrs) == 0 {
			return fmt.Errorf("interface %s has no IPv4 address", args.IfName)
		}

		return nil
	})

	return err
}

