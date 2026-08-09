package storage

import (
	"context"
	"io"
)

type VideoStore interface {
	Save(ctx context.Context, key string, source io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
