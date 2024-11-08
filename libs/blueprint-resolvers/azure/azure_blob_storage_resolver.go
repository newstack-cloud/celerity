package azure

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/two-hundred/celerity/libs/blueprint-resolvers/utils"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/includes"
	"github.com/two-hundred/celerity/libs/blueprint/subengine"
)

type azureBlobStorageChildResolver struct {
	client *azblob.Client
}

// NewResolver creates a new instance of a ChildResolver
// that resolves child blueprints from an Azure Blob Storage container.
// This relies on fields defined in the include metadata,
// specifically the `container` field.
func NewResolver(client *azblob.Client) includes.ChildResolver {
	return &azureBlobStorageChildResolver{
		client,
	}
}

func (r *azureBlobStorageChildResolver) Resolve(
	ctx context.Context,
	includeName string,
	include *subengine.ResolvedInclude,
	params core.BlueprintParams,
) (*includes.ChildBlueprintInfo, error) {

	err := utils.ValidateInclude(
		include,
		includeName,
		[]string{"container"},
		"Azure Blob Storage",
		"azure blob storage",
	)
	if err != nil {
		return nil, err
	}

	path := includes.StringValue(include.Path)
	container := includes.StringValue(include.Metadata.Fields["container"])

	stream, err := r.client.DownloadStream(ctx, container, path, nil)
	if err != nil {
		var responseErr *azcore.ResponseError
		if errors.As(err, &responseErr) && responseErr.StatusCode == 404 {
			return nil, includes.ErrBlueprintNotFound(
				includeName,
				fmt.Sprintf("azure-blob-storage://%s/%s", container, path),
			)
		}

		return nil, includes.ErrResolveFailure(includeName, err)
	}

	downloadedData := bytes.Buffer{}
	retryReader := stream.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	_, err = downloadedData.ReadFrom(retryReader)
	if err != nil {
		return nil, includes.ErrResolveFailure(includeName, err)
	}

	downloadedDataStr := downloadedData.String()
	return &includes.ChildBlueprintInfo{
		BlueprintSource: &downloadedDataStr,
	}, nil
}
