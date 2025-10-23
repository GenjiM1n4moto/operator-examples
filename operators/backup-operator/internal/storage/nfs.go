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
)

// NFSBackend implements the Backend interface for NFS storage
type NFSBackend struct {
	config *Config
	// In production, add NFS mount path and client
	mountPath string
}

// NewNFSBackend creates a new NFS storage backend
func NewNFSBackend(config *Config) (Backend, error) {
	// In production:
	// 1. Parse NFS server and export path from endpoint
	// 2. Mount NFS share (or assume it's already mounted)
	// 3. Verify mount is accessible

	return &NFSBackend{
		config:    config,
		mountPath: "/mnt/nfs", // Default mount path
	}, nil
}

// Upload uploads data to NFS
func (n *NFSBackend) Upload(ctx context.Context, data io.Reader, path string, metadata map[string]string) error {
	// In production:
	// 1. Create target directory if needed
	// 2. Write data to file on NFS mount
	// 3. Store metadata in a sidecar file (e.g., .metadata.json)

	fullPath := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, path)
	fmt.Printf("NFSBackend: Would upload to %s (simulated)\n", fullPath)

	return nil
}

// Download downloads data from NFS
func (n *NFSBackend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	// In production:
	// 1. Open file from NFS mount
	// 2. Return file handle as io.ReadCloser

	fullPath := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, path)
	fmt.Printf("NFSBackend: Would download from %s (simulated)\n", fullPath)

	return nil, fmt.Errorf("download not yet implemented")
}

// Delete deletes a file from NFS
func (n *NFSBackend) Delete(ctx context.Context, path string) error {
	// In production:
	// 1. Delete file from NFS mount
	// 2. Delete metadata file if exists

	fullPath := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, path)
	fmt.Printf("NFSBackend: Would delete %s (simulated)\n", fullPath)

	return nil
}

// List lists all files with the given prefix
func (n *NFSBackend) List(ctx context.Context, prefix string) ([]BackupInfo, error) {
	// In production:
	// 1. List files in directory matching prefix
	// 2. Read metadata from sidecar files
	// 3. Return BackupInfo slice

	fullPrefix := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, prefix)
	fmt.Printf("NFSBackend: Would list files with prefix %s (simulated)\n", fullPrefix)

	return []BackupInfo{}, nil
}

// Exists checks if a file exists on NFS
func (n *NFSBackend) Exists(ctx context.Context, path string) (bool, error) {
	// In production:
	// Use os.Stat to check if file exists

	fullPath := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, path)
	fmt.Printf("NFSBackend: Would check existence of %s (simulated)\n", fullPath)

	return false, nil
}

// GetMetadata retrieves metadata for a file
func (n *NFSBackend) GetMetadata(ctx context.Context, path string) (map[string]string, error) {
	// In production:
	// Read metadata from sidecar file (.metadata.json)

	fullPath := fmt.Sprintf("%s/%s/%s", n.mountPath, n.config.Prefix, path)
	fmt.Printf("NFSBackend: Would get metadata for %s (simulated)\n", fullPath)

	return map[string]string{}, nil
}
