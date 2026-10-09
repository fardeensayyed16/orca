package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fardeensayyed16/orca/pkg/config"
	"github.com/spf13/cobra"
)

var (
	ServerFlag     string
	TokenFlag      string
	ConfigFlag     string
	RequestTimeout time.Duration
)

var rootCmd = &cobra.Command{
	Use:           "orca",
	Short:         "Orca Container Orchestrator CLI",
	Long:          `Orca is a lightweight container orchestration system.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitConfig(cmd); err != nil {
			return err
		}
		if flag := cmd.Root().PersistentFlags().Lookup("server"); flag != nil && flag.Changed {
			ServerFlag = flag.Value.String()
		}
		if flag := cmd.Root().PersistentFlags().Lookup("token"); flag != nil && flag.Changed {
			TokenFlag = flag.Value.String()
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&ServerFlag, "server", "", "Orca master server URL")
	rootCmd.PersistentFlags().StringVar(&TokenFlag, "token", "", "Authentication token")
	rootCmd.PersistentFlags().StringVar(&ConfigFlag, "config", "", "Path to config file")
	rootCmd.PersistentFlags().DurationVar(&RequestTimeout, "request-timeout", 30*time.Second, "HTTP request timeout")
}
