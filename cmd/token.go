package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
		if err := requireLoopbackServer(c.Server); err != nil {
			return fmt.Errorf("token show requires a loopback --server: %w", err)
		}
		body, status, err := apiRequest(cmd, http.MethodPost, "/api/v1/token/show", nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return responseError(status, body)
		}
		var result map[string]string
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("decode token response: %w", err)
		}
		if result["token"] == "" {
			return fmt.Errorf("token response did not contain a token")
		}
		fmt.Println(result["token"])
		return nil
	},
}

func init() {
	tokenCmd.AddCommand(tokenShowCmd)
	rootCmd.AddCommand(tokenCmd)
}

func requireLoopbackServer(server string) error {
	parsed, err := url.Parse(server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	return fmt.Errorf("server host %q is not loopback", host)
}
