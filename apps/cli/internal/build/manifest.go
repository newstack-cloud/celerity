package build

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/newstack-cloud/celerity/apps/cli/buildmanifest"
)

// WriteManifest writes the build manifest to buildmanifest.FileName.
// Thin re-export of buildmanifest.Write so internal/build call sites are
// unchanged.
func WriteManifest(manifest *Manifest) error {
	return buildmanifest.Write(manifest)
}

// ReadManifest reads the build manifest from buildmanifest.FileName, returning
// nil if the file does not exist. Thin re-export of buildmanifest.Read.
func ReadManifest() (*Manifest, error) {
	return buildmanifest.Read()
}

// UploadManifest serialises the manifest and uploads it via the given
// uploader, stamping the returned URL onto manifest.RemoteURL.
//
// Callers must call this before persisting the manifest locally with
// WriteManifest so the RemoteURL is part of the on-disk copy but not of the
// uploaded bytes. The upload happens before the stamping, so the S3 object
// holds no self-reference.
//
// A nil uploader is a no-op that leaves manifest.RemoteURL empty, which lets
// deploy targets without artifact upload share the same call site.
func UploadManifest(
	ctx context.Context,
	uploader ArtifactUploader,
	manifest *Manifest,
	instanceName string,
) error {
	if uploader == nil {
		return nil
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling build manifest: %w", err)
	}
	url, err := uploader.UploadManifest(ctx, data, instanceName)
	if err != nil {
		return fmt.Errorf("uploading build manifest: %w", err)
	}
	manifest.RemoteURL = url
	return nil
}
