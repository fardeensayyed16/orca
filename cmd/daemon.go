package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fardeensayyed16/orca/internal/api"
	"github.com/fardeensayyed16/orca/internal/proxy"
	"github.com/fardeensayyed16/orca/internal/storage"
	"github.com/fardeensayyed16/orca/pkg/config"
	"github.com/spf13/cobra"
)

var (
	daemonBindAddress      string
	daemonAdvertiseAddress string
	daemonNoGUI            bool
	daemonDataDir          string
	daemonConfig           string
)

var daemonCmd = &cobra.Command{
	Use:   "daemon [flags]",
	Short: "Start the Orca daemon server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// 1. Resolve configuration
		bindAddr := daemonBindAddress
		if bindAddr == "" {
			bindAddr = config.GlobalConfig.Daemon.BindAddress
		}
		if bindAddr == "" {
			bindAddr = "0.0.0.0:9001"
		}

		advAddr := daemonAdvertiseAddress
		if advAddr == "" {
			advAddr = config.GlobalConfig.Daemon.AdvertiseAddress
		}
		if advAddr == "" {
			advAddr = bindAddr
		}

		guiBindAddr := config.GlobalConfig.Daemon.GUIBindAddress
		if guiBindAddr == "" {
			guiBindAddr = "127.0.0.1:9000"
		}

		dataDir := daemonDataDir
		if dataDir == "" {
			dataDir = config.GlobalConfig.Daemon.DataDir
		}

		// 2. Initialize SQLite Storage
		store, err := storage.NewSQLiteStorage(dataDir)
		if err != nil {
			return fmt.Errorf("storage initialization failed: %w", err)
		}
		defer store.Close()

		// 3. Restore proxy listeners (Rule C2)
		proxyMgr := proxy.NewProxyManager(store, bindAddr)
		defer proxyMgr.Close()

		if err := proxyMgr.RestoreListeners(ctx); err != nil {
			log.Printf("Warning: restoring proxy listeners failed: %v", err)
		}

		// 4. Create API Server
		apiServer := api.NewServer(store)
		apiHTTP := &http.Server{
			Addr:    bindAddr,
			Handler: apiServer,
		}

		// 5. Start API Server goroutine
		go func() {
			if err := apiHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("API server failure: %v", err)
			}
		}()

		// 6. GUI Listener (Stub or enabled)
		var guiHTTP *http.Server
		if !daemonNoGUI {
			guiMux := http.NewServeMux()
			guiMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("orca GUI"))
			})

			guiHTTP = &http.Server{
				Addr:    guiBindAddr,
				Handler: guiMux,
			}

			go func() {
				if err := guiHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("GUI server error: %v", err)
				}
			}()
		}

		// 7. Output exactly the 3 required startup log lines
		fmt.Printf("API listening on %s\n", bindAddr)
		if !daemonNoGUI {
			fmt.Printf("GUI listening on %s\n", guiBindAddr)
		} else {
			fmt.Printf("GUI listening on %s (disabled)\n", guiBindAddr)
		}
		fmt.Printf("Advertise address: %s\n", advAddr)

		// 8. Wait for shutdown signal
		<-ctx.Done()

		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if guiHTTP != nil {
			_ = guiHTTP.Shutdown(shutdownCtx)
		}
		_ = apiHTTP.Shutdown(shutdownCtx)

		return nil
	},
}

func init() {
	daemonCmd.Flags().StringVar(&daemonBindAddress, "bind-address", "0.0.0.0:9001", "API server bind address")
	daemonCmd.Flags().StringVar(&daemonAdvertiseAddress, "advertise-address", "", "Advertised address for cluster nodes")
	daemonCmd.Flags().BoolVar(&daemonNoGUI, "no-gui", false, "Disable embedded web GUI")
	daemonCmd.Flags().StringVar(&daemonDataDir, "data-dir", "", "Data directory for SQLite database")
	daemonCmd.Flags().StringVar(&daemonConfig, "config", "", "Path to config file (default: ~/.orca/config.yaml)")

	rootCmd.AddCommand(daemonCmd)
}
