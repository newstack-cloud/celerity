package build

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ManifestSuite struct {
	suite.Suite
	origDir string
}

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(ManifestSuite))
}

func (s *ManifestSuite) SetupTest() {
	// WriteManifest and ReadManifest use the relative path ManifestFile,
	// so we chdir into a temp directory for each test.
	dir := s.T().TempDir()
	origDir, err := os.Getwd()
	s.Require().NoError(err)
	s.origDir = origDir
	s.Require().NoError(os.Chdir(dir))
}

func (s *ManifestSuite) TearDownTest() {
	s.Require().NoError(os.Chdir(s.origDir))
}

func (s *ManifestSuite) Test_WriteManifest_thenReadManifest_roundTrip() {
	original := &Manifest{
		Version: ManifestVersion,
		Runtime: "nodejs24.x",
		Target:  "aws-serverless",
		Lambda: &LambdaManifest{
			EntryPoint: "__celerity_lambda_entry__.handler",
			AppCode: &Artifact{
				Type:        ArtifactTypeLambdaArchive,
				LocalPath:   "/tmp/app-abc123.zip",
				ContentHash: "sha256:abc123",
				Size:        8192,
			},
			SharedLayer: &Artifact{
				Type:        ArtifactTypeLambdaLayer,
				LocalPath:   "/tmp/shared-layer-def456.zip",
				ContentHash: "sha256:def456",
				Size:        4096,
			},
		},
		Handlers: map[string]*HandlerArtifacts{
			"orderHandler": {
				Lambda: &LambdaHandlerArtifacts{
					Dependencies: &Artifact{
						Type:        ArtifactTypeLambdaLayer,
						LocalPath:   "/tmp/orderHandler-layer-ghi789.zip",
						ContentHash: "sha256:ghi789",
						Size:        2048,
					},
				},
			},
			"userHandler": {Lambda: &LambdaHandlerArtifacts{}},
		},
	}

	err := WriteManifest(original)
	s.Require().NoError(err)

	loaded, err := ReadManifest()
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	s.Equal(original.Version, loaded.Version)
	s.Equal(original.Runtime, loaded.Runtime)
	s.Equal(original.Target, loaded.Target)

	s.Require().NotNil(loaded.Lambda)
	s.Equal(original.Lambda.EntryPoint, loaded.Lambda.EntryPoint)

	s.Require().NotNil(loaded.Lambda.AppCode)
	s.Equal(original.Lambda.AppCode.ContentHash, loaded.Lambda.AppCode.ContentHash)
	s.Equal(original.Lambda.AppCode.LocalPath, loaded.Lambda.AppCode.LocalPath)

	s.Require().NotNil(loaded.Lambda.SharedLayer)
	s.Equal(original.Lambda.SharedLayer.ContentHash, loaded.Lambda.SharedLayer.ContentHash)
	s.Equal(original.Lambda.SharedLayer.Size, loaded.Lambda.SharedLayer.Size)

	s.Require().Contains(loaded.Handlers, "orderHandler")
	s.Require().NotNil(loaded.Handlers["orderHandler"].Lambda)
	s.Require().NotNil(loaded.Handlers["orderHandler"].Lambda.Dependencies)
	s.Equal(
		original.Handlers["orderHandler"].Lambda.Dependencies.ContentHash,
		loaded.Handlers["orderHandler"].Lambda.Dependencies.ContentHash,
	)

	s.Require().Contains(loaded.Handlers, "userHandler")
	userLambda := loaded.Handlers["userHandler"].Lambda
	if userLambda != nil {
		s.Nil(userLambda.Dependencies, "userHandler uses shared layer")
	}
}

func (s *ManifestSuite) Test_ReadManifest_returnsNilForMissingFile() {
	loaded, err := ReadManifest()

	s.NoError(err)
	s.Nil(loaded)
}

func (s *ManifestSuite) Test_WriteManifest_createsCelerityDirectory() {
	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  "python3.12",
		Target:   "aws-serverless",
		Handlers: map[string]*HandlerArtifacts{},
	}

	err := WriteManifest(manifest)
	s.Require().NoError(err)

	info, err := os.Stat(".celerity")
	s.Require().NoError(err)
	s.True(info.IsDir())
}

func (s *ManifestSuite) Test_WriteManifest_persistsRemoteURL() {
	manifest := &Manifest{
		Version:   ManifestVersion,
		Runtime:   "nodejs24.x",
		Target:    "aws-serverless",
		Handlers:  map[string]*HandlerArtifacts{},
		RemoteURL: "s3://celerity-build-123-us-east-1/default/build-manifest.json",
	}

	s.Require().NoError(WriteManifest(manifest))

	loaded, err := ReadManifest()
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	s.Equal(manifest.RemoteURL, loaded.RemoteURL)
}

func (s *ManifestSuite) Test_UploadManifest_setsRemoteURLOnSuccess() {
	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*HandlerArtifacts{},
	}
	uploader := &fakeUploader{
		manifestURL: "s3://test-bucket/default/build-manifest.json",
	}

	err := UploadManifest(context.Background(), uploader, manifest, "default")
	s.Require().NoError(err)

	s.Equal("s3://test-bucket/default/build-manifest.json", manifest.RemoteURL)
	s.Equal("default", uploader.lastInstance)
	s.Len(uploader.lastData, len(uploader.lastData))

	// The uploaded bytes must NOT self-reference. RemoteURL is set after
	// marshalling, so the S3 object has an empty field.
	var uploaded Manifest
	s.Require().NoError(json.Unmarshal(uploader.lastData, &uploaded))
	s.Empty(uploaded.RemoteURL)
}

func (s *ManifestSuite) Test_UploadManifest_wrapsUploaderError() {
	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*HandlerArtifacts{},
	}
	boom := errors.New("boom")
	uploader := &fakeUploader{manifestErr: boom}

	err := UploadManifest(context.Background(), uploader, manifest, "default")
	s.Require().Error(err)
	s.ErrorIs(err, boom)
	s.Empty(manifest.RemoteURL)
}

func (s *ManifestSuite) Test_UploadManifest_nilUploader_noop() {
	manifest := &Manifest{
		Version:  ManifestVersion,
		Runtime:  "nodejs24.x",
		Target:   "aws-serverless",
		Handlers: map[string]*HandlerArtifacts{},
	}

	err := UploadManifest(context.Background(), nil, manifest, "default")
	s.NoError(err)
	s.Empty(manifest.RemoteURL)
}
