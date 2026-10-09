package cmd

import (
	"fmt"
	"net/http"

	"github.com/fardeensayyed16/orca/pkg/types"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, status, err := apiRequest(cmd, http.MethodGet, "/api/v1/ping", nil)
		reachable := err == nil && status == http.StatusOK
		if err != nil {
			fmt.Println("Daemon reachable: no")
		} else if !reachable {
			fmt.Println("Daemon reachable: no")
		} else {
			fmt.Println("Daemon reachable: yes")
		}
		if !reachable {
			return nil
		}

		var services []serviceListItem
		if err := decodeResponse(cmd, "/api/v1/services", &services); err != nil {
			return err
		}
		var replicas []types.Replica
		if err := decodeResponse(cmd, "/api/v1/pods", &replicas); err != nil {
			return err
		}
		running, healthy, ready := 0, 0, 0
		for _, replica := range replicas {
			if replica.Status == types.PodStatusRunning {
				running++
			}
			if replica.Health == types.HealthStatusHealthy {
				healthy++
			}
			if replica.Ready {
				ready++
			}
		}
		fmt.Printf("Services: %d\nRunning replicas: %d\nHealthy replicas: %d\nReady replicas: %d\n",
			len(services), running, healthy, ready)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
