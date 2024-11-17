package main

import (
	"context"
	"log"

	"github.com/two-hundred/celerity/libs/deploy-engine/plugin"
	"github.com/two-hundred/celerity/libs/deploy-engine/plugin/sdk/providerv1"
	"github.com/two-hundred/celerity/libs/deploy-engine/registry/providers/aws"
)

func main() {
	providerServer := providerv1.NewProviderPlugin(aws.NewProvider())
	options := plugin.ServeProviderOptions{
		ID:              "celerity/aws",
		ProtocolVersion: 1,
	}
	err := plugin.ServeProvider(context.Background(), providerServer, options)
	if err != nil {
		log.Fatal(err.Error())
	}
}
