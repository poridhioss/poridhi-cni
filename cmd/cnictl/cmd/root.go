package cmd

import (
	"github.com/spf13/cobra"
)

var (
	outputFormat string
)

var rootCmd = &cobra.Command{
	Use:   "cnictl",
	Short: "CNI debugging and management tool",
	Long: `CNICtl is a command-line tool for debugging and managing
CNI plugin state, including namespaces, bridges, IPAM, and connectivity.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json)")
}
