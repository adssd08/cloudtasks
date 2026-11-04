package storage

import (
	"context"
	"io"
)

type ObjectStore interface {
	Put(
		ctx context.Context,
		key string,
		body io.ReadSeeker,
		contentType string,
	) error

	Get(ctx context.Context, key string) (io.ReadCloser, error)

	Delete(ctx context.Context, key string) error
}
