package testutils

import (
	"context"
	"log"
	"net"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/deploy-engine/plugin/pluginservicev1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func StartPluginServiceServer(
	hostID string,
	pluginManager pluginservicev1.Manager,
	functionRegistry provider.FunctionRegistry,
	resourceDeployService provider.ResourceDeployService,
) (pluginservicev1.ServiceClient, func()) {
	bufferSize := 101024 * 1024
	listener := bufconn.Listen(bufferSize)
	serviceServer := pluginservicev1.NewServiceServer(
		pluginManager,
		functionRegistry,
		resourceDeployService,
		hostID,
	)

	server := pluginservicev1.NewServer(
		serviceServer,
		pluginservicev1.WithListener(listener),
	)
	close, err := server.Serve()
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

	client := pluginservicev1.NewServiceClient(conn)

	return client, close
}
