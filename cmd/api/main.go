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

	app := platform.NewHTTPApp()

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
