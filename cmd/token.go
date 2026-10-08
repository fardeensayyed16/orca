package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage authentication tokens",
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show join token for worker nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Worker join token: stub-token-12345")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	tokenCmd.AddCommand(tokenShowCmd)
	rootCmd.AddCommand(tokenCmd)
}

