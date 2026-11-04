package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"example.com/cloudtasks/internal/platform"
	"example.com/cloudtasks/internal/storage"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return fmt.Errorf("S3_BUCKET must be configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clients, err := platform.NewAWSClients(ctx)
	if err != nil {
		return err
	}

	store := storage.NewS3Store(clients.S3, bucket)
	key := "checks/" + uuid.NewString()
	expected := []byte{0, 1, 2, 127, 128, 255}

	if err := store.Put(ctx, key, bytes.NewReader(expected), "application/octet-stream"); err != nil {
		return err
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := store.Delete(cleanupCtx, key); err != nil {
			log.Printf("cleanup object %q: %v", key, err)
		}
	}()

	body, err := store.Get(ctx, key)
	if err != nil {
		return err
	}

	actual, readErr := io.ReadAll(body)
	closeErr := body.Close()

	if readErr != nil {
		return fmt.Errorf("read downloaded object: %w", readErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close downloaded object: %w", closeErr)
	}

	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("downloaded bytes do not match uploaded bytes")
	}

	if err := store.Delete(ctx, key); err != nil {
		return err
	}

	fmt.Println("binary upload, read-back, and delete succeeded")
	return nil
}
