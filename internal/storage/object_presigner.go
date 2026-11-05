package storage

import (
	"context"
	"time"
)

type PresignedRequest struct {
	URL     string              `json:"url"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"headers"`
}

type ObjectPresigner interface {
	PresignUpload(ctx context.Context, key string, ttl time.Duration) (PresignedRequest, error)
	PresignDownload(ctx context.Context, key string, ttl time.Duration) (PresignedRequest, error)
}
