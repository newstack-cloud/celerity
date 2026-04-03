package seed

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOProvisioner provisions MinIO buckets.
type MinIOProvisioner struct {
	client *minio.Client
	logger *zap.Logger
}

// NewMinIOProvisioner creates a provisioner targeting a MinIO endpoint.
func NewMinIOProvisioner(endpoint string, accessKey string, secretKey string, logger *zap.Logger) (*MinIOProvisioner, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("creating MinIO client: %w", err)
	}

	return &MinIOProvisioner{
		client: client,
		logger: logger,
	}, nil
}

func (p *MinIOProvisioner) ProvisionBucket(ctx context.Context, bucketName string) error {
	exists, err := p.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("checking bucket %s: %w", bucketName, err)
	}

	if exists {
		p.logger.Debug("bucket already exists, skipping", zap.String("bucket", bucketName))
		return nil
	}

	if err := p.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("creating bucket %s: %w", bucketName, err)
	}

	p.logger.Debug("bucket created", zap.String("bucket", bucketName))
	return nil
}

// MinIOUploader implements StorageUploader for MinIO.
type MinIOUploader struct {
	client *minio.Client
	logger *zap.Logger
}

// NewMinIOUploader creates an uploader targeting a MinIO endpoint.
func NewMinIOUploader(endpoint string, accessKey string, secretKey string, logger *zap.Logger) (*MinIOUploader, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("creating MinIO client: %w", err)
	}

	return &MinIOUploader{
		client: client,
		logger: logger,
	}, nil
}

func (u *MinIOUploader) Upload(
	ctx context.Context,
	bucketName string,
	objectKey string,
	data []byte,
) error {
	contentType := DetectContentType(objectKey, data)

	_, err := u.client.PutObject(
		ctx,
		bucketName,
		objectKey,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return fmt.Errorf("uploading %s to bucket %s: %w", objectKey, bucketName, err)
	}

	return nil
}

// DetectContentType determines the MIME type for an object by extension,
// falling back to http.DetectContentType for unrecognised extensions.
func DetectContentType(objectKey string, data []byte) string {
	// Try extension-based detection first for common types.
	ext := filepath.Ext(objectKey)
	switch ext {
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".txt":
		return "text/plain"
	}

	// Fall back to content sniffing.
	return http.DetectContentType(data)
}
