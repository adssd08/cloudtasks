package platform

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type AWSClients struct {
	S3  *s3.Client
	SQS *sqs.Client
}

func NewAWSClients(ctx context.Context) (*AWSClients, error) {
	cfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}

	if cfg.Region == "" {
		return nil, fmt.Errorf("AWS region must be configured")
	}

	endpoint := os.Getenv("AWS_ENDPOINT_URL")

	s3Client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
			options.UsePathStyle = true
		}
	})

	sqsClient := sqs.NewFromConfig(cfg, func(options *sqs.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &AWSClients{
		S3:  s3Client,
		SQS: sqsClient,
	}, nil
}
