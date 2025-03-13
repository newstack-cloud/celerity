package testprovider

import (
	"context"
	"log"
	"net"

	"github.com/two-hundred/celerity/libs/plugin-framework/plugin"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/providerserverv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/plugin/sdk/providerv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// StartPluginServer starts the test provider plugin server
// to run in the same process as the test suite.
func StartPluginServer(serviceClient pluginservicev1.ServiceClient) (providerserverv1.ProviderClient, func()) {
	bufferSize := 101024 * 1024
	listener := bufconn.Listen(bufferSize)
	providerServer := providerv1.NewProviderPlugin(NewProvider())
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
		Listener:        listener,
	}

	close, err := plugin.ServeProviderV1(
		context.Background(),
		providerServer,
		serviceClient,
		config,
	)
	if err != nil {
		log.Fatal(err.Error())
	}

	conn, err := grpc.NewClient(
		"",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	client := providerserverv1.NewProviderClient(conn)

	return client, close
}
