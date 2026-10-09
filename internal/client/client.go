package client

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fardeensayyed16/orca/pkg/config"
	"github.com/spf13/cobra"
)

// New returns an HTTP client with the configured endpoint, bearer token, and timeout.
func New(cmd *cobra.Command) *APIClient {
	server := config.GlobalConfig.Client.Server
	token := config.GlobalConfig.Client.Token
	timeout := 30 * time.Second
	if value, ok := os.LookupEnv("ORCA_SERVER"); ok {
		server = value
	}
	if value, ok := os.LookupEnv("ORCA_TOKEN"); ok {
		token = value
	}

	if cmd != nil {
		root := cmd.Root()
		if root != nil {
			flags := root.PersistentFlags()
			if flag := flags.Lookup("server"); flag != nil && flag.Changed {
				server = flag.Value.String()
			}
			if flag := flags.Lookup("token"); flag != nil && flag.Changed {
				token = flag.Value.String()
			}
			if flag := flags.Lookup("request-timeout"); flag != nil {
				if parsed, err := time.ParseDuration(flag.Value.String()); err == nil && parsed > 0 {
					timeout = parsed
				}
			}
		}
	}
	if server == "" {
		server = "http://localhost:9001"
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}

	return &APIClient{
		Server: server,
		Token:  token,
		HTTP: &http.Client{
			Timeout: timeout,
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
	req, err := http.NewRequest(method, strings.TrimRight(c.Server, "/")+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}
