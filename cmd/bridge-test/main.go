package main

import (
	"fmt"
	"net"
	"os"

	"github.com/poridhioss/poridhi-cni/pkg/bridge"
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
		if len(os.Args) < 3 {
			fmt.Println("Usage: bridge-test create <name>")
			os.Exit(1)
		}
		createBridge(os.Args[2])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: bridge-test delete <name>")
			os.Exit(1)
		}
		deleteBridge(os.Args[2])

	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: bridge-test info <name>")
			os.Exit(1)
		}
		showInfo(os.Args[2])

	case "attach":
		if len(os.Args) < 4 {
			fmt.Println("Usage: bridge-test attach <bridge> <interface>")
			os.Exit(1)
		}
		attachInterface(os.Args[2], os.Args[3])

	case "fdb":
		if len(os.Args) < 3 {
			fmt.Println("Usage: bridge-test fdb <name>")
			os.Exit(1)
		}
		showFDB(os.Args[2])

	case "demo":
		runDemo()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: bridge-test <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  create <name>              - Create a bridge")
	fmt.Println("  delete <name>              - Delete a bridge")
	fmt.Println("  info <name>                - Show bridge info")
	fmt.Println("  attach <bridge> <iface>    - Attach interface to bridge")
	fmt.Println("  fdb <name>                 - Show forwarding database")
	fmt.Println("  demo                       - Run full demo with containers")
}

func createBridge(name string) {
	br, err := bridge.Create(name)
	if err != nil {
		fmt.Printf("Failed to create bridge: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created bridge: %s\n", br.Name)
}

func deleteBridge(name string) {
	br, err := bridge.Create(name)
	if err != nil {
		fmt.Printf("Bridge not found: %v\n", err)
		os.Exit(1)
	}

	if err := br.Delete(); err != nil {
		fmt.Printf("Failed to delete bridge: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted bridge: %s\n", name)
}

func showInfo(name string) {
	br, err := bridge.Create(name)
	if err != nil {
		fmt.Printf("Bridge not found: %v\n", err)
		os.Exit(1)
	}

	info, err := br.GetInfo()
	if err != nil {
		fmt.Printf("Failed to get info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Bridge: %s\n", info["name"])
	fmt.Printf("  MAC:       %s\n", info["mac"])
	fmt.Printf("  MTU:       %d\n", info["mtu"])
	fmt.Printf("  State:     %s\n", info["state"])
	fmt.Printf("  Addresses: %v\n", info["addresses"])
	fmt.Printf("  Attached:  %v\n", info["attached"])
}

func showFDB(name string) {
	br, err := bridge.Create(name)
	if err != nil {
		fmt.Printf("Bridge not found: %v\n", err)
		os.Exit(1)
	}

	entries, err := br.GetFDB()
	if err != nil {
		fmt.Printf("Failed to get FDB: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Attached interfaces for %s:\n", name)
	if len(entries) == 0 {
		fmt.Println("  (no interfaces attached)")
	}
	for _, entry := range entries {
		fmt.Printf("  %s (MAC: %s, idx: %d)\n", entry.PortName, entry.MAC, entry.PortIdx)
	}
	fmt.Println("\nFor full FDB with learned MACs, run:")
	fmt.Printf("  bridge fdb show dev %s\n", name)
}

func attachInterface(bridgeName, ifName string) {
	br, err := bridge.Create(bridgeName)
	if err != nil {
		fmt.Printf("Bridge not found: %v\n", err)
		os.Exit(1)
	}

	if err := br.AttachInterface(ifName); err != nil {
		fmt.Printf("Failed to attach: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Attached %s to %s\n", ifName, bridgeName)
}

func runDemo() {
	fmt.Println("=== Bridge Demo ===")
	fmt.Println()

	// Step 1: Create bridge
	fmt.Println("1. Creating bridge 'cni0'...")
	br, err := bridge.Create("cni0")
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		br.Delete()
		fmt.Println("\n8. Deleted bridge")
	}()
	fmt.Println("   Created bridge: cni0")

	// Step 2: Assign gateway IP
	fmt.Println("\n2. Assigning gateway IP 10.244.0.1/24...")
	if err := br.SetIP("10.244.0.1/24"); err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   Assigned: 10.244.0.1/24")

	// Step 3: Bring bridge up
	fmt.Println("\n3. Bringing bridge up...")
	if err := br.SetUp(); err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   Bridge is UP")

	// Step 4: Create namespace for container
	fmt.Println("\n4. Creating container namespace...")
	ns, err := netns.CreateNamed("bridge-demo")
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		netns.Delete("bridge-demo")
		fmt.Println("9. Deleted namespace")
	}()
	fmt.Println("   Created: /var/run/netns/bridge-demo")

	// Step 5: Create veth pair
	fmt.Println("\n5. Creating veth pair...")
	pair, err := veth.Create("veth-host", "veth-cont", 1500)
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		veth.DeleteByName("veth-host")
		fmt.Println("7. Deleted veth pair")
	}()
	fmt.Println("   Created: veth-host <--> veth-cont")

	// Step 6: Attach veth to bridge
	fmt.Println("\n6. Attaching veth-host to bridge...")
	if err := br.AttachInterface("veth-host"); err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	if err := veth.SetUp("veth-host"); err != nil {
		fmt.Printf("   Failed to bring up: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   Attached and UP")

	// Step 7: Move veth to namespace and configure
	fmt.Println("\n7. Configuring container networking...")
	if err := pair.MoveToNetNS(ns.Path()); err != nil {
		fmt.Printf("   Failed to move veth: %v\n", err)
		os.Exit(1)
	}

	err = netns.WithNetNS(ns.Path(), func() error {
		if err := veth.Rename("veth-cont", "eth0"); err != nil {
			return fmt.Errorf("rename: %w", err)
		}
		contIP := &net.IPNet{
			IP:   net.ParseIP("10.244.0.2"),
			Mask: net.CIDRMask(24, 32),
		}
		if err := veth.AddIP("eth0", contIP); err != nil {
			return fmt.Errorf("add IP: %w", err)
		}
		if err := veth.SetUp("eth0"); err != nil {
			return fmt.Errorf("set up eth0: %w", err)
		}
		if err := veth.SetUp("lo"); err != nil {
			return fmt.Errorf("set up lo: %w", err)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("   Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   Container eth0: 10.244.0.2/24")

	// Show bridge info
	fmt.Println("\n=== Bridge State ===")
	info, _ := br.GetInfo()
	fmt.Printf("Bridge: %s\n", info["name"])
	fmt.Printf("  Gateway IP: %v\n", info["addresses"])
	fmt.Printf("  Attached:   %v\n", info["attached"])

	fmt.Println("\n=== Demo Ready ===")
	fmt.Println("Test connectivity in another terminal:")
	fmt.Println("  sudo ping -c 3 10.244.0.2                              (host to container)")
	fmt.Println("  sudo ip netns exec bridge-demo ping -c 3 10.244.0.1    (container to gateway)")
	fmt.Println("\nPress Enter to cleanup...")
	fmt.Scanln()

	fmt.Println("\n=== Cleaning up ===")
}