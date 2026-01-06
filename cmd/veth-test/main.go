package main

import (
	"fmt"
	"net"
	"os"

	"github.com/vishvananda/netlink"
	"github.com/poridhioss/poridhi-cni/pkg/netns"
	"github.com/poridhioss/poridhi-cni/pkg/veth"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		if len(os.Args) < 4 {
			fmt.Println("Usage: veth-test create <host-name> <peer-name>")
			os.Exit(1)
		}
		createVeth(os.Args[2], os.Args[3])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: veth-test delete <name>")
			os.Exit(1)
		}
		deleteVeth(os.Args[2])

	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: veth-test info <name>")
			os.Exit(1)
		}
		showInfo(os.Args[2])

	case "move":
		if len(os.Args) < 4 {
			fmt.Println("Usage: veth-test move <veth-name> <namespace-name>")
			os.Exit(1)
		}
		moveToNS(os.Args[2], os.Args[3])

	case "demo":
		runDemo()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: veth-test <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  create <host-name> <peer-name>  - Create a veth pair")
	fmt.Println("  delete <name>                   - Delete a veth (and its peer)")
	fmt.Println("  info <name>                     - Show interface info")
	fmt.Println("  move <name> <namespace>         - Move veth to namespace")
	fmt.Println("  demo                            - Run full demo with connectivity test")
}

func createVeth(hostName, peerName string) {
	pair, err := veth.Create(hostName, peerName, 1500)
	if err != nil {
		fmt.Printf("Failed to create veth pair: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created veth pair:\n")
	fmt.Printf("  Host end:      %s (index: %d)\n", pair.HostName, pair.HostEnd.Attrs().Index)
	fmt.Printf("  Container end: %s (index: %d)\n", pair.ContainerName, pair.ContainerEnd.Attrs().Index)
}

func deleteVeth(name string) {
	if err := veth.DeleteByName(name); err != nil {
		fmt.Printf("Failed to delete veth: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted veth: %s (peer also deleted)\n", name)
}

func showInfo(name string) {
	info, err := veth.GetLinkInfo(name)
	if err != nil {
		fmt.Printf("Failed to get info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Interface: %s\n", info["name"])
	fmt.Printf("  MAC:   %s\n", info["mac"])
	fmt.Printf("  MTU:   %d\n", info["mtu"])
	fmt.Printf("  State: %s\n", info["state"])
	fmt.Printf("  Index: %d\n", info["index"])
	fmt.Printf("  Addrs: %v\n", info["addrs"])
}

func moveToNS(vethName, nsName string) {
	nsPath := fmt.Sprintf("/var/run/netns/%s", nsName)

	// Get the link first
	pair := &veth.VethPair{}
	link, err := netlink.LinkByName(vethName)
	if err != nil {
		fmt.Printf("Failed to get veth: %v\n", err)
		os.Exit(1)
	}
	pair.ContainerEnd = link
	pair.ContainerName = vethName

	if err := pair.MoveToNetNS(nsPath); err != nil {
		fmt.Printf("Failed to move veth: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Moved %s to namespace %s\n", vethName, nsName)
}

func runDemo() {
	fmt.Println("=== Veth Pair Demo ===")
	fmt.Println()

	// Step 1: Create namespace
	fmt.Println("1. Creating test namespace...")
	ns, err := netns.CreateNamed("veth-demo")
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		netns.Delete("veth-demo")
		fmt.Println("8. Cleaned up namespace")
	}()
	fmt.Printf("   Created: %s\n", ns.Path())

	// Step 2: Create veth pair
	fmt.Println("\n2. Creating veth pair...")
	pair, err := veth.Create("veth-host", "veth-cont", 1500)
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		veth.DeleteByName("veth-host")
		fmt.Println("7. Cleaned up veth pair")
	}()
	fmt.Printf("   Created: %s <--> %s\n", pair.HostName, pair.ContainerName)

	// Step 3: Configure host end
	fmt.Println("\n3. Configuring host end...")
	hostIP := &net.IPNet{
		IP:   net.ParseIP("10.0.0.1"),
		Mask: net.CIDRMask(24, 32),
	}
	if err := veth.AddIP("veth-host", hostIP); err != nil {
		fmt.Printf("   Failed to add IP: %v\n", err)
		os.Exit(1)
	}
	if err := veth.SetUp("veth-host"); err != nil {
		fmt.Printf("   Failed to bring up: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   IP: %s, State: UP\n", hostIP.String())

	// Step 4: Move container end to namespace
	fmt.Println("\n4. Moving container end to namespace...")
	if err := pair.MoveToNetNS(ns.Path()); err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   Moved veth-cont to veth-demo namespace")

	// Step 5: Configure container end (inside namespace)
	fmt.Println("\n5. Configuring container end (inside namespace)...")
	err = netns.WithNetNS(ns.Path(), func() error {
		// Rename to eth0
		if err := veth.Rename("veth-cont", "eth0"); err != nil {
			return fmt.Errorf("rename failed: %w", err)
		}

		// Add IP
		contIP := &net.IPNet{
			IP:   net.ParseIP("10.0.0.2"),
			Mask: net.CIDRMask(24, 32),
		}
		if err := veth.AddIP("eth0", contIP); err != nil {
			return fmt.Errorf("add IP failed: %w", err)
		}

		// Bring up
		if err := veth.SetUp("eth0"); err != nil {
			return fmt.Errorf("set up failed: %w", err)
		}

		// Also bring up loopback
		if err := veth.SetUp("lo"); err != nil {
			return fmt.Errorf("set lo up failed: %w", err)
		}

		fmt.Println("   Renamed to eth0, IP: 10.0.0.2/24, State: UP")
		return nil
	})
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}

	// Step 6: Show final state
	fmt.Println("\n6. Final state:")
	fmt.Println("   Host namespace:")
	hostInfo, _ := veth.GetLinkInfo("veth-host")
	fmt.Printf("      veth-host: %s, MAC: %s\n", hostInfo["addrs"], hostInfo["mac"])

	fmt.Println("   Container namespace:")
	netns.WithNetNS(ns.Path(), func() error {
		contInfo, _ := veth.GetLinkInfo("eth0")
		fmt.Printf("      eth0: %s, MAC: %s\n", contInfo["addrs"], contInfo["mac"])
		return nil
	})

	fmt.Println("\n=== Demo Ready ===")
	fmt.Println("The veth pair is now configured. Test connectivity in another terminal:")
	fmt.Println("  ping -c 3 10.0.0.2  (from host)")
	fmt.Println("  sudo ip netns exec veth-demo ping -c 3 10.0.0.1  (from container)")
	fmt.Println("\nPress Enter to cleanup...")
	fmt.Scanln()

	fmt.Println("\n=== Cleaning up ===")
}