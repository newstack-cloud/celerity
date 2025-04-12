package providerserverv1

import (
	context "context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/two-hundred/celerity/libs/plugin-framework/pluginservicev1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/pluginutils"
	"github.com/two-hundred/celerity/libs/plugin-framework/utils"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// The protocol version that is used during the handshake to ensure the plugin
	// is compatible with the host service.
	protocolVersion = "1.0"
)

// ServerOption is a function that configures a server.
type ServerOption func(*Server)

// WithDebug is a server option that enables debug mode.
func WithDebug() ServerOption {
	return func(s *Server) {
		s.debug = true
	}
}

// WithUnixSocket is a server option that sets the Unix socket path.
func WithUnixSocket(path string) ServerOption {
	return func(s *Server) {
		s.unixSocket = path
	}
}

// WithTCPPort is a server option that sets the TCP port.
func WithTCPPort(port int) ServerOption {
	return func(s *Server) {
		s.tcpPort = port
	}
}

// WithListener is a server option that sets the listener
// that the server should use.
func WithListener(listener net.Listener) ServerOption {
	return func(s *Server) {
		s.listener = listener
	}
}

// Server is a plugin server.
type Server struct {
	pluginID          string
	pluginMetadata    *pluginservicev1.PluginMetadata
	debug             bool
	unixSocket        string
	tcpPort           int
	provider          ProviderServer
	pluginService     pluginservicev1.ServiceClient
	hostInfoContainer pluginutils.HostInfoContainer
	listener          net.Listener
}

func NewServer(
	pluginID string,
	pluginMetadata *pluginservicev1.PluginMetadata,
	provider ProviderServer,
	pluginServiceClient pluginservicev1.ServiceClient,
	hostInfoContainer pluginutils.HostInfoContainer,
	opts ...ServerOption,
) *Server {
	server := &Server{
		pluginID:          pluginID,
		pluginMetadata:    pluginMetadata,
		provider:          provider,
		pluginService:     pluginServiceClient,
		hostInfoContainer: hostInfoContainer,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *Server) Serve() (func(), error) {
	listener, err := s.createListener()
	if err != nil {
		return nil, err
	}

	// If the TCP port is not set and a unix socket is not provided,
	// get the dynamically assigned port if a custom listener
	// has not been provided.
	if s.tcpPort == 0 && s.unixSocket == "" && s.listener == nil {
		s.tcpPort = listener.Addr().(*net.TCPAddr).Port
	}

	opts := []grpc.ServerOption{
		grpc.Creds(insecure.NewCredentials()),
	}

	grpcServer := grpc.NewServer(opts...)
	RegisterProviderServer(grpcServer, s.provider)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("failed to serve provider plugin server: %s", err)
		}
	}()

	closer := s.createCloser(grpcServer)

	resp, err := s.pluginService.Register(
		context.TODO(),
		&pluginservicev1.PluginRegistrationRequest{
			PluginId:   s.pluginID,
			PluginType: pluginservicev1.PluginType_PLUGIN_TYPE_PROVIDER,
			// Process IDs are sufficient for plugin instance IDs,
			// in the future we may want to allow for plugins that run in
			// containers.
			InstanceId:       strconv.Itoa(os.Getpid()),
			ProtocolVersions: []string{protocolVersion},
			Port:             int32(s.tcpPort),
			Metadata:         s.pluginMetadata,
			UnixSocket:       s.unixSocket,
		},
	)
	if err != nil {
		return closer, err
	}

	if !resp.Success {
		return closer, fmt.Errorf("failed to register plugin with host service: %s", resp.Message)
	}

	s.hostInfoContainer.SetID(resp.HostId)

	return closer, nil
}

func (s *Server) createCloser(baseServer *grpc.Server) func() {
	return func() {
		if s.listener != nil {
			err := s.listener.Close()
			if err != nil {
				log.Printf("failed to close listener for provider plugin server: %s", err)
			}
			s.deregisterPlugin()
			baseServer.Stop()
		}
	}
}

func (s *Server) deregisterPlugin() {
	resp, err := s.pluginService.Deregister(
		context.TODO(),
		&pluginservicev1.PluginDeregistrationRequest{
			PluginType: pluginservicev1.PluginType_PLUGIN_TYPE_PROVIDER,
			InstanceId: strconv.Itoa(os.Getpid()),
			HostId:     s.hostInfoContainer.GetID(),
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to deregister plugin with host service: %s\n", err)
	} else if !resp.Success {
		fmt.Fprintf(os.Stderr, "failed to deregister plugin with host service: %s\n", resp.Message)
	}
}

func (s *Server) createListener() (net.Listener, error) {
	return utils.CreateListener(&utils.ListenerConfig{
		Listener:   s.listener,
		UnixSocket: s.unixSocket,
		TCPPort:    s.tcpPort,
	})
}
