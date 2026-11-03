package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"example.com/cloudtasks/internal/platform"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	clients, err := platform.NewAWSClients(ctx)
	if err != nil {
		return err
	}

	output, err := clients.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("list S3 buckets: %w", err)
	}

	fmt.Printf("S3 connection succeeded; buckets returned: %d\n", len(output.Buckets))

	for _, bucket := range output.Buckets {
		fmt.Println(aws.ToString(bucket.Name))
	}

	return nil
}
