package storage

import (
	"fmt"
	"os"
)

// Config holds konfigurasi storage dari environment
type Config struct {
	Driver      string // "local" atau "s3"
	Root        string // untuk local: path ke direktori penyimpanan
	PublicURL   string // base URL publik; contoh: "http://localhost:8080/assets" atau "https://cdn.example.com"
	S3Bucket    string // untuk S3
	S3Region    string // untuk S3
	S3Endpoint  string // untuk S3 (optional, untuk MinIO)
	S3AccessKey string // untuk S3
	S3SecretKey string // untuk S3
}

// LoadConfig membaca konfigurasi storage dari environment
func LoadConfig() (*Config, error) {
	driver := os.Getenv("STORAGE_DRIVER")
	if driver == "" {
		driver = "local"
	}

	if driver != "local" && driver != "s3" {
		return nil, fmt.Errorf("unsupported STORAGE_DRIVER: %s (must be 'local' or 's3')", driver)
	}

	cfg := &Config{
		Driver:    driver,
		PublicURL: os.Getenv("STORAGE_PUBLIC_URL"),
	}

	// load driver-specific config
	if driver == "local" {
		root := os.Getenv("STORAGE_ROOT")
		if root == "" {
			root = "./storage" // default
		}
		cfg.Root = root
	} else if driver == "s3" {
		cfg.S3Bucket = os.Getenv("S3_BUCKET")
		cfg.S3Region = os.Getenv("S3_REGION")
		cfg.S3Endpoint = os.Getenv("S3_ENDPOINT")
		cfg.S3AccessKey = os.Getenv("S3_ACCESS_KEY")
		cfg.S3SecretKey = os.Getenv("S3_SECRET_KEY")

		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3_BUCKET is required when STORAGE_DRIVER=s3")
		}
		if cfg.S3Region == "" {
			return nil, fmt.Errorf("S3_REGION is required when STORAGE_DRIVER=s3")
		}
	}

	if cfg.PublicURL == "" {
		if driver == "local" {
			cfg.PublicURL = "http://localhost:8080/assets"
		} else {
			cfg.PublicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", cfg.S3Bucket, cfg.S3Region)
		}
	}

	return cfg, nil
}
