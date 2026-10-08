package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fardeensayyed16/orca/internal/client"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage authentication tokens",
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the API token (loopback --server only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(cmd)
		req, err := c.NewRequest(http.MethodPost, "/api/v1/token/show")
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
			return fmt.Errorf("unauthorized (HTTP 401): %s", e["error"])
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
		}
		var result map[string]string
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Println(string(body))
			return nil
		}
		fmt.Println(result["token"])
		return nil
	},
}

func init() {
	tokenCmd.AddCommand(tokenShowCmd)
	rootCmd.AddCommand(tokenCmd)
}
