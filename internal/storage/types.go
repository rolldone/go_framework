package storage

import (
	"context"
	"io"
)

// FileMeta holds metadata tentang file yang disimpan
type FileMeta struct {
	Key       string // unique key/path di storage
	Size      int64  // bytes
	MimeType  string // content type
	CreatedAt int64  // unix timestamp
}

// Store adalah interface abstraksi storage (lokal atau cloud)
type Store interface {
	// Put menyimpan file dengan content dari reader
	Put(ctx context.Context, key string, data io.Reader) error

	// Get mengambil file dari storage
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete menghapus file dari storage
	Delete(ctx context.Context, key string) error

	// Stat mengambil metadata file
	Stat(ctx context.Context, key string) (*FileMeta, error)

	// List menampilkan files dengan prefix (untuk cleanup/listing)
	List(ctx context.Context, prefix string) ([]string, error)

	// PublicURL mengembalikan public/signed URL untuk akses file
	// Untuk local storage, bisa return relative path; untuk S3 bisa presigned URL
	PublicURL(ctx context.Context, key string) (string, error)

	// Close menutup koneksi storage jika perlu
	Close() error
}
