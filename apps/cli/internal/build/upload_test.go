package build

import "context"

// fakeUploader is a test-only ArtifactUploader that records the last
// call without touching any network. Used by manifest_test.go and any
// future unit-level test that needs an uploader without MinIO.
type fakeUploader struct {
	artifacts []uploadedArtifact

	manifestURL  string
	manifestErr  error
	lastData     []byte
	lastInstance string
}

type uploadedArtifact struct {
	artifact *Artifact
	key      string
}

func (f *fakeUploader) Upload(_ context.Context, artifact *Artifact, key string) error {
	f.artifacts = append(f.artifacts, uploadedArtifact{artifact: artifact, key: key})
	return nil
}

func (f *fakeUploader) UploadManifest(_ context.Context, data []byte, instanceName string) (string, error) {
	f.lastData = append([]byte(nil), data...)
	f.lastInstance = instanceName
	if f.manifestErr != nil {
		return "", f.manifestErr
	}
	return f.manifestURL, nil
}
