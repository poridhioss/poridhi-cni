package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/poridhioss/poridhi-cni/pkg/netns"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debugging tools",
}

var debugConnectivityCmd = &cobra.Command{
	Use:   "connectivity <namespace>",
	Short: "Test container connectivity",
	Args:  cobra.ExactArgs(1),
	RunE:  runDebugConnectivity,
}

var debugIptablesCmd = &cobra.Command{
	Use:   "iptables",
	Short: "Show CNI iptables rules",
	RunE:  runDebugIptables,
}

func init() {
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugConnectivityCmd)
	debugCmd.AddCommand(debugIptablesCmd)
}

func runDebugConnectivity(cmd *cobra.Command, args []string) error {
	nsName := args[0]
	nsPath := fmt.Sprintf("/var/run/netns/%s", nsName)

	fmt.Printf("\nConnectivity Test: %s\n", nsName)
	fmt.Println(strings.Repeat("━", 60))

	tests := []struct {
		name   string
		target string
	}{
		{"Gateway", "10.244.0.1"},
		{"DNS (Google)", "8.8.8.8"},
		{"Internet (Cloudflare)", "1.1.1.1"},
	}

	for _, test := range tests {
		err := netns.WithNetNS(nsPath, func() error {
			cmd := exec.Command("ping", "-c", "1", "-W", "2", test.target)
			return cmd.Run()
		})

		if err == nil {
			fmt.Printf("  [PASS] %s (%s): reachable\n", test.name, test.target)
		} else {
			fmt.Printf("  [FAIL] %s (%s): unreachable\n", test.name, test.target)
		}
	}

	// DNS resolution test
	fmt.Println("\nDNS Resolution:")
	dnsCmd := exec.Command("ip", "netns", "exec", nsName, "nslookup", "example.com")
	output, err := dnsCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  [FAIL] DNS resolution failed: %v\n", err)
	} else {
		fmt.Printf("  %s\n", strings.ReplaceAll(strings.TrimSpace(string(output)), "\n", "\n  "))
	}

	fmt.Println()
	return nil
}

func runDebugIptables(cmd *cobra.Command, args []string) error {
	fmt.Println("\nCNI iptables Rules")
	fmt.Println(strings.Repeat("━", 60))

	// NAT rules
	fmt.Println("\nNAT Table (POSTROUTING):")
	output, _ := exec.Command("iptables", "-t", "nat", "-L", "POSTROUTING", "-n", "-v").CombinedOutput()
	fmt.Println(string(output))

	// FORWARD rules
	fmt.Println("Filter Table (FORWARD):")
	output, _ = exec.Command("iptables", "-L", "FORWARD", "-n", "-v").CombinedOutput()
	fmt.Println(string(output))

	return nil
}

