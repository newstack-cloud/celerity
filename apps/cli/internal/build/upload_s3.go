package build

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// S3ArtifactUploader uploads build artifacts to an S3 bucket.
// The bucket is auto-created with a conventional name if it doesn't exist.
type S3ArtifactUploader struct {
	client         *s3.Client
	bucket         string
	bucketEnsured  bool
	region         string
	providerConfig map[string]any
}

func newS3Uploader(providerConfig map[string]any) *S3ArtifactUploader {
	return &S3ArtifactUploader{
		providerConfig: providerConfig,
		region:         awsConfigValue(providerConfig, "region", "us-east-1"),
	}
}

func (u *S3ArtifactUploader) Upload(ctx context.Context, artifact *Artifact, key string) error {
	client, err := u.getClient(ctx)
	if err != nil {
		return err
	}

	bucket, err := u.ensureBucket(ctx, client)
	if err != nil {
		return err
	}

	// Check if object already exists (hash-based dedup).
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		artifact.S3Bucket = bucket
		artifact.S3Key = key
		return nil
	}

	file, err := os.Open(artifact.LocalPath)
	if err != nil {
		return fmt.Errorf("opening artifact for upload: %w", err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("uploading to s3://%s/%s: %w", bucket, key, err)
	}

	artifact.S3Bucket = bucket
	artifact.S3Key = key
	return nil
}

// UploadManifest uploads the serialised build manifest JSON to the
// same bucket used for artifacts and returns the s3:// URL. Unlike
// Upload, this always overwrites the existing object. The manifest at a
// fixed per-instance key is mutable by design, since every build supersedes
// the previous one.
func (u *S3ArtifactUploader) UploadManifest(
	ctx context.Context, data []byte, instanceName string,
) (string, error) {
	client, err := u.getClient(ctx)
	if err != nil {
		return "", err
	}
	bucket, err := u.ensureBucket(ctx, client)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("%s/build-manifest.json", instanceName)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("uploading build manifest to s3://%s/%s: %w", bucket, key, err)
	}
	return fmt.Sprintf("s3://%s/%s", bucket, key), nil
}

func (u *S3ArtifactUploader) getClient(ctx context.Context) (*s3.Client, error) {
	if u.client != nil {
		return u.client, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, u.awsConfigOptions()...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}

	endpoint := awsConfigValue(u.providerConfig, "endpoint.s3", "")
	if endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	if awsConfigValue(u.providerConfig, "s3UsePathStyle", "") == "true" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	u.client = s3.NewFromConfig(cfg, s3Opts...)
	return u.client, nil
}

func (u *S3ArtifactUploader) ensureBucket(ctx context.Context, client *s3.Client) (string, error) {
	if u.bucketEnsured {
		return u.bucket, nil
	}

	// Allow bucket override from deploy config.
	if override := awsConfigValue(u.providerConfig, "build.artifactBucket", ""); override != "" {
		u.bucket = override
		u.bucketEnsured = true
		return u.bucket, nil
	}

	accountID, err := u.resolveAccountID(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving AWS account ID for bucket name: %w", err)
	}

	u.bucket = fmt.Sprintf("celerity-build-%s-%s", accountID, u.region)

	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(u.bucket),
	})
	if err == nil {
		u.bucketEnsured = true
		return u.bucket, nil
	}

	createInput := &s3.CreateBucketInput{
		Bucket: aws.String(u.bucket),
	}
	if u.region != "us-east-1" {
		createInput.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(u.region),
		}
	}

	_, err = client.CreateBucket(ctx, createInput)
	if err != nil {
		return "", fmt.Errorf("creating build artifact bucket %s: %w", u.bucket, err)
	}

	u.bucketEnsured = true
	return u.bucket, nil
}

func (u *S3ArtifactUploader) resolveAccountID(ctx context.Context) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, u.awsConfigOptions()...)
	if err != nil {
		return "", err
	}

	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("getting caller identity: %w", err)
	}

	return *identity.Account, nil
}

// Artifacts have to land in the account the blueprint deploys to, so every
// client the uploader builds resolves credentials exactly as the deployment
// does. Building one of them differently, as the account lookup used to, sends
// that call down the ambient credential chain and names the artifact bucket
// after whatever account that chain resolves to.
func (u *S3ArtifactUploader) awsConfigOptions() []func(*awsconfig.LoadOptions) error {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(u.region),
	}

	accessKey := awsConfigValue(u.providerConfig, "accessKeyId", "")
	secretKey := awsConfigValue(u.providerConfig, "secretAccessKey", "")
	if accessKey != "" && secretKey != "" {
		sessionToken := awsConfigValue(u.providerConfig, "sessionToken", "")
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken),
		))
	}

	if profile := awsConfigValue(u.providerConfig, "profile", ""); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	return opts
}

// Deploy target settings are written namespaced in app.deploy.jsonc
// ("aws.region") and reach the uploader in that form. The bare key is still
// accepted so a caller holding an already-stripped config keeps working.
// Looking up only the bare key, as this once did, matched none of the deploy
// target's AWS settings, so the region silently defaulted to us-east-1,
// artifacts were uploaded to a bucket in the wrong region, and Lambda then
// refused to read them across regions.
func awsConfigValue(config map[string]any, key, defaultVal string) string {
	if value := stringFromConfig(config, "aws."+key, ""); value != "" {
		return value
	}

	return stringFromConfig(config, key, defaultVal)
}

func stringFromConfig(config map[string]any, key, defaultVal string) string {
	if config == nil {
		return defaultVal
	}
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}
