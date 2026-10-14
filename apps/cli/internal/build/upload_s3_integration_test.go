package build

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/suite"
)

const (
	testMinioEndpoint = "http://localhost:49000"
	testMinioBucket   = "celerity-build-test"
	testMinioRegion   = "us-east-1"
)

type S3UploadIntegrationSuite struct {
	suite.Suite
	s3Client *s3.Client
}

func TestS3UploadIntegrationSuite(t *testing.T) {
	if !isMinioAvailable() {
		t.Skip("MinIO not available on port 49000, run docker-compose.test-deps.yml")
	}
	suite.Run(t, new(S3UploadIntegrationSuite))
}

func (s *S3UploadIntegrationSuite) SetupSuite() {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(testMinioRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", ""),
		),
	)
	s.Require().NoError(err)

	s.s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(testMinioEndpoint)
		o.UsePathStyle = true
	})

	_, err = s.s3Client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(testMinioBucket),
	})
	// Ignore "bucket already exists" errors from previous test runs.
	if err != nil {
		s.T().Logf("CreateBucket: %v (may already exist)", err)
	}
}

func (s *S3UploadIntegrationSuite) TestUpload_uploadsArtifactToMinio() {
	uploader := s.newUploader()

	artifact := s.createTestArtifact("test-handler.zip", "handler code content")
	key := "test-instance/handler/abc123.zip"

	err := uploader.Upload(context.Background(), artifact, key)
	s.Require().NoError(err)
	s.Equal(testMinioBucket, artifact.S3Bucket)
	s.Equal(key, artifact.S3Key)

	s.assertObjectExists(key)
}

func (s *S3UploadIntegrationSuite) TestUpload_skipsIfKeyAlreadyExists() {
	uploader := s.newUploader()

	artifact := s.createTestArtifact("dedup-test.zip", "some content")
	key := "test-instance/dedup/sha256-dedup.zip"

	err := uploader.Upload(context.Background(), artifact, key)
	s.Require().NoError(err)

	// Upload again with a different local file, which should skip the upload
	// and still set the S3 fields.
	artifact2 := s.createTestArtifact("dedup-test-2.zip", "different content")
	err = uploader.Upload(context.Background(), artifact2, key)
	s.Require().NoError(err)
	s.Equal(testMinioBucket, artifact2.S3Bucket)
	s.Equal(key, artifact2.S3Key)
}

func (s *S3UploadIntegrationSuite) TestUpload_ensuresBucketFromConfig() {
	customBucket := fmt.Sprintf("celerity-build-custom-%d", os.Getpid())

	uploader := newS3Uploader(map[string]any{
		"accessKeyId":              "minioadmin",
		"secretAccessKey":          "minioadmin",
		"region":                   testMinioRegion,
		"endpoint.s3":              testMinioEndpoint,
		"s3UsePathStyle":           "true",
		"aws.build.artifactBucket": customBucket,
	})

	// Create the custom bucket first.
	_, err := s.s3Client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(customBucket),
	})
	if err != nil {
		s.T().Logf("CreateBucket: %v (may already exist)", err)
	}

	artifact := s.createTestArtifact("custom-bucket.zip", "custom content")
	key := "test/custom.zip"

	err = uploader.Upload(context.Background(), artifact, key)
	s.Require().NoError(err)
	s.Equal(customBucket, artifact.S3Bucket)
}

func (s *S3UploadIntegrationSuite) TestUploadManifest_writesJSONBlobToS3() {
	uploader := s.newUploader()

	data := []byte(`{"version":"1","target":"aws-serverless"}`)
	url, err := uploader.UploadManifest(context.Background(), data, "my-app")
	s.Require().NoError(err)
	s.Equal(fmt.Sprintf("s3://%s/my-app/build-manifest.json", testMinioBucket), url)

	got, err := s.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(testMinioBucket),
		Key:    aws.String("my-app/build-manifest.json"),
	})
	s.Require().NoError(err)
	defer got.Body.Close()
	body, err := io.ReadAll(got.Body)
	s.Require().NoError(err)
	s.Equal(data, body)
	s.NotNil(got.ContentType)
	s.Equal("application/json", aws.ToString(got.ContentType))
}

func (s *S3UploadIntegrationSuite) TestUploadManifest_overwritesExistingObject() {
	uploader := s.newUploader()

	first := []byte(`{"version":"1","target":"first"}`)
	_, err := uploader.UploadManifest(context.Background(), first, "overwrite-test")
	s.Require().NoError(err)

	second := []byte(`{"version":"1","target":"second"}`)
	_, err = uploader.UploadManifest(context.Background(), second, "overwrite-test")
	s.Require().NoError(err)

	got, err := s.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(testMinioBucket),
		Key:    aws.String("overwrite-test/build-manifest.json"),
	})
	s.Require().NoError(err)
	defer got.Body.Close()
	body, err := io.ReadAll(got.Body)
	s.Require().NoError(err)
	s.Equal(second, body)
}

func (s *S3UploadIntegrationSuite) newUploader() *S3ArtifactUploader {
	return newS3Uploader(map[string]any{
		"accessKeyId":              "minioadmin",
		"secretAccessKey":          "minioadmin",
		"region":                   testMinioRegion,
		"endpoint.s3":              testMinioEndpoint,
		"s3UsePathStyle":           "true",
		"aws.build.artifactBucket": testMinioBucket,
	})
}

func (s *S3UploadIntegrationSuite) createTestArtifact(name, content string) *Artifact {
	dir := s.T().TempDir()
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0644))
	return &Artifact{
		Type:      ArtifactTypeLambdaArchive,
		LocalPath: path,
	}
}

func (s *S3UploadIntegrationSuite) assertObjectExists(key string) {
	_, err := s.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(testMinioBucket),
		Key:    aws.String(key),
	})
	s.Require().NoError(err, "expected object %s to exist in bucket %s", key, testMinioBucket)
}

func isMinioAvailable() bool {
	conn, err := net.Dial("tcp", "localhost:49000")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
