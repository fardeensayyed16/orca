package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"

	"github.com/fardeensayyed16/orca/internal/client"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the Orca web GUI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(cmd)
		if err := requireLoopbackServer(c.Server); err != nil {
			return fmt.Errorf("gui requires a loopback --server: %w", err)
		}
		target := "http://127.0.0.1:9000"
		if err := openBrowser(target); err != nil {
			return err
		}
		fmt.Printf("Opened %s\n", target)
		return nil
	},
}

func openBrowser(target string) error {
	if _, err := url.ParseRequestURI(target); err != nil {
		return fmt.Errorf("invalid browser URL: %w", err)
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		command = exec.Command("open", target)
	default:
		if _, wsl := os.LookupEnv("WSL_DISTRO_NAME"); wsl {
			command = exec.Command("cmd.exe", "/c", "start", "", target)
		} else {
			command = exec.Command("xdg-open", target)
		}
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
