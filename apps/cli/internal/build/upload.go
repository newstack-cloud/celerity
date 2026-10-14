package build

// ResolveUploader returns the appropriate artifact uploader for the deploy
// target, or nil if the target doesn't require pre-upload (e.g. Azure).
func ResolveUploader(deployTarget string, providerConfig map[string]any) ArtifactUploader {
	switch deployTarget {
	case "aws-serverless":
		return newS3Uploader(providerConfig)
	default:
		return nil
	}
}
