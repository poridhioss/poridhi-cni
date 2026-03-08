package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/poridhioss/poridhi-cni/pkg/vxlan"
	"github.com/spf13/cobra"
)

var vxlanCmd = &cobra.Command{
	Use:   "vxlan",
	Short: "VXLAN operations",
}

var vxlanShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show VXLAN interface details",
	Args:  cobra.ExactArgs(1),
	RunE:  runVXLANShow,
}

var vxlanFDBCmd = &cobra.Command{
	Use:   "fdb <name>",
	Short: "Show FDB entries for VXLAN",
	Args:  cobra.ExactArgs(1),
	RunE:  runVXLANFDB,
}

func init() {
	rootCmd.AddCommand(vxlanCmd)
	vxlanCmd.AddCommand(vxlanShowCmd)
	vxlanCmd.AddCommand(vxlanFDBCmd)
}

func runVXLANShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	info, err := vxlan.GetInfo(name)
	if err != nil {
		return fmt.Errorf("failed to get vxlan info: %w", err)
	}

	if outputFormat == "json" {
		data, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("\nVXLAN Interface: %s\n", info.Name)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  VNI:      %d\n", info.VNI)
	fmt.Printf("  Port:     %d\n", info.Port)
	fmt.Printf("  Local IP: %s\n", info.LocalIP)
	fmt.Printf("  MTU:      %d\n", info.MTU)
	fmt.Printf("  State:    %s\n", info.State)

	if len(info.Addresses) > 0 {
		fmt.Printf("  IPs:      %s\n", strings.Join(info.Addresses, ", "))
	}

	fmt.Println()
	return nil
}

func runVXLANFDB(cmd *cobra.Command, args []string) error {
	name := args[0]

	entries, err := vxlan.ListFDBEntries(name)
	if err != nil {
		return fmt.Errorf("failed to list FDB: %w", err)
	}

	fmt.Printf("\nFDB Entries for %s\n", name)
	fmt.Println(strings.Repeat("=", 60))

	if len(entries) == 0 {
		fmt.Println("  No entries")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  MAC\tREMOTE VTEP")
	for _, e := range entries {
		fmt.Fprintf(w, "  %s\t%s\n", e.MAC.String(), e.RemoteIP.String())
	}
	w.Flush()

	fmt.Println()
	return nil
}


