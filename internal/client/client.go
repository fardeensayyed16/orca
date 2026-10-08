package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fardeensayyed16/orca/pkg/config"
	"github.com/spf13/cobra"
)

// New returns an HTTP client configured with the bearer token and 30s timeout.
// The server URL and token are resolved via flag > env > config file > default.
func New(cmd *cobra.Command) *APIClient {
	server := config.GlobalConfig.Client.Server
	token := config.GlobalConfig.Client.Token

	if cmd != nil {
		if s, _ := cmd.Flags().GetString("server"); s != "" {
			server = s
		}
		if t, _ := cmd.Flags().GetString("token"); t != "" {
			token = t
		}
	}

	if server == "" {
		server = "http://localhost:9001"
	}

	return &APIClient{
		Server: server,
		Token:  token,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type APIClient struct {
	Server string
	Token  string
	HTTP   *http.Client
}

// NewRequest creates a request with the Authorization header pre-set.
func (c *APIClient) NewRequest(method, path string) (*http.Request, error) {
	url := c.Server + path
	// Ensure scheme
	if len(url) > 0 && url[0] != 'h' {
		url = "http://" + url
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}
