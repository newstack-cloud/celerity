// Package buildmanifest defines the on-disk build manifest produced by the
// Celerity CLI build step and consumed by the Celerity transformer plugin when
// producing concrete deploy resources (e.g. AWS Lambda function definitions).
//
// It is deliberately a small, dependency-free package (standard library only)
// so it can be shared as the single source of truth for the manifest wire
// format across the CLI and the transformer, instead of each side keeping a
// hand-copied struct that can silently drift.
package buildmanifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// Version is the current build manifest schema version.
	Version = "1"

	// FileName is the repo-relative path the CLI writes the manifest to.
	FileName = ".celerity/build-manifest.json"

	// ResourceLinksFilename is the name of the blueprint-derived resource
	// links JSON file bundled into the Lambda app archive. The SDK reads it
	// from the handler's module directory on cold start, removing the need to
	// pass the payload through environment variables (which have a tight 4 KB
	// total limit on AWS Lambda).
	ResourceLinksFilename = "__celerity_resource_links__.json"

	// HandlerScopeFilename is the name of the per-handler resource scope file
	// bundled alongside the resource links. Resource links describe every
	// resource in the application; this records which of them each handler
	// uses, and so which resource layers a given handler initialises.
	//
	// It is a separate file rather than a change to the resource links format
	// because the SDK ships in the runtime layer at its published version while
	// the CLI writing these files is whatever the developer has installed, so
	// the two are routinely out of step. An SDK that predates this file ignores
	// it and behaves as before, and a newer SDK falls back when it is absent.
	HandlerScopeFilename = "__celerity_handler_scope__.json"

	// Artifact type discriminators recorded on Artifact.Type.
	ArtifactTypeContainerImage = "container-image"
	ArtifactTypeLambdaArchive  = "lambda-archive"
	ArtifactTypeLambdaLayer    = "lambda-layer"
)

// Manifest records build artifacts produced by the build step. Written to
// FileName and referenced by the deploy engine via a context variable pointer.
//
// Target-specific output (AWS Lambda, container, etc.) lives under a dedicated
// typed sub-manifest so adding a new strategy does not require changing the
// top-level shape.
type Manifest struct {
	Version  string                       `json:"version"`
	Runtime  string                       `json:"runtime"`
	Target   string                       `json:"target"`
	Handlers map[string]*HandlerArtifacts `json:"handlers"`
	// Lambda is populated only when the build target is aws-serverless.
	Lambda *LambdaManifest `json:"lambda,omitempty"`
	// RemoteURL is the cloud storage URL where the manifest itself has been
	// uploaded, set after a successful upload. Populated only in the
	// locally-persisted copy; the uploaded object does not contain this field
	// (it is set after marshalling for upload). Consumers reading the uploaded
	// manifest can ignore it.
	RemoteURL string `json:"remoteUrl,omitempty"`
}

// HandlerArtifacts holds per-handler build metadata. Target-specific overrides
// (e.g. a custom dependency layer for Lambda) live under their own sub-struct
// so each strategy owns the shape of its per-handler data.
type HandlerArtifacts struct {
	Lambda *LambdaHandlerArtifacts `json:"lambda,omitempty"`
}

// Artifact represents a single build artifact (zip archive or image).
type Artifact struct {
	Type        string `json:"type"`
	LocalPath   string `json:"localPath,omitempty"`
	S3Bucket    string `json:"s3Bucket,omitempty"`
	S3Key       string `json:"s3Key,omitempty"`
	ImageURI    string `json:"imageUri,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// LambdaManifest holds AWS Lambda-specific build output referenced by every
// handler's Lambda function. Populated by the CLI's Lambda build strategy and
// read by the Celerity transformer plugin.
type LambdaManifest struct {
	// AppCode is the single shared code asset, the whole app source tree plus
	// the generated Celerity Lambda entry point file.
	AppCode *Artifact `json:"appCode,omitempty"`
	// EntryPoint is the Lambda Handler field value, identical for every handler
	// in a project (e.g. "__celerity_lambda_entry__.handler").
	EntryPoint string `json:"entryPoint,omitempty"`
	// SharedLayer is the production dependency layer used by handlers that do
	// not opt into a custom per-handler dep layer.
	SharedLayer *Artifact `json:"sharedLayer,omitempty"`
	// Architecture is the instruction set the archive's code is built for,
	// as a GOARCH name ("arm64", "amd64"). Empty for an interpreted runtime,
	// where the code is not built for one.
	//
	// Recorded because a compiled handlers executable and the function it is
	// deployed to have to agree. A mismatch is a valid archive that the kernel
	// refuses to exec, surfacing as a bare exit status 126. The transformer
	// declares the function's architecture from this field.
	Architecture string `json:"architecture,omitempty"`
	// LambdaRuntime is the concrete Lambda runtime identifier the functions
	// need, where the build has determined it. A compiled executable runs on
	// a custom runtime ("provided.al2023") rather than on one named by the
	// blueprint's portable runtime identifier.
	LambdaRuntime string `json:"lambdaRuntime,omitempty"`
	// MaintenanceRunner is the database maintenance runner's archive, built
	// only when the blueprint has a celerity/sqlDatabase.
	//
	// It is a compiled Go binary rather than application code, so it is neither
	// the app bundle nor a layer. It shares nothing with them and is versioned
	// with the CLI rather than with the user's source.
	MaintenanceRunner *Artifact `json:"maintenanceRunner,omitempty"`
}

// LambdaHandlerArtifacts holds per-handler Lambda overrides.
type LambdaHandlerArtifacts struct {
	// Dependencies is the handler's custom dependency layer, populated only
	// when the developer has declared handlerDependencies. Nil handlers fall
	// back to LambdaManifest.SharedLayer.
	Dependencies *Artifact `json:"dependencies,omitempty"`
}

// Write writes the build manifest to FileName, creating the parent directory
// if needed.
func Write(manifest *Manifest) error {
	dir := filepath.Dir(FileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling build manifest: %w", err)
	}

	if err := os.WriteFile(FileName, data, 0644); err != nil {
		return fmt.Errorf("writing build manifest: %w", err)
	}

	return nil
}

// Read reads the build manifest from FileName. Returns (nil, nil) if the file
// does not exist.
func Read() (*Manifest, error) {
	data, err := os.ReadFile(FileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading build manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing build manifest: %w", err)
	}

	return &manifest, nil
}
