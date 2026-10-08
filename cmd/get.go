package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fardeensayyed16/orca/internal/client"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [nodes|services|pods]",
	Short: "Get resources from Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List all registered worker nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(cmd)
		req, err := c.NewRequest(http.MethodGet, "/api/v1/nodes")
		if err != nil {
			return err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusUnauthorized {
			var e map[string]string
			_ = json.Unmarshal(body, &e)
			return fmt.Errorf("unauthorized (HTTP 401): %s — check --token or ORCA_TOKEN", e["error"])
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
		}
		fmt.Print(string(body))
		return nil
	},
}

func init() {
	getCmd.AddCommand(getNodesCmd)
	rootCmd.AddCommand(getCmd)
}
