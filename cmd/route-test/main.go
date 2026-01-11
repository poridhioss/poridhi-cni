package main

import (
	"fmt"
	"os"
	"strings"
	
	"github.com/poridhioss/poridhi-cni/pkg/route"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "status":
		showStatus()
	case "enable":
		enableForwarding()
	case "disable":
		disableForwarding()
	case "persist":
		persistForwarding()
	case "routes":
		showRoutes()
	case "neighbors":
		showNeighbors()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: route-test <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  status     Show IP forwarding status")
	fmt.Println("  enable     Enable IP forwarding")
	fmt.Println("  disable    Disable IP forwarding")
	fmt.Println("  persist    Make IP forwarding persistent")
	fmt.Println("  routes     Show routing table")
	fmt.Println("  neighbors  Show neighbor cache (requires interface as arg)")
}

func showStatus() {
	enabled, err := route.IsIPForwardingEnabled()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if enabled {
		fmt.Println("IP forwarding: ENABLED")
	} else {
		fmt.Println("IP forwarding: DISABLED")
	}
}

func enableForwarding() {
	if err := route.EnableIPForwarding(); err != nil {
		fmt.Printf("Error enabling IP forwarding: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("IP forwarding enabled")
}

func disableForwarding() {
	if err := route.DisableIPForwarding(); err != nil {
		fmt.Printf("Error disabling IP forwarding: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("IP forwarding disabled")
}

func persistForwarding() {
	if err := route.PersistIPForwarding(); err != nil {
		fmt.Printf("Error persisting IP forwarding: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("IP forwarding persistence configured")
	fmt.Println("Run 'sudo sysctl --system' to apply")
}

func showRoutes() {
	routes, err := route.GetRouteTable()
	if err != nil {
		fmt.Printf("Error getting routes: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%-20s %-16s %-10s %s\n", "DESTINATION", "GATEWAY", "DEVICE", "SCOPE")
	fmt.Println(strings.Repeat("-", 60))

	for _, r := range routes {
		gw := r.Gateway
		if gw == "" {
			gw = "-"
		}
		fmt.Printf("%-20s %-16s %-10s %s\n", r.Destination, gw, r.Device, r.Scope)
	}
}

func showNeighbors() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: route-test neighbors <interface>")
		os.Exit(1)
	}

	linkName := os.Args[2]
	neighbors, err := route.GetNeighbors(linkName)
	if err != nil {
		fmt.Printf("Error getting neighbors: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%-16s %-20s %s\n", "IP", "MAC", "STATE")
	fmt.Println(strings.Repeat("-", 50))

	for _, n := range neighbors {
		fmt.Printf("%-16s %-20s %s\n", n.IP, n.MAC, n.State)
	}
}


