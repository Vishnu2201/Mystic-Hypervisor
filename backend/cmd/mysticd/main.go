package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mystic-hypervisor/mystic/backend/internal/api"
	"github.com/mystic-hypervisor/mystic/backend/internal/config"
	"github.com/mystic-hypervisor/mystic/backend/internal/logging"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/incus"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/interfaces"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/kvm"
	"github.com/mystic-hypervisor/mystic/backend/internal/providers/lxc"
)

func main() {
	logger := logging.GetLogger()
	logger.Info("Starting Mystic Hypervisor daemon (mysticd)", "version", "0.1.0-foundation")

	// 1. Load Configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Register Non-Destructive Provider Stubs for Milestone 1
	_ = interfaces.RegisterProvider("incus", incus.NewIncusProvider(cfg.Provider.IncusSocket))
	_ = interfaces.RegisterProvider("lxc", lxc.NewLXCProvider(cfg.Provider.LXCSocket))
	_ = interfaces.RegisterProvider("kvm", kvm.NewKVMProvider(cfg.Provider.KVMPath))

	logger.Info("Registered virtualization providers",
		"registered", interfaces.ListProviders(),
		"default", cfg.Provider.DefaultProvider,
	)

	// 3. Initialize Router & Server
	router := api.NewRouter(cfg)
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Handle Signal Context for Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Mystic Hypervisor daemon listening", "addr", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutdown signal received, initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	_ = router.Close()
	logger.Info("Mystic Hypervisor daemon stopped cleanly.")
}
