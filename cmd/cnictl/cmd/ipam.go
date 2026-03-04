package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var ipamCmd = &cobra.Command{
	Use:   "ipam",
	Short: "IPAM operations",
}

var ipamListCmd = &cobra.Command{
	Use:   "list [network]",
	Short: "List IP allocations",
	RunE:  runIPAMList,
}

var ipamStatsCmd = &cobra.Command{
	Use:   "stats [network]",
	Short: "Show IPAM statistics",
	RunE:  runIPAMStats,
}

func init() {
	rootCmd.AddCommand(ipamCmd)
	ipamCmd.AddCommand(ipamListCmd)
	ipamCmd.AddCommand(ipamStatsCmd)
}

type IPAllocation struct {
	IP          string `json:"ip"`
	ContainerID string `json:"container_id"`
}

type IPAMInfo struct {
	Network     string         `json:"network"`
	Allocations []IPAllocation `json:"allocations"`
	Total       int            `json:"total"`
}

func runIPAMList(cmd *cobra.Command, args []string) error {
	baseDir := "/var/lib/cni/networks"

	var networks []string
	if len(args) > 0 {
		networks = []string{args[0]}
	} else {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to read IPAM directory: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				networks = append(networks, e.Name())
			}
		}
	}

	for _, network := range networks {
		info, err := getIPAMInfo(baseDir, network)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get info for %s: %v\n", network, err)
			continue
		}

		if outputFormat == "json" {
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
		} else {
			printIPAMInfo(info)
		}
	}

	return nil
}

func getIPAMInfo(baseDir, network string) (*IPAMInfo, error) {
	networkDir := filepath.Join(baseDir, network)

	info := &IPAMInfo{
		Network: network,
	}

	entries, err := os.ReadDir(networkDir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		name := e.Name()
		// Skip non-IP files
		if name == "lock" || strings.HasPrefix(name, "last_") {
			continue
		}

		// Read container ID
		data, err := os.ReadFile(filepath.Join(networkDir, name))
		if err != nil {
			continue
		}

		info.Allocations = append(info.Allocations, IPAllocation{
			IP:          name,
			ContainerID: strings.TrimSpace(string(data)),
		})
	}

	info.Total = len(info.Allocations)
	return info, nil
}

func printIPAMInfo(info *IPAMInfo) {
	fmt.Printf("\nNetwork: %s\n", info.Network)
	fmt.Println(strings.Repeat("━", 60))
	fmt.Printf("Total Allocations: %d\n\n", info.Total)

	if len(info.Allocations) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "IP\tCONTAINER")
		for _, alloc := range info.Allocations {
			fmt.Fprintf(w, "%s\t%s\n", alloc.IP, alloc.ContainerID)
		}
		w.Flush()
	} else {
		fmt.Println("No allocations")
	}
	fmt.Println()
}

func runIPAMStats(cmd *cobra.Command, args []string) error {
	fmt.Println("IPAM Statistics")
	fmt.Println(strings.Repeat("━", 60))

	baseDir := "/var/lib/cni/networks"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NETWORK\tALLOCATED\tAVAILABLE\tUTILIZATION")

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		info, err := getIPAMInfo(baseDir, e.Name())
		if err != nil {
			continue
		}

		// Assume /24 subnet = 254 usable IPs
		available := 254 - info.Total
		utilization := float64(info.Total) / 254.0 * 100

		fmt.Fprintf(w, "%s\t%d\t%d\t%.1f%%\n",
			info.Network, info.Total, available, utilization)
	}
	w.Flush()

	return nil
}


