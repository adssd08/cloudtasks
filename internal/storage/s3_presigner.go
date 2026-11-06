package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Presigner struct {
	client *s3.PresignClient
	bucket string
}

var _ ObjectPresigner = (*S3Presigner)(nil)

func NewS3Presigner(client *s3.Client, bucket string) *S3Presigner {
	return &S3Presigner{client: s3.NewPresignClient(client), bucket: bucket}
}

func (s *S3Presigner) PresignUpload(ctx context.Context, key string, ttl time.Duration) (PresignedRequest, error) {

	request, err := s.client.PresignPutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			IfNoneMatch: aws.String("*"),
		},
		func(options *s3.PresignOptions) {
			options.Expires = ttl
		},
	)

	if err != nil {
		return PresignedRequest{}, fmt.Errorf("presign upload: %w", err)
	}

	return PresignedRequest{
		URL:     request.URL,
		Method:  request.Method,
		Headers: request.SignedHeader,
	}, nil
}

func (s *S3Presigner) PresignDownload(ctx context.Context, key string, ttl time.Duration) (PresignedRequest, error) {
	request, err := s.client.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
		func(options *s3.PresignOptions) {
			options.Expires = ttl
		},
	)

	if err != nil {
		return PresignedRequest{}, fmt.Errorf("presign download: %w", err)
	}

	return PresignedRequest{
		URL:     request.URL,
		Method:  request.Method,
		Headers: request.SignedHeader,
	}, nil
}
