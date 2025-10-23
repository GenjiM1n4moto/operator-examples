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
	"os"
	"strings"
	"testing"
)

// TestMinIOCompatibility tests S3 backend compatibility with MinIO
func TestMinIOCompatibility(t *testing.T) {
	// Skip if not running in cluster or MinIO not available
	if os.Getenv("SKIP_MINIO_TEST") != "" {
		t.Skip("Skipping MinIO compatibility test")
	}
	if os.Getenv("RUN_MINIO_COMPAT") == "" {
		t.Skip("Set RUN_MINIO_COMPAT=1 to run MinIO compatibility tests")
	}

	// MinIO configuration
	config := &Config{
		Type:      "s3",
		Endpoint:  "http://minio.minio.svc.cluster.local:9000",
		Bucket:    "test-s3-compat",
		Prefix:    "test",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin123",
		Region:    "us-east-1", // MinIO default region
	}

	// Create S3 backend
	backend, err := NewS3Backend(config)
	if err != nil {
		t.Fatalf("Failed to create S3 backend: %v", err)
	}

	ctx := context.Background()

	// Test 1: Upload
	t.Run("Upload", func(t *testing.T) {
		testData := strings.NewReader("Hello MinIO from S3 backend!")
		metadata := map[string]string{
			"test-key": "test-value",
			"source":   "s3-minio-test",
		}

		err := backend.Upload(ctx, testData, "test-upload.txt", metadata)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
		t.Logf("✅ Upload successful")
	})

	// Test 2: Exists
	t.Run("Exists", func(t *testing.T) {
		exists, err := backend.Exists(ctx, "test-upload.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}
		if !exists {
			t.Fatalf("File should exist but doesn't")
		}
		t.Logf("✅ Exists check successful")
	})

	// Test 3: GetMetadata
	t.Run("GetMetadata", func(t *testing.T) {
		metadata, err := backend.GetMetadata(ctx, "test-upload.txt")
		if err != nil {
			t.Fatalf("GetMetadata failed: %v", err)
		}
		if metadata["test-key"] != "test-value" {
			t.Fatalf("Expected metadata 'test-key'='test-value', got '%v'", metadata["test-key"])
		}
		t.Logf("✅ GetMetadata successful: %v", metadata)
	})

	// Test 4: Download
	t.Run("Download", func(t *testing.T) {
		reader, err := backend.Download(ctx, "test-upload.txt")
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read downloaded data: %v", err)
		}

		expectedData := "Hello MinIO from S3 backend!"
		if string(data) != expectedData {
			t.Fatalf("Downloaded data mismatch. Expected '%s', got '%s'", expectedData, string(data))
		}
		t.Logf("✅ Download successful: %s", string(data))
	})

	// Test 5: Upload multiple files for List test
	t.Run("UploadMultiple", func(t *testing.T) {
		for i := 1; i <= 3; i++ {
			data := strings.NewReader(fmt.Sprintf("File %d content", i))
			err := backend.Upload(ctx, data, fmt.Sprintf("file-%d.txt", i), nil)
			if err != nil {
				t.Fatalf("Failed to upload file-%d.txt: %v", i, err)
			}
		}
		t.Logf("✅ Multiple uploads successful")
	})

	// Test 6: List
	t.Run("List", func(t *testing.T) {
		backups, err := backend.List(ctx, "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(backups) < 4 { // test-upload.txt + file-1.txt, file-2.txt, file-3.txt
			t.Fatalf("Expected at least 4 files, got %d", len(backups))
		}

		t.Logf("✅ List successful, found %d files:", len(backups))
		for _, backup := range backups {
			t.Logf("  - %s (size: %d, modified: %v)", backup.Name, backup.Size, backup.ModifiedTime)
		}
	})

	// Test 7: Delete
	t.Run("Delete", func(t *testing.T) {
		// Delete all test files
		filesToDelete := []string{"test-upload.txt", "file-1.txt", "file-2.txt", "file-3.txt"}
		for _, file := range filesToDelete {
			err := backend.Delete(ctx, file)
			if err != nil {
				t.Fatalf("Failed to delete %s: %v", file, err)
			}
		}

		// Verify deletion
		exists, err := backend.Exists(ctx, "test-upload.txt")
		if err != nil {
			t.Fatalf("Exists check after delete failed: %v", err)
		}
		if exists {
			t.Fatalf("File should not exist after deletion")
		}
		t.Logf("✅ Delete successful")
	})

	t.Logf("\n🎉 All MinIO compatibility tests passed!")
}
