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
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Backend implements the Backend interface for S3-compatible storage
type S3Backend struct {
	config   *Config
	client   *s3.Client
	uploader *manager.Uploader
}

// NewS3Backend creates a new S3 storage backend
func NewS3Backend(cfg *Config) (Backend, error) {
	ctx := context.Background()

	// Build AWS config options
	var configOpts []func(*config.LoadOptions) error

	// Set region if specified
	if cfg.Region != "" {
		configOpts = append(configOpts, config.WithRegion(cfg.Region))
	}

	// Set credentials if provided
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		configOpts = append(configOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	// Load AWS SDK config
	awsConfig, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build S3 client options
	var s3Opts []func(*s3.Options)

	// Set custom endpoint for MinIO, Ceph, etc.
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			// Force path-style addressing for MinIO compatibility
			o.UsePathStyle = true
		})
	}

	// Create S3 client
	client := s3.NewFromConfig(awsConfig, s3Opts...)

	// Create uploader for efficient uploads
	uploader := manager.NewUploader(client)

	// Note: Skip connection test during initialization to allow operator to run outside cluster
	// The actual connection will be tested when backup Jobs run inside the cluster

	return &S3Backend{
		config:   cfg,
		client:   client,
		uploader: uploader,
	}, nil
}

// Upload uploads data to S3
func (s *S3Backend) Upload(ctx context.Context, data io.Reader, path string, metadata map[string]string) error {
	// Build the full S3 key
	key := s.buildKey(path)

	// Convert metadata to S3 format
	s3Metadata := make(map[string]string)
	for k, v := range metadata {
		s3Metadata[k] = v
	}

	// Upload using the uploader (handles multipart automatically for large files)
	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(s.config.Bucket),
		Key:      aws.String(key),
		Body:     data,
		Metadata: s3Metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3://%s/%s: %w", s.config.Bucket, key, err)
	}

	return nil
}

// Download downloads data from S3
func (s *S3Backend) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	key := s.buildKey(path)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from s3://%s/%s: %w", s.config.Bucket, key, err)
	}

	return result.Body, nil
}

// Delete deletes an object from S3
func (s *S3Backend) Delete(ctx context.Context, path string) error {
	key := s.buildKey(path)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete s3://%s/%s: %w", s.config.Bucket, key, err)
	}

	return nil
}

// List lists all objects with the given prefix
func (s *S3Backend) List(ctx context.Context, prefix string) ([]BackupInfo, error) {
	fullPrefix := s.buildKey(prefix)

	var backups []BackupInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects with prefix %s: %w", fullPrefix, err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}

			// Get metadata for this object
			metadata, err := s.GetMetadata(ctx, strings.TrimPrefix(*obj.Key, s.config.Prefix+"/"))
			if err != nil {
				// Log error but continue listing
				metadata = make(map[string]string)
			}

			backups = append(backups, BackupInfo{
				Name:         filepath.Base(*obj.Key),
				Path:         *obj.Key,
				Size:         *obj.Size,
				ModifiedTime: *obj.LastModified,
				Metadata:     metadata,
			})
		}
	}

	return backups, nil
}

// Exists checks if an object exists in S3
func (s *S3Backend) Exists(ctx context.Context, path string) (bool, error) {
	key := s.buildKey(path)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		var notFound *types.NotFound
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &notFound) || errors.As(err, &noSuchKey) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence of s3://%s/%s: %w", s.config.Bucket, key, err)
	}

	return true, nil
}

// GetMetadata retrieves metadata for an object
func (s *S3Backend) GetMetadata(ctx context.Context, path string) (map[string]string, error) {
	key := s.buildKey(path)

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for s3://%s/%s: %w", s.config.Bucket, key, err)
	}

	// Copy metadata from S3 object
	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return metadata, nil
}

// buildKey builds the full S3 key from a relative path
func (s *S3Backend) buildKey(path string) string {
	if s.config.Prefix == "" {
		return path
	}
	return filepath.Join(s.config.Prefix, path)
}
