package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Orca cluster status: OK")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

