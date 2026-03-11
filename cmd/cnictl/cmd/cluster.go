package cmd

import (
	"fmt"

	"github.com/poridhioss/poridhi-cni/pkg/cluster"
	"github.com/spf13/cobra"
)

var clusterConfigPath string

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Cluster operations",
}

var clusterInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cluster networking on this node",
	RunE:  runClusterInit,
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster network status",
	RunE:  runClusterStatus,
}

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterStatusCmd)

	clusterInitCmd.Flags().StringVarP(&clusterConfigPath, "config", "c", "/etc/cni/cluster.json", "Cluster config path")
	clusterStatusCmd.Flags().StringVarP(&clusterConfigPath, "config", "c", "/etc/cni/cluster.json", "Cluster config path")
}

func runClusterInit(cmd *cobra.Command, args []string) error {
	fmt.Printf("Loading cluster config from %s...\n", clusterConfigPath)

	config, err := cluster.LoadConfig(clusterConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	mgr, err := cluster.NewNetworkManager(config)
	if err != nil {
		return fmt.Errorf("failed to create network manager: %w", err)
	}

	fmt.Printf("Initializing network for node: %s\n", mgr.GetLocalNode().Name)
	fmt.Printf("Pod CIDR: %s\n", mgr.GetPodCIDR())

	if err := mgr.Setup(); err != nil {
		return fmt.Errorf("failed to setup network: %w", err)
	}

	fmt.Println("\nCluster network initialized successfully!")
	fmt.Println("\nCreated:")
	fmt.Printf("  Bridge: cni0\n")
	fmt.Printf("  VXLAN:  vxlan0 (VNI=%d)\n", config.VNI)

	remotes := config.GetRemoteNodes(mgr.GetLocalNode())
	if len(remotes) > 0 {
		fmt.Println("\nRemote nodes:")
		for _, n := range remotes {
			fmt.Printf("  %s: %s (%s)\n", n.Name, n.IP, n.PodCIDR)
		}
	}

	return nil
}

func runClusterStatus(cmd *cobra.Command, args []string) error {
	config, err := cluster.LoadConfig(clusterConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mgr, err := cluster.NewNetworkManager(config)
	if err != nil {
		return fmt.Errorf("failed to create network manager: %w", err)
	}

	local := mgr.GetLocalNode()
	fmt.Printf("\nCluster Network Status\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Local Node: %s (%s)\n", local.Name, local.IP)
	fmt.Printf("Pod CIDR:   %s\n", local.PodCIDR)
	fmt.Printf("VNI:        %d\n\n", config.VNI)

	fmt.Println("All Nodes:")
	for _, n := range config.Nodes {
		marker := "  "
		if n.Name == local.Name {
			marker = "* "
		}
		fmt.Printf("%s%s: %s (%s)\n", marker, n.Name, n.IP, n.PodCIDR)
	}

	return nil
}
