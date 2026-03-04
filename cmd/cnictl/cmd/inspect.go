package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"syscall"

	"github.com/poridhioss/poridhi-cni/pkg/netns"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect CNI resources",
}

var inspectNsCmd = &cobra.Command{
	Use:   "ns <namespace>",
	Short: "Inspect a network namespace",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspectNs,
}

var inspectBridgeCmd = &cobra.Command{
	Use:   "bridge <name>",
	Short: "Inspect a bridge",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspectBridge,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.AddCommand(inspectNsCmd)
	inspectCmd.AddCommand(inspectBridgeCmd)
}

type NamespaceInfo struct {
	Name       string          `json:"name"`
	Interfaces []InterfaceInfo `json:"interfaces"`
	Routes     []RouteInfo     `json:"routes"`
}

type InterfaceInfo struct {
	Name  string   `json:"name"`
	State string   `json:"state"`
	MAC   string   `json:"mac"`
	IPs   []string `json:"ips"`
}

type RouteInfo struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Device      string `json:"device"`
}

func runInspectNs(cmd *cobra.Command, args []string) error {
	nsName := args[0]
	nsPath := fmt.Sprintf("/var/run/netns/%s", nsName)

	info := &NamespaceInfo{Name: nsName}

	err := netns.WithNetNS(nsPath, func() error {
		// Get interfaces
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			attrs := link.Attrs()
			ifInfo := InterfaceInfo{
				Name:  attrs.Name,
				State: attrs.OperState.String(),
				MAC:   attrs.HardwareAddr.String(),
			}

			addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
			for _, addr := range addrs {
				ifInfo.IPs = append(ifInfo.IPs, addr.IPNet.String())
			}

			info.Interfaces = append(info.Interfaces, ifInfo)
		}

		// Get routes
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err != nil {
			return err
		}

		for _, route := range routes {
			ri := RouteInfo{}

			if route.Dst != nil {
				ri.Destination = route.Dst.String()
			} else {
				ri.Destination = "default"
			}

			if route.Gw != nil {
				ri.Gateway = route.Gw.String()
			} else {
				ri.Gateway = "-"
			}

			if route.LinkIndex > 0 {
				link, err := netlink.LinkByIndex(route.LinkIndex)
				if err == nil {
					ri.Device = link.Attrs().Name
				}
			}

			info.Routes = append(info.Routes, ri)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to inspect namespace: %w", err)
	}

	return printNamespaceInfo(info)
}


func printNamespaceInfo(info *NamespaceInfo) error {
	if outputFormat == "json" {
		data, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("\nNamespace: %s\n", info.Name)
	fmt.Println(strings.Repeat("━", 60))

	fmt.Println("\nInterfaces:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tSTATE\tMAC\tIP")
	for _, iface := range info.Interfaces {
		ips := strings.Join(iface.IPs, ", ")
		if ips == "" {
			ips = "-"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", iface.Name, iface.State, iface.MAC, ips)
	}
	w.Flush()

	fmt.Println("\nRoutes:")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  DESTINATION\tGATEWAY\tDEVICE")
	for _, route := range info.Routes {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", route.Destination, route.Gateway, route.Device)
	}
	w.Flush()

	fmt.Println()
	return nil
}


func runInspectBridge(cmd *cobra.Command, args []string) error {
	bridgeName := args[0]

	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge not found: %w", err)
	}

	fmt.Printf("\nBridge: %s\n", bridgeName)
	fmt.Println(strings.Repeat("━", 60))

	// Get bridge IP
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	fmt.Println("\nAddresses:")
	for _, addr := range addrs {
		fmt.Printf("  %s\n", addr.IPNet.String())
	}

	// Get attached interfaces
	fmt.Println("\nAttached Interfaces:")
	links, _ := netlink.LinkList()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tSTATE\tMAC")
	for _, l := range links {
		if l.Attrs().MasterIndex == link.Attrs().Index {
			fmt.Fprintf(w, "  %s\t%s\t%s\n",
				l.Attrs().Name,
				l.Attrs().OperState.String(),
				l.Attrs().HardwareAddr.String())
		}
	}
	w.Flush()

	// Get FDB entries
	fmt.Println("\nFDB Entries:")
	neighs, _ := netlink.NeighList(link.Attrs().Index, syscall.AF_BRIDGE)
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  MAC\tPORT\tSTATE")
	for _, n := range neighs {
		portName := "-"
		if n.LinkIndex > 0 {
			if plink, err := netlink.LinkByIndex(n.LinkIndex); err == nil {
				portName = plink.Attrs().Name
			}
		}
		fmt.Fprintf(w, "  %s\t%s\t%d\n", n.HardwareAddr.String(), portName, n.State)
	}
	w.Flush()

	fmt.Println()
	return nil
}
