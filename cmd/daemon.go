package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	daemonBindAddress      string
	daemonAdvertiseAddress string
	daemonNoGUI            bool
	daemonDataDir          string
	daemonConfig           string
)

var daemonCmd = &cobra.Command{
	Use:   "daemon [flags]",
	Short: "Start the Orca daemon server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Orca daemon...")
		return nil
	},
}

func init() {
	daemonCmd.Flags().StringVar(&daemonBindAddress, "bind-address", "0.0.0.0:9001", "API server bind address")
	daemonCmd.Flags().StringVar(&daemonAdvertiseAddress, "advertise-address", "", "Advertised address for cluster nodes")
	daemonCmd.Flags().BoolVar(&daemonNoGUI, "no-gui", false, "Disable embedded web GUI")
	daemonCmd.Flags().StringVar(&daemonDataDir, "data-dir", "", "Data directory for SQLite database")
	daemonCmd.Flags().StringVar(&daemonConfig, "config", "", "Path to config file (default: ~/.orca/config.yaml)")

	rootCmd.AddCommand(daemonCmd)
}
