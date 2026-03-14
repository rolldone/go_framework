package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStore mengimplementasikan Store interface untuk local filesystem
type LocalStore struct {
	root      string // direktori storage
	publicURL string // base URL untuk akses publik
}

// NewLocalStore membuat instance LocalStore baru
func NewLocalStore(root, publicURL string) (*LocalStore, error) {
	// ensure direktori ada
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStore{
		root:      root,
		publicURL: publicURL,
	}, nil
}

// Put menyimpan file ke local filesystem
func (s *LocalStore) Put(ctx context.Context, key string, data io.Reader) error {
	fullPath := filepath.Join(s.root, key)

	// ensure parent directory ada
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// create/overwrite file
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// copy file content
	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get mengambil file dari local filesystem sebagai ReadCloser
func (s *LocalStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.root, key)

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

// Delete menghapus file dari local filesystem
func (s *LocalStore) Delete(ctx context.Context, key string) error {
	fullPath := filepath.Join(s.root, key)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", key)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Stat mengambil metadata file
func (s *LocalStore) Stat(ctx context.Context, key string) (*FileMeta, error) {
	fullPath := filepath.Join(s.root, key)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &FileMeta{
		Key:       key,
		Size:      info.Size(),
		CreatedAt: info.ModTime().Unix(),
	}, nil
}

// List menampilkan semua keys dengan prefix tertentu
func (s *LocalStore) List(ctx context.Context, prefix string) ([]string, error) {
	prefixPath := filepath.Join(s.root, prefix)
	var keys []string

	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// calculate relative path sebagai key
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}

		// gunakan forward slashes untuk consistency
		key := filepath.ToSlash(rel)

		// filter by prefix jika ada
		if prefix == "" || (len(path) >= len(prefixPath) && path[:len(prefixPath)] == prefixPath) {
			keys = append(keys, key)
		}

		return nil
	})

	return keys, err
}

// PublicURL mengembalikan URL publik untuk akses file
// Untuk local storage, ini akan menjadi path relatif dari base publicURL
func (s *LocalStore) PublicURL(ctx context.Context, key string) (string, error) {
	// return base publicURL + key
	return s.publicURL + "/" + key, nil
}

// Close menutup koneksi (untuk local storage tidak ada yang perlu ditutup)
func (s *LocalStore) Close() error {
	return nil
}

// GetRoot mengembalikan direktori root storage (untuk static serving)
func (s *LocalStore) GetRoot() string {
	return s.root
}
