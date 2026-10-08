package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Start the Orca web GUI server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Orca web GUI...")
		// Stubbed implementation for Phase 1
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}

