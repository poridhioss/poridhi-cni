package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/poridhioss/poridhi-cni/pkg/iptables"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "setup":
		setupNAT()
	case "cleanup":
		cleanupNAT()
	case "status":
		showStatus()
	case "conntrack":
		showConntrack()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: nat-test <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  setup      Setup NAT and FORWARD rules")
	fmt.Println("  cleanup    Remove NAT and FORWARD rules")
	fmt.Println("  status     Show current iptables rules")
	fmt.Println("  conntrack  Show connection tracking entries (optional: filter IP as arg)")
}

func setupNAT() {
	mgr, err := iptables.NewManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.SetupNAT("10.244.0.0/24", "cni0"); err != nil {
		fmt.Printf("Error setting up NAT: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("MASQUERADE rule added")

	if err := mgr.SetupForward("10.244.0.0/24", "cni0"); err != nil {
		fmt.Printf("Error setting up FORWARD: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("FORWARD rules added")

	fmt.Println("\nNAT setup complete. Containers should now have internet access.")
}

func cleanupNAT() {
	mgr, err := iptables.NewManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.CleanupNAT("10.244.0.0/24", "cni0"); err != nil {
		fmt.Printf("Error cleaning up NAT: %v\n", err)
	} else {
		fmt.Println("MASQUERADE rule removed")
	}

	if err := mgr.CleanupForward("cni0"); err != nil {
		fmt.Printf("Error cleaning up FORWARD: %v\n", err)
	} else {
		fmt.Println("FORWARD rules removed")
	}
}

func showStatus() {
	mgr, err := iptables.NewManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== CNI iptables Rules ===")
	fmt.Println()

	rules, err := mgr.GetCNIRules()
	if err != nil {
		fmt.Printf("Error getting rules: %v\n", err)
		os.Exit(1)
	}

	if len(rules) == 0 {
		fmt.Println("No CNI-related iptables rules found.")
		return
	}

	for _, r := range rules {
		fmt.Printf("[%s/%s] %s\n", r.Table, r.Chain, r.Rule)
	}

	fmt.Println()
	fmt.Println("=== All NAT POSTROUTING Rules ===")
	fmt.Println()

	natRules, err := mgr.ListNATRules()
	if err != nil {
		fmt.Printf("Error listing NAT rules: %v\n", err)
		return
	}
	for _, r := range natRules {
		fmt.Println(r)
	}

	fmt.Println()
	fmt.Println("=== All FORWARD Rules ===")
	fmt.Println()

	fwdRules, err := mgr.ListForwardRules()
	if err != nil {
		fmt.Printf("Error listing FORWARD rules: %v\n", err)
		return
	}
	for _, r := range fwdRules {
		fmt.Println(r)
	}
}

func showConntrack() {
	filterIP := ""
	if len(os.Args) >= 3 {
		filterIP = os.Args[2]
	}

	entries, err := iptables.GetConntrackEntries(filterIP)
	if err != nil {
		fmt.Printf("Error reading conntrack: %v\n", err)
		fmt.Println("Tip: Try 'sudo modprobe nf_conntrack' if the module isn't loaded")
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No conntrack entries found.")
		if filterIP != "" {
			fmt.Printf("(filtered by: %s)\n", filterIP)
		}
		return
	}

	fmt.Printf("%-6s %-12s %-16s %-16s %-8s %-8s %-16s %-16s\n",
		"PROTO", "STATE", "ORIG_SRC", "ORIG_DST", "SPORT", "DPORT", "REPLY_SRC", "REPLY_DST")
	fmt.Println(strings.Repeat("-", 100))

	for _, e := range entries {
		state := e.State
		if state == "" {
			state = "-"
		}
		replySrc := e.ReplySrc
		if replySrc == "" {
			replySrc = "-"
		}
		replyDst := e.ReplyDst
		if replyDst == "" {
			replyDst = "-"
		}
		fmt.Printf("%-6s %-12s %-16s %-16s %-8s %-8s %-16s %-16s\n",
			e.Protocol, state, e.OrigSrc, e.OrigDst, e.OrigSPort, e.OrigDPort, replySrc, replyDst)
	}
}
