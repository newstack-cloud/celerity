package plugin

import (
	context "context"
	"errors"
	"net"

	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/providerserverv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/pluginutils"
)

var (
	// ErrUnsupportedProviderProtocolVersion is returned when the protocol version
	// is not supported.
	ErrUnsupportedProviderProtocolVersion = errors.New("unsupported provider protocol version")
)

// ServeProviderConfiguration contains configuration for serving the plugin.
type ServeProviderConfiguration struct {
	// The unique identifier for the provider plugin.
	// In addition to being unique, the ID should point to the location
	// where the provider plugin can be downloaded.
	// {hostname/}?{namespace}/{provider}
	//
	// For example:
	// registry.celerityframework.io/celerity/aws
	// celerity/aws
	//
	// The last portion of the ID is the unique name of the provider
	// that is expected to be used as the namespace for resources, data sources
	// and custom variable types used in blueprints.
	// For example, the namespace for AWS resources is "aws"
	// used in the resource type "aws/lambda/function".
	ID string

	// ProtocolVersion is the protocol version that should be
	// used for the plugin.
	// Currently, the only supported protocol version is 1.
	ProtocolVersion uint32

	// PluginMetadata is the metadata for the plugin.
	// This is used to provide information about the plugin
	// to the host service.
	PluginMetadata *pluginservicev1.PluginMetadata

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

	// Listener is the listener that the provider plugin server should use.
	// If this is provided, TCPPort and UnixSocketPath will be ignored.
	Listener net.Listener
}

// ServeProviderV1 handles serving the v1 provider plugin with the given providerServer and options.
// This will deal with registering the provider with the host service.
func ServeProviderV1(
	ctx context.Context,
	providerServer any,
	pluginServiceClient pluginservicev1.ServiceClient,
	hostInfoContainer pluginutils.HostInfoContainer,
	config ServeProviderConfiguration,
) (func(), error) {
	if config.ID == "" {
		return nil, errors.New("ID is required for a provider plugin")
	}

	if config.PluginMetadata == nil {
		return nil, errors.New("PluginMetadata is required for a provider plugin")
	}

	if config.ProtocolVersion != 1 {
		return nil, ErrUnsupportedProviderProtocolVersion
	}

	provider, isv1Provider := providerServer.(providerserverv1.ProviderServer)
	if !isv1Provider {
		return nil, errors.New("unsupported provider server type")
	}

	opts := []providerserverv1.ServerOption{}
	if config.Debug {
		opts = append(opts, providerserverv1.WithDebug())
	}

	if config.TCPPort != 0 && config.UnixSocketPath != "" {
		return nil, errors.New("both TCPPort and UnixSocketPath cannot be set")
	}

	if config.UnixSocketPath != "" {
		opts = append(opts, providerserverv1.WithUnixSocket(config.UnixSocketPath))
	}

	if config.TCPPort != 0 {
		opts = append(opts, providerserverv1.WithTCPPort(config.TCPPort))
	}

	if config.Listener != nil {
		opts = append(opts, providerserverv1.WithListener(config.Listener))
	}

	server := providerserverv1.NewServer(
		config.ID,
		config.PluginMetadata,
		provider,
		pluginServiceClient,
		hostInfoContainer,
		opts...,
	)
	return server.Serve()
}
