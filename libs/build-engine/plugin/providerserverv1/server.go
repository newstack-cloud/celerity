package providerserverv1

import (
	context "context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/two-hundred/celerity/libs/build-engine/plugin/pluginservice"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// The protocol version that is used during the handshake to ensure the plugin
	// is compatible with the host service.
	protocolVersion = 1
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

// Server is a plugin server.
type Server struct {
	pluginID       string
	debug          bool
	unixSocket     string
	tcpPort        int
	provider       ProviderServer
	serviceFactory func() (pluginservice.ServiceClient, func(), error)
}

func NewServer(
	pluginID string,
	provider ProviderServer,
	serviceFactory func() (pluginservice.ServiceClient, func(), error),
	opts ...ServerOption,
) *Server {
	server := &Server{
		pluginID:       pluginID,
		provider:       provider,
		serviceFactory: serviceFactory,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *Server) Serve() error {
	listener, err := s.createListener()
	if err != nil {
		return err
	}

	// If the TCP port is not set and a unix socket is not provided,
	// get the dynamically assigned port.
	if s.tcpPort == 0 && s.unixSocket == "" {
		s.tcpPort = listener.Addr().(*net.TCPAddr).Port
	}

	opts := []grpc.ServerOption{
		grpc.Creds(insecure.NewCredentials()),
	}

	grpcServer := grpc.NewServer(opts...)
	RegisterProviderServer(grpcServer, s.provider)

	go grpcServer.Serve(listener)

	service, closeServiceConn, err := s.serviceFactory()
	if err != nil {
		return err
	}
	resp, err := service.Register(
		context.TODO(),
		&pluginservice.PluginRegistrationRequest{
			PluginId: s.pluginID,
			// Process IDs are sufficient for plugin instance IDs,
			// in the future we may want to allow for plugins that run in
			// containers.
			InstanceId:      strconv.Itoa(os.Getpid()),
			ProtocolVersion: protocolVersion,
			Port:            int32(s.tcpPort),
			UnixSocket:      s.unixSocket,
		},
	)
	if err != nil {
		return err
	}
	closeServiceConn()

	if !resp.Success {
		return fmt.Errorf("failed to register plugin with host service: %s", resp.Message)
	}

	s.waitForShutdown()

	return nil
}

func (s *Server) waitForShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(
		c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	)

	<-c
	service, closeServiceConn, err := s.serviceFactory()
	if err != nil {
		// todo: use logger instead of fmt.Fprintf
		fmt.Fprintf(os.Stderr, "failed to create service client: %s\n", err)
		return
	}

	resp, err := service.Deregister(context.TODO(), &pluginservice.PluginDeregistrationRequest{
		PluginId:   s.pluginID,
		InstanceId: strconv.Itoa(os.Getpid()),
	})
	if err != nil {
		// todo: use logger instead of fmt.Fprintf
		fmt.Fprintf(os.Stderr, "failed to deregister plugin with host service: %s\n", err)
	} else if !resp.Success {
		fmt.Fprintf(os.Stderr, "failed to deregister plugin with host service: %s\n", resp.Message)
	}

	closeServiceConn()
}

func (s *Server) createListener() (net.Listener, error) {
	if s.unixSocket != "" {
		if err := os.Remove(s.unixSocket); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		return net.Listen("unix", s.unixSocket)
	}

	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.tcpPort))
}
