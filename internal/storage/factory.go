package storage

import (
	"fmt"
)

// NewStore membuat dan mengembalikan Store instance berdasarkan konfigurasi
func NewStore(cfg *Config) (Store, error) {
	switch cfg.Driver {
	case "local":
		return NewLocalStore(cfg.Root, cfg.PublicURL)
	case "s3":
		// TODO: implementasi S3Store dengan AWS SDK atau BeyondStorage S3 driver
		return nil, fmt.Errorf("s3 driver not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Driver)
	}
}
