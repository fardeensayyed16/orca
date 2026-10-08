package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type DaemonConfig struct {
	BindAddress      string `mapstructure:"bind_address"`
	AdvertiseAddress string `mapstructure:"advertise_address"`
	GUIBindAddress   string `mapstructure:"gui_bind_address"`
	DataDir          string `mapstructure:"data_dir"`
	ServicePortRange string `mapstructure:"service_port_range"`
}

type ClientConfig struct {
	Server string `mapstructure:"server"`
	Token  string `mapstructure:"token"`
}

type Config struct {
	Daemon DaemonConfig `mapstructure:"daemon"`
	Client ClientConfig `mapstructure:"client"`
}

var GlobalConfig Config

func InitConfig(cmd *cobra.Command) error {
	v := viper.New()

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultDataDir := filepath.Join(home, ".orca")

	// Defaults
	v.SetDefault("daemon.bind_address", "0.0.0.0:9001")
	v.SetDefault("daemon.advertise_address", "")
	v.SetDefault("daemon.gui_bind_address", "127.0.0.1:9000")
	v.SetDefault("daemon.data_dir", defaultDataDir)
	v.SetDefault("daemon.service_port_range", "30000-32767")

	v.SetDefault("client.server", "http://localhost:9001")
	v.SetDefault("client.token", "")

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(filepath.Join(home, ".orca"))
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Environment variables
	v.SetEnvPrefix("ORCA")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if cmd != nil {
		_ = v.BindPFlags(cmd.Flags())
		_ = v.BindPFlags(cmd.PersistentFlags())
		// Explicit bindings for flags to nested viper keys if needed
		if f := cmd.Flags().Lookup("bind-address"); f != nil {
			_ = v.BindPFlag("daemon.bind_address", f)
		}
		if f := cmd.Flags().Lookup("advertise-address"); f != nil {
			_ = v.BindPFlag("daemon.advertise_address", f)
		}
		if f := cmd.Flags().Lookup("data-dir"); f != nil {
			_ = v.BindPFlag("daemon.data_dir", f)
		}
		if f := cmd.PersistentFlags().Lookup("server"); f != nil {
			_ = v.BindPFlag("client.server", f)
		}
		if f := cmd.PersistentFlags().Lookup("token"); f != nil {
			_ = v.BindPFlag("client.token", f)
		}
	}

	if err := v.Unmarshal(&GlobalConfig); err != nil {
		return fmt.Errorf("unable to decode into config struct: %w", err)
	}

	return nil
}
