package build

import "github.com/newstack-cloud/celerity/apps/cli/buildmanifest"

const (
	ArtifactTypeLambdaArchive = buildmanifest.ArtifactTypeLambdaArchive
	ArtifactTypeLambdaLayer   = buildmanifest.ArtifactTypeLambdaLayer

	LayerSizeWarning int64 = 200 * 1024 * 1024 // 200MB
	LayerSizeLimit   int64 = 250 * 1024 * 1024 // 250MB
)
