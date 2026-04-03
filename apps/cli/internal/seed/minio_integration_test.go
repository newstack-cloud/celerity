package seed

import (
	"context"
	"testing"

	minioclient "github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MinIOIntegrationSuite struct {
	suite.Suite
	endpoint  string
	accessKey string
	secretKey string
	logger    *zap.Logger
}

func TestMinIOIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MinIOIntegrationSuite))
}

func (s *MinIOIntegrationSuite) SetupTest() {
	s.endpoint = testutils.RequireEnv(s.T(), "CELERITY_TEST_MINIO_ENDPOINT")
	s.accessKey = testutils.RequireEnv(s.T(), "CELERITY_TEST_MINIO_ACCESS_KEY")
	s.secretKey = testutils.RequireEnv(s.T(), "CELERITY_TEST_MINIO_SECRET_KEY")
	logger, _ := zap.NewDevelopment()
	s.logger = logger

	// Strip http:// prefix for MinIO client (it takes host:port).
	s.endpoint = stripScheme(s.endpoint)
}

func stripScheme(endpoint string) string {
	if len(endpoint) > 7 && endpoint[:7] == "http://" {
		return endpoint[7:]
	}
	if len(endpoint) > 8 && endpoint[:8] == "https://" {
		return endpoint[8:]
	}
	return endpoint
}

func (s *MinIOIntegrationSuite) removeBucket(bucketName string) {
	client, err := minioclient.New(s.endpoint, &minioclient.Options{
		Creds:  miniocreds.NewStaticV4(s.accessKey, s.secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return
	}
	// Remove all objects first.
	for obj := range client.ListObjects(context.Background(), bucketName, minioclient.ListObjectsOptions{Recursive: true}) {
		_ = client.RemoveObject(context.Background(), bucketName, obj.Key, minioclient.RemoveObjectOptions{})
	}
	_ = client.RemoveBucket(context.Background(), bucketName)
}

func (s *MinIOIntegrationSuite) Test_provision_bucket() {
	bucketName := "integration-test-bucket"
	s.removeBucket(bucketName)

	provisioner, err := NewMinIOProvisioner(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)

	err = provisioner.ProvisionBucket(context.Background(), bucketName)
	s.Require().NoError(err)

	// Verify bucket exists.
	client, err := minioclient.New(s.endpoint, &minioclient.Options{
		Creds:  miniocreds.NewStaticV4(s.accessKey, s.secretKey, ""),
		Secure: false,
	})
	s.Require().NoError(err)
	exists, err := client.BucketExists(context.Background(), bucketName)
	s.Require().NoError(err)
	s.Assert().True(exists)
}

func (s *MinIOIntegrationSuite) Test_provision_bucket_idempotent() {
	bucketName := "integration-test-idempotent"
	s.removeBucket(bucketName)

	provisioner, err := NewMinIOProvisioner(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)

	s.Require().NoError(provisioner.ProvisionBucket(context.Background(), bucketName))
	s.Require().NoError(provisioner.ProvisionBucket(context.Background(), bucketName))
}

func (s *MinIOIntegrationSuite) Test_upload_file() {
	bucketName := "integration-test-upload"
	s.removeBucket(bucketName)

	provisioner, err := NewMinIOProvisioner(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)
	s.Require().NoError(provisioner.ProvisionBucket(context.Background(), bucketName))

	uploader, err := NewMinIOUploader(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)

	err = uploader.Upload(context.Background(), bucketName, "test.json", []byte(`{"key":"value"}`))
	s.Require().NoError(err)

	// Verify object exists by reading it back.
	client, err := minioclient.New(s.endpoint, &minioclient.Options{
		Creds:  miniocreds.NewStaticV4(s.accessKey, s.secretKey, ""),
		Secure: false,
	})
	s.Require().NoError(err)
	obj, err := client.GetObject(context.Background(), bucketName, "test.json", minioclient.GetObjectOptions{})
	s.Require().NoError(err)
	defer obj.Close()

	info, err := obj.Stat()
	s.Require().NoError(err)
	s.Assert().Equal("application/json", info.ContentType)
	s.Assert().Equal(int64(15), info.Size)
}

func (s *MinIOIntegrationSuite) Test_upload_detects_content_type() {
	bucketName := "integration-test-ct"
	s.removeBucket(bucketName)

	provisioner, err := NewMinIOProvisioner(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)
	s.Require().NoError(provisioner.ProvisionBucket(context.Background(), bucketName))

	uploader, err := NewMinIOUploader(s.endpoint, s.accessKey, s.secretKey, s.logger)
	s.Require().NoError(err)

	s.Require().NoError(uploader.Upload(context.Background(), bucketName, "style.css", []byte("body { color: red; }")))

	client, err := minioclient.New(s.endpoint, &minioclient.Options{
		Creds:  miniocreds.NewStaticV4(s.accessKey, s.secretKey, ""),
		Secure: false,
	})
	s.Require().NoError(err)
	obj, err := client.GetObject(context.Background(), bucketName, "style.css", minioclient.GetObjectOptions{})
	s.Require().NoError(err)
	defer obj.Close()
	info, _ := obj.Stat()
	s.Assert().Equal("text/css", info.ContentType)
}
