package blobstore

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type BlobStoreFS struct {
	root string
}

func NewBlobStoreFS(root string) *BlobStoreFS {
	return &BlobStoreFS{root: root}
}

func (b *BlobStoreFS) Has(ctx context.Context, sha string) (bool, error) {
	path := filepath.Join(b.root, sha)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (b *BlobStoreFS) Put(ctx context.Context, sha string, r io.Reader, size int64) error {
	path := filepath.Join(b.root, sha)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.CopyN(f, r, size)
	return err
}

func (b *BlobStoreFS) Open(ctx context.Context, sha string) (io.ReadCloser, int64, error) {
	path := filepath.Join(b.root, sha)
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}
