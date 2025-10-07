package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
)

type S3Storage struct {
	client     *s3.Client
	bucket     string
	region     string
	cdnBaseURL string
}

type S3Config struct {
	AccessKey  string
	SecretKey  string
	Region     string
	Bucket     string
	Endpoint   string // Optional for S3-compatible services
	CDNBaseURL string
}

func NewS3Storage(cfg S3Config) (domain.Storage, error) {
	// Create AWS config
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	s3Options := func(o *s3.Options) {
		o.Region = cfg.Region
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for MinIO and other S3-compatible services
		}
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, s3Options)

	return &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		region:     cfg.Region,
		cdnBaseURL: strings.TrimRight(cfg.CDNBaseURL, "/"),
	}, nil
}

// Upload uploads a file to S3 with deduplication by SHA256
func (s *S3Storage) Upload(ctx context.Context, key string, r io.Reader, contentType string, metadata map[string]string) (*domain.StorageResult, error) {
	// Read content to calculate SHA256 and prepare for upload
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(content)
	sha256Hash := hex.EncodeToString(hash[:])

	// Add SHA256 to metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["sha256"] = sha256Hash

	// Prepare the upload input
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}

	// Upload to S3
	_, err = s.client.PutObject(ctx, putInput)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate URLs
	cdnURL := ""
	if s.cdnBaseURL != "" {
		cdnURL = fmt.Sprintf("%s/%s", s.cdnBaseURL, key)
	}

	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)

	return &domain.StorageResult{
		Key:        key,
		SHA256:     sha256Hash,
		Size:       int64(len(content)),
		S3URL:      s3URL,
		CDNURL:     cdnURL,
		UploadedAt: time.Now(),
	}, nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.GetObject(ctx, getInput)
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	return result.Body, nil
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.HeadObject(ctx, headInput)
	if err != nil {
		// Check if the error is because the object doesn't exist
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check S3 object: %w", err)
	}

	return true, nil
}

// Delete deletes a file from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetSignedURL generates a presigned URL for downloading
func (s *S3Storage) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	getInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	presignResult, err := presignClient.PresignGetObject(ctx, getInput, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignResult.URL, nil
}

// ListByPrefix lists all objects with a given prefix
func (s *S3Storage) ListByPrefix(ctx context.Context, prefix string, maxKeys int32) ([]string, error) {
	listInput := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: &maxKeys,
	}

	result, err := s.client.ListObjectsV2(ctx, listInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 objects: %w", err)
	}

	var keys []string
	for _, obj := range result.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}

// GetMetadata retrieves metadata for an S3 object
func (s *S3Storage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.HeadObject(ctx, headInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object metadata: %w", err)
	}

	return result.Metadata, nil
}

// CheckSHA256 checks if an object with the given SHA256 exists
func (s *S3Storage) CheckSHA256(ctx context.Context, sha256Hash string) (string, bool, error) {
	// List objects with SHA256 in metadata
	// Note: This is not efficient for large buckets. Consider using a database index instead.
	prefix := "media/" // Assuming all media files are under this prefix

	var continuationToken *string
	for {
		listInput := &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		}

		result, err := s.client.ListObjectsV2(ctx, listInput)
		if err != nil {
			return "", false, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		// Check each object's metadata
		for _, obj := range result.Contents {
			if obj.Key == nil {
				continue
			}

			metadata, err := s.GetMetadata(ctx, *obj.Key)
			if err != nil {
				continue // Skip objects we can't read metadata for
			}

			if hash, ok := metadata["sha256"]; ok && hash == sha256Hash {
				return *obj.Key, true, nil
			}
		}

		// Check if there are more results
		if result.IsTruncated != nil && *result.IsTruncated {
			continuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return "", false, nil
}

// GenerateThumbnailKey generates a key for thumbnail storage
func (s *S3Storage) GenerateThumbnailKey(originalKey string, size string) string {
	dir := path.Dir(originalKey)
	filename := path.Base(originalKey)
	ext := path.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	return fmt.Sprintf("%s/thumbnails/%s_%s%s", dir, nameWithoutExt, size, ext)
}

// UploadThumbnail uploads a thumbnail to S3
func (s *S3Storage) UploadThumbnail(ctx context.Context, originalKey string, size string, r io.Reader, contentType string) (string, error) {
	thumbnailKey := s.GenerateThumbnailKey(originalKey, size)

	content, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to read thumbnail content: %w", err)
	}

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(thumbnailKey),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"original_key": originalKey,
			"size":         size,
		},
		CacheControl: aws.String("public, max-age=31536000"), // Cache for 1 year
	}

	_, err = s.client.PutObject(ctx, putInput)
	if err != nil {
		return "", fmt.Errorf("failed to upload thumbnail to S3: %w", err)
	}

	return thumbnailKey, nil
}

// SetCORS sets CORS configuration for the bucket (useful for browser uploads)
func (s *S3Storage) SetCORS(ctx context.Context) error {
	corsRules := []types.CORSRule{
		{
			AllowedHeaders: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "POST", "DELETE", "HEAD"},
			AllowedOrigins: []string{"*"}, // In production, specify actual origins
			ExposeHeaders:  []string{"ETag"},
			MaxAgeSeconds:  aws.Int32(3000),
		},
	}

	corsInput := &s3.PutBucketCorsInput{
		Bucket: aws.String(s.bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: corsRules,
		},
	}

	_, err := s.client.PutBucketCors(ctx, corsInput)
	if err != nil {
		return fmt.Errorf("failed to set CORS configuration: %w", err)
	}

	return nil
}

// GetCDNURL returns the CDN URL for a given key
func (s *S3Storage) GetCDNURL(key string) string {
	if s.cdnBaseURL == "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	}
	return fmt.Sprintf("%s/%s", s.cdnBaseURL, key)
}
