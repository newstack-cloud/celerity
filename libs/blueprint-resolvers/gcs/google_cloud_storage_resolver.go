package gcs

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
	"google.golang.org/api/option"
)

type gcsChildResolver struct {
	endpoint string
}

// NewResolver creates a new instance of a ChildResolver
// that resolves child blueprints from a Google Cloud Storage bucket.
// This relies on fields defined in the include metadata,
// specifically the `bucket` and `region` fields.
// Credentials are loaded via the standard Google Cloud SDK mechanisms.
func NewResolver(endpoint string) includes.ChildResolver {
	return &gcsChildResolver{
		endpoint,
	}
}

func (r *gcsChildResolver) Resolve(
	ctx context.Context,
	includeName string,
	include *subengine.ResolvedInclude,
	params core.BlueprintParams,
) (*includes.ChildBlueprintInfo, error) {

	path := includes.StringValue(include.Path)
	if path == "" {
		return nil, includes.ErrInvalidPath(includeName, "google cloud storage")
	}

	metadata := include.Metadata
	if metadata == nil || metadata.Fields == nil {
		return nil, includes.ErrInvalidMetadata(
			includeName,
			"invalid metadata provided for a Google Cloud Storage include",
		)
	}

	bucket := includes.StringValue(metadata.Fields["bucket"])
	if bucket == "" {
		return nil, includes.ErrInvalidMetadata(
			includeName,
			"missing bucket field in metadata for a Google Cloud Storage include",
		)
	}

	client, err := createClient(ctx, r.endpoint)
	if err != nil {
		return nil, includes.ErrResolveFailure(includeName, err)
	}

	bucketHandle := client.Bucket(bucket)
	objectHandle := bucketHandle.Object(path)
	reader, err := objectHandle.NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, includes.ErrBlueprintNotFound(
				includeName,
				fmt.Sprintf("gcs://%s/%s", bucket, path),
			)
		}

		return nil, includes.ErrResolveFailure(includeName, err)
	}
	defer reader.Close()

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, includes.ErrResolveFailure(includeName, err)
	}
	bodyStr := string(bodyBytes)

	return &includes.ChildBlueprintInfo{
		BlueprintSource: &bodyStr,
	}, nil
}

func createClient(ctx context.Context, endpoint string) (*storage.Client, error) {
	if endpoint == "" {
		return storage.NewClient(ctx)
	}

	return storage.NewClient(ctx, option.WithEndpoint(endpoint))
}
