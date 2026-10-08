package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [services|nodes|replicas]",
	Short: "Get resources from Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Getting resources from Orca...")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}

