package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"upload-service/config"
)

func Run(logger *slog.Logger, cfg *config.Config) {
	container, err := NewContainer(logger, cfg)
	if err != nil {
		logger.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = container.DB.Close()
		container.RabbitConsumer.Close()
		container.RabbitPublisher.Close()
	}()

	consumerCtx, stopConsumer := context.WithCancel(context.Background())
	defer stopConsumer()
	consumerErrors := make(chan error, 1)
	go func() {
		if err := container.RabbitConsumer.Consume(consumerCtx); err != nil && consumerCtx.Err() == nil {
			consumerErrors <- err
		}
	}()

	router := container.Router()
	server := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			container.Logger.Error("Server ListenAndServe error", "error", err)
		}
	}()

	container.Logger.Info("Server started", "addr", cfg.Server.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		container.Logger.Info("Shutdown signal received")
	case err := <-consumerErrors:
		container.Logger.Error("lifecycle event consumer stopped; shutting down for restart", "error", err)
	}

	stopConsumer()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		container.Logger.Warn("Server forced to shutdown", "error", err)
	} else {
		container.Logger.Info("Server stopped gracefully")
	}
}
