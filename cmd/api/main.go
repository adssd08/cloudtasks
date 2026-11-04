package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/cloudtasks/internal/platform"
	"example.com/cloudtasks/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return fmt.Errorf("S3_BUCKET must be configured")
	}

	startupCtx, cancelStartup := context.WithTimeout(
		signalCtx,
		10*time.Second,
	)

	clients, err := platform.NewAWSClients(startupCtx)
	cancelStartup()

	if err != nil {
		return err
	}

	store := storage.NewS3Store(clients.S3, bucket)
	app := platform.NewHTTPApp(store)

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- app.Listen("127.0.0.1:8080")
	}()

	select {
	case err := <-serverErr:
		return err
	case <-signalCtx.Done():
		log.Print("shutdown requested")
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown API: %w", err)
	}

	log.Print("API stopped cleanly")
	return nil
}
