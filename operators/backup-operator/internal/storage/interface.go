/*
Copyright 2025 hepj1999@gmail.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// BackupInfo contains metadata about a stored backup
type BackupInfo struct {
	// Name/key of the backup
	Name string
	// Full path/location
	Path string
	// Size in bytes
	Size int64
	// Last modified time
	ModifiedTime time.Time
	// Additional metadata
	Metadata map[string]string
}

// Backend defines the interface for different storage backends (S3, NFS, GCS, etc.)
type Backend interface {
	// Upload uploads data to the storage backend
	Upload(ctx context.Context, data io.Reader, path string, metadata map[string]string) error

	// Download downloads data from the storage backend
	Download(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete deletes a backup from the storage backend
	Delete(ctx context.Context, path string) error

	// List lists all backups with the given prefix
	List(ctx context.Context, prefix string) ([]BackupInfo, error)

	// Exists checks if a backup exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetMetadata retrieves metadata for a backup
	GetMetadata(ctx context.Context, path string) (map[string]string, error)
}

// Config contains common configuration for storage backends
type Config struct {
	// Type of backend: s3, nfs, gcs, azure
	Type string
	// Endpoint URL
	Endpoint string
	// Bucket or container name
	Bucket string
	// Path prefix
	Prefix string
	// Credentials
	AccessKey string
	SecretKey string
	// Region (for S3)
	Region string
	// Storage class (for S3: STANDARD, GLACIER, DEEP_ARCHIVE)
	StorageClass string
}

// NewBackend creates a new storage backend based on the config
func NewBackend(config *Config) (Backend, error) {
	switch config.Type {
	case "s3":
		return NewS3Backend(config)
	case "nfs":
		return NewNFSBackend(config)
	// case "gcs":
	// 	return NewGCSBackend(config)
	// case "azure":
	// 	return NewAzureBackend(config)
	default:
		return nil, fmt.Errorf("unsupported storage backend type: %s", config.Type)
	}
}
