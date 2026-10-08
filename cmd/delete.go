package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [service_name]",
	Short: "Delete a service from Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Deleting service from Orca...")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

