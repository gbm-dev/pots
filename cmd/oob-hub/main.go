package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gbm-dev/pots/internal/auth"
	"github.com/gbm-dev/pots/internal/config"
	"github.com/gbm-dev/pots/internal/modem"
	"github.com/gbm-dev/pots/internal/sshserver"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := config.LoadFromEnv()

	// Init user store
	store, err := auth.NewFileStore(cfg.UserDataDir)
	if err != nil {
		log.Fatalf("initializing user store: %v", err)
	}

	// Run legacy migration if old files exist
	if err := auth.MigrateFromLegacy(cfg.UserDataDir, store); err != nil {
		log.Printf("warning: legacy migration failed: %v", err)
	}

	// Parse site config
	sites, err := config.ParseSitesFile(cfg.SitesPath)
	if err != nil {
		log.Fatalf("loading sites config: %v", err)
	}
	log.Printf("loaded %d sites from %s", len(sites), cfg.SitesPath)

	// Create modem pool
	pool := modem.NewPool(cfg.ModemCount)
	free, total := pool.Available()
	log.Printf("modem pool: %d/%d devices available", free, total)

	// Start SSH server
	srv, err := sshserver.New(cfg, store, pool, sites)
	if err != nil {
		log.Fatalf("creating SSH server: %v", err)
	}

	// Signal handling for graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("shutdown complete")
}
