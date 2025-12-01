package blobstore

import (
	"context"
	"io"
)

type BlobStore interface {
	Has(ctx context.Context, sha string) (bool, error)
	Put(ctx context.Context, sha string, r io.Reader, size int64) error
	Open(ctx context.Context, sha string) (io.ReadCloser, int64, error)
}
