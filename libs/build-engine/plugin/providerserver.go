package plugin

import (
	context "context"
	"errors"
	"os"
	"strconv"

	"github.com/two-hundred/celerity/libs/build-engine/plugin/pluginservice"
	"github.com/two-hundred/celerity/libs/build-engine/plugin/providerserverv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// ErrUnsupportedProviderProtocolVersion is returned when the protocol version
	// is not supported.
	ErrUnsupportedProviderProtocolVersion = errors.New("unsupported provider protocol version")
)

// ServeProviderOptions are the options for serving the plugin.
type ServeProviderOptions struct {
	// The unique identifier for the provider plugin.
	// In addition to being unique, the ID should point to the location
	// where the provider plugin can be downloaded.
	// {hostname/}?{namespace}/{provider}
	//
	// For example:
	// registry.celerityframework.com/celerity/aws
	// celerity/aws
	ID string
	// ProtocolVersion is the protocol version that should be
	// used for the plugin.
	// Currently, the only supported protocol version is 1.
	ProtocolVersion uint32

	// Debug runs the provider plugin in a mode compatible with
	// debugging processes such as delve.
	Debug bool

	// UnixSocketPath is the path to the Unix socket that the provider
	// plugin should listen on.
	// If this is set, the TCPPort should be empty.
	UnixSocketPath string

	// TCPPort is the port that the provider plugin should listen on.
	// If this is set, the UnixSocketPath should be empty.
	// If this is not set and UnixSocketPath is not set, the provider
	// plugin will listen on the next available port.
	TCPPort int
}

// Serves the plugin with the given providerServer and options.
// This will deal with registering the provider with the host service.
func ServeProvider(ctx context.Context, providerServer interface{}, options ServeProviderOptions) error {
	if options.ProtocolVersion != 1 {
		return ErrUnsupportedProviderProtocolVersion
	}

	provider, isv1Provider := providerServer.(providerserverv1.ProviderServer)
	if !isv1Provider {
		return errors.New("unsupported provider server type")
	}

	opts := []providerserverv1.ServerOption{}
	if options.Debug {
		opts = append(opts, providerserverv1.WithDebug())
	}

	if options.TCPPort != 0 && options.UnixSocketPath != "" {
		return errors.New("both TCPPort and UnixSocketPath cannot be set")
	}

	if options.UnixSocketPath != "" {
		opts = append(opts, providerserverv1.WithUnixSocket(options.UnixSocketPath))
	}

	if options.TCPPort != 0 {
		opts = append(opts, providerserverv1.WithTCPPort(options.TCPPort))
	}

	server := providerserverv1.NewServer(
		options.ID,
		provider,
		createServiceClientFactory(),
		opts...,
	)
	return server.Serve()
}

func createServiceClientFactory() func() (pluginservice.ServiceClient, func(), error) {
	return func() (pluginservice.ServiceClient, func(), error) {
		servicePort := os.Getenv("CELERITY_BUILD_ENGINE_PLUGIN_SERVICE_PORT")
		if servicePort == "" {
			servicePort = strconv.Itoa(pluginservice.DefaultPort)
		}

		conn, err := grpc.NewClient("127.0.0.1:"+servicePort, grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		))
		if err != nil {
			return nil, nil, err
		}

		client := pluginservice.NewServiceClient(conn)
		close := func() {
			conn.Close()
		}
		return client, close, nil
	}
}
