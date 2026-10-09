package client

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fardeensayyed16/orca/pkg/config"
	"github.com/spf13/cobra"
)

func TestNewResolvesFlagsBeforeEnvironmentAndConfig(t *testing.T) {
	oldConfig := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = oldConfig })
	t.Setenv("ORCA_SERVER", "http://env.example:9001")
	t.Setenv("ORCA_TOKEN", "env-token")
	config.GlobalConfig.Client.Server = "http://config.example:9001"
	config.GlobalConfig.Client.Token = "config-token"

	root := &cobra.Command{Use: "orca"}
	root.PersistentFlags().String("server", "", "")
	root.PersistentFlags().String("token", "", "")
	root.PersistentFlags().Duration("request-timeout", 30*time.Second, "")
	if err := root.PersistentFlags().Set("server", "http://flag.example:9001"); err != nil {
		t.Fatal(err)
	}
	if err := root.PersistentFlags().Set("token", "flag-token"); err != nil {
		t.Fatal(err)
	}
	if err := root.PersistentFlags().Set("request-timeout", "5s"); err != nil {
		t.Fatal(err)
	}

	api := New(root)
	if api.Server != "http://flag.example:9001" || api.Token != "flag-token" {
		t.Fatalf("flag precedence failed: server=%q token=%q", api.Server, api.Token)
	}
	if api.HTTP.Timeout != 5*time.Second {
		t.Fatalf("timeout = %s, want 5s", api.HTTP.Timeout)
	}
	request, err := api.NewRequest(http.MethodGet, "/api/v1/nodes")
	if err != nil {
		t.Fatal(err)
	}
	wantAuthorization := strings.Join([]string{"Bearer", api.Token}, " ")
	if got := request.Header.Get("Authorization"); got != wantAuthorization {
		t.Fatalf("Authorization header does not contain configured bearer token")
	}
}

func TestNewUsesEnvironmentBeforeConfigAndDefault(t *testing.T) {
	oldConfig := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = oldConfig })
	t.Setenv("ORCA_SERVER", "env.example:9001")
	t.Setenv("ORCA_TOKEN", "env-token")
	config.GlobalConfig.Client.Server = "http://config.example:9001"
	config.GlobalConfig.Client.Token = "config-token"
	api := New(nil)
	if api.Server != "http://env.example:9001" || api.Token != "env-token" {
		t.Fatalf("environment precedence failed: server=%q token=%q", api.Server, api.Token)
	}

	if err := os.Unsetenv("ORCA_SERVER"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("ORCA_TOKEN"); err != nil {
		t.Fatal(err)
	}
	api = New(nil)
	if api.Server != "http://config.example:9001" || api.Token != "config-token" {
		t.Fatalf("config fallback failed: server=%q token=%q", api.Server, api.Token)
	}

	config.GlobalConfig.Client.Server = ""
	config.GlobalConfig.Client.Token = ""
	api = New(nil)
	if api.Server != "http://localhost:9001" || api.Token != "" {
		t.Fatalf("default resolution failed: server=%q token=%q", api.Server, api.Token)
	}
}
