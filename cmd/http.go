package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/fardeensayyed16/orca/internal/client"
	"github.com/spf13/cobra"
)

func apiRequest(cmd *cobra.Command, method, path string, payload any) ([]byte, int, error) {
	c := client.New(cmd)
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, strings.TrimRight(c.Server, "/")+path, body)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return responseBody, response.StatusCode, nil
}

func decodeResponse[T any](cmd *cobra.Command, path string, result *T) error {
	body, status, err := apiRequest(cmd, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return responseError(status, body)
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

func responseError(status int, body []byte) error {
	if status == http.StatusUnauthorized {
		var response struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &response)
		if response.Error != "" {
			return fmt.Errorf("unauthorized (HTTP 401): %s — check --token, ORCA_TOKEN, or client.token config", response.Error)
		}
	}
	return fmt.Errorf("server returned HTTP %d: %s", status, string(body))
}

func escapedPathSegment(value string) string {
	return url.PathEscape(value)
}
