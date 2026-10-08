package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scaleCmd = &cobra.Command{
	Use:   "scale [service_name] --replicas [count]",
	Short: "Scale a service in Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Scaling service in Orca...")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	scaleCmd.Flags().IntP("replicas", "r", 1, "Number of replicas")
	rootCmd.AddCommand(scaleCmd)
}

