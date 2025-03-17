package main

import (
	"context"
	"log"

	"github.com/two-hundred/celerity/libs/plugin-framework/plugin"
	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/registry/providers/aws"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/pluginutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

func main() {
	providerServer := providerv1.NewProviderPlugin(aws.NewProvider())
	config := plugin.ServeProviderConfiguration{
		ID: "celerity/aws",
		PluginMetadata: &pluginservicev1.PluginMetadata{
			PluginVersion: "1.0.0",
			DisplayName:   "AWS",
			FormattedDescription: "AWS provider for the Deploy Engine including `resources`, `data sources`," +
				" `links` and `custom variable types` for interacting with AWs services.",
			RepositoryUrl: "https://github.com/two-hundred/celerity-provider-aws",
			Author:        "Two Hundred",
		},
		ProtocolVersion: 1,
	}
	serviceClient, closeService, err := pluginservicev1.NewEnvServiceClient()
	if err != nil {
		log.Fatal(err.Error())
	}
	defer closeService()

	close, err := plugin.ServeProviderV1(
		context.Background(),
		providerServer,
		serviceClient,
		config,
	)
	if err != nil {
		log.Fatal(err.Error())
	}
	pluginutils.WaitForShutdown(close)
}
