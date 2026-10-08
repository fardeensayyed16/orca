package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fardeensayyed16/orca/internal/api"
	"github.com/fardeensayyed16/orca/internal/proxy"
	"github.com/fardeensayyed16/orca/internal/reconciler"
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

		// Resolve addresses
		bindAddr := firstNonEmpty(daemonBindAddress, config.GlobalConfig.Daemon.BindAddress, "0.0.0.0:9001")
		guiBindAddr := firstNonEmpty(config.GlobalConfig.Daemon.GUIBindAddress, "127.0.0.1:9000")
		advAddr := firstNonEmpty(daemonAdvertiseAddress, config.GlobalConfig.Daemon.AdvertiseAddress, bindAddr)
		dataDir := firstNonEmpty(daemonDataDir, config.GlobalConfig.Daemon.DataDir)

		// 1. SQLite Storage
		store, err := storage.NewSQLiteStorage(dataDir)
		if err != nil {
			return fmt.Errorf("storage initialization failed: %w", err)
		}
		defer store.Close()

		// 2. Token: load or auto-generate
		token, err := store.GetToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		if token == "" {
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return fmt.Errorf("failed to generate token: %w", err)
			}
			token = hex.EncodeToString(raw)
			if err := store.SetToken(ctx, token); err != nil {
				return fmt.Errorf("failed to persist token: %w", err)
			}
			fmt.Printf("Generated API token: %s. Save this — orclet and CLI need it.\n", token)
		}

		// 3. Start Node Liveness Watcher (15s threshold, 5s check interval)
		livenessWatcher := reconciler.NewNodeLivenessWatcher(store, 15*time.Second, 5*time.Second)
		go livenessWatcher.Start(ctx)

		// 4. Restore proxy listeners (Rule C2)
		proxyMgr := proxy.NewProxyManager(store, bindAddr)
		defer proxyMgr.Close()
		if err := proxyMgr.RestoreListeners(ctx); err != nil {
			log.Printf("Warning: restoring proxy listeners: %v", err)
		}

		// 5. API Server (token-aware)
		apiSrv := api.NewServer(store, token)
		apiHTTP := &http.Server{
			Addr:         bindAddr,
			Handler:      apiSrv,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
		go func() {
			if err := apiHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("API server: %v", err)
			}
		}()

		// 6. GUI Listener (stub; loopback only)
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
					log.Printf("GUI server: %v", err)
				}
			}()
		}

		// 7. Exactly 3 required startup log lines
		fmt.Printf("API listening on %s\n", bindAddr)
		fmt.Printf("GUI listening on %s\n", guiBindAddr)
		fmt.Printf("Advertise address: %s\n", advAddr)

		// 8. Wait for shutdown
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if guiHTTP != nil {
			_ = guiHTTP.Shutdown(shutdownCtx)
		}
		_ = apiHTTP.Shutdown(shutdownCtx)
		return nil
	},
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	daemonCmd.Flags().StringVar(&daemonBindAddress, "bind-address", "0.0.0.0:9001", "API server bind address")
	daemonCmd.Flags().StringVar(&daemonAdvertiseAddress, "advertise-address", "", "Advertised address for cluster nodes")
	daemonCmd.Flags().BoolVar(&daemonNoGUI, "no-gui", false, "Disable embedded web GUI")
	daemonCmd.Flags().StringVar(&daemonDataDir, "data-dir", "", "Data directory for SQLite database")
	daemonCmd.Flags().StringVar(&daemonConfig, "config", "", "Path to config file (default: ~/.orca/config.yaml)")
	rootCmd.AddCommand(daemonCmd)
}
