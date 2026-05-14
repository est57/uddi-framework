// UDDI API Gateway
// REST + gRPC gateway for the UDDI protocol
// Handles DID registration, resolution, credential issuance, and ZKP verification

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uddi-protocol/uddi/api/internal/blockchain"
	"github.com/uddi-protocol/uddi/api/internal/config"
	"github.com/uddi-protocol/uddi/api/internal/server"
	"github.com/uddi-protocol/uddi/api/internal/zkp"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// ── Dependencies ──────────────────────────────────────────────────────────
	var chainClient *blockchain.Client
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		store, err := blockchain.NewPostgresDIDStore(ctx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			logger.Error("failed to connect to DID database", "error", err)
			os.Exit(1)
		}
		defer store.Close()
		chainClient = blockchain.NewClientWithStore(cfg.BlockchainRPC, store)
		logger.Info("DID registry using Postgres store")
	} else {
		chainClient, err = blockchain.NewClient(cfg.BlockchainRPC)
		if err != nil {
			logger.Error("failed to initialize DID registry", "error", err)
			os.Exit(1)
		}
		logger.Info("DID registry using in-memory store")
	}

	zkpService := zkp.NewService(cfg.ZKPServiceURL)
	if cfg.RedisURL != "" {
		logger.Info("auth challenges using Redis store")
	} else {
		logger.Info("auth challenges using in-memory store")
	}

	// ── Router ────────────────────────────────────────────────────────────────
	r, err := server.NewRouter(cfg, chainClient, zkpService)
	if err != nil {
		logger.Error("failed to initialize router", "error", err)
		os.Exit(1)
	}

	// ── Server ────────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("UDDI API Gateway starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
	logger.Info("Server stopped")
}
