package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var scaleReplicas int

var scaleCmd = &cobra.Command{
	Use:   "scale <service> --replicas <count>",
	Short: "Scale a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if scaleReplicas <= 0 {
			return fmt.Errorf("--replicas must be greater than zero")
		}
		body, status, err := apiRequest(cmd, http.MethodPatch,
			"/api/v1/services/"+escapedPathSegment(args[0])+"/scale",
			map[string]int{"replicas": scaleReplicas})
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return responseError(status, body)
		}
		fmt.Printf("Scaled service %s to %d replicas\n", args[0], scaleReplicas)
		return nil
	},
}

func init() {
	scaleCmd.Flags().IntVarP(&scaleReplicas, "replicas", "r", 0, "Desired replica count")
	rootCmd.AddCommand(scaleCmd)
}
