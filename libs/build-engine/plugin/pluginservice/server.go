package pluginservice

import context "context"

const (
	// DefaultPort is the default port for the plugin service
	// gRPC server.
	DefaultPort = 43044
)

type pluginServiceServer struct {
	UnimplementedServiceServer
	manager Manager
}

func NewServiceServer(pluginManager Manager) ServiceServer {
	return &pluginServiceServer{
		manager: pluginManager,
	}
}

func (s *pluginServiceServer) Register(ctx context.Context, req *PluginRegistrationRequest) (*PluginRegistrationResponse, error) {
	s.manager.RegisterPlugin(&PluginInstanceInfo{
		PluginType:      req.PluginType,
		ProtocolVersion: req.ProtocolVersion,
		ID:              req.PluginId,
		InstanceID:      req.InstanceId,
		TCPPort:         int(req.Port),
		UnixSocketPath:  req.UnixSocket,
	})
	return nil, nil
}

func (s *pluginServiceServer) Deregister(ctx context.Context, req *PluginDeregistrationRequest) (*PluginDeregistrationResponse, error) {
	return nil, nil
}
