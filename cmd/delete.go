package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/fardeensayyed16/orca/internal/client"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a resource from the Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var deleteServiceCmd = &cobra.Command{
	Use:   "service <name>",
	Short: "Delete a service from the Orca cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(cmd)
		req, err := c.NewRequest(http.MethodDelete, "/api/v1/services/"+url.PathEscape(args[0]))
		if err != nil {
			return err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read service deletion response: %w", err)
		}
		if resp.StatusCode == http.StatusNoContent {
			fmt.Printf("Deleted service %s\n", args[0])
			return nil
		}
		return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
	},
}

var deleteNodeCmd = &cobra.Command{
	Use:   "node <name>",
	Short: "Drain and delete a worker node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(cmd)
		req, err := c.NewRequest(http.MethodDelete, "/api/v1/nodes/"+url.PathEscape(args[0]))
		if err != nil {
			return err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read node deletion response: %w", err)
		}

		fmt.Printf("Node deletion result: HTTP %d\n", resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusAccepted, http.StatusNoContent:
			return nil
		case http.StatusNotFound:
			return fmt.Errorf("node %q not found", args[0])
		default:
			return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
		}
	},
}

func init() {
	deleteCmd.AddCommand(deleteServiceCmd)
	deleteCmd.AddCommand(deleteNodeCmd)
	rootCmd.AddCommand(deleteCmd)
}
