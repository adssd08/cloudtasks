package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/platform"
	"example.com/cloudtasks/internal/services"
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL must be configured")
	}

	auth, err := identity.NewDevelopmentAuth(
		os.Getenv("APP_ENV"),
		os.Getenv("DEV_ALICE_TOKEN"),
		os.Getenv("DEV_BOB_TOKEN"),
	)
	if err != nil {
		return err
	}

	startupCtx, cancelStartup := context.WithTimeout(
		signalCtx,
		10*time.Second,
	)

	clients, err := platform.NewAWSClients(startupCtx)

	if err != nil {
		cancelStartup()
		return err
	}

	db, err := platform.NewPostgres(startupCtx, databaseURL)
	cancelStartup()

	if err != nil {
		return err
	}
	defer db.Close()

	store := storage.NewS3Store(clients.S3, bucket)
	presigner := storage.NewS3Presigner(clients.S3, bucket)
	repository := storage.NewAttachmentRepository(db)

	attachmentService := services.NewAttachmentService(repository, store, presigner)

	app := platform.NewHTTPApp(attachmentService, auth)

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
