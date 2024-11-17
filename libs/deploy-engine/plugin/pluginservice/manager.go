package pluginservice

import (
	"fmt"
	sync "sync"
)

type Manager interface {
	RegisterPlugin(info *PluginInstanceInfo) error
	GetPlugin(pluginType PluginType, id string) *PluginInstance
}

type managerImpl struct {
	pluginTypeProtocolVersions map[PluginType]int32
	pluginInstances            map[PluginType]map[string]*PluginInstance
	pluginFactory              PluginFactory
	mu                         sync.RWMutex
}

// NewManager creates a new instance of a plugin manager
// that enables the deploy engine to interact with plugins
// and backs the "pluginservice" gRPC service that plugins
// can register with.
func NewManager(protocolVersions map[PluginType]int32, pluginFactory PluginFactory) Manager {
	return &managerImpl{
		pluginTypeProtocolVersions: protocolVersions,
		pluginFactory:              pluginFactory,
	}
}

func (m *managerImpl) RegisterPlugin(info *PluginInstanceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pluginInstances[info.PluginType] == nil {
		m.pluginInstances[info.PluginType] = make(map[string]*PluginInstance)
	}

	hostProtocolVersion, ok := m.pluginTypeProtocolVersions[info.PluginType]
	if !ok {
		return fmt.Errorf("plugin type %d is not supported", info.PluginType)
	}

	if info.ProtocolVersion != hostProtocolVersion {
		return fmt.Errorf(
			"plugin protocol version %d is not supported, expected %d",
			info.ProtocolVersion,
			hostProtocolVersion,
		)
	}

	_, hasPlugin := m.pluginInstances[info.PluginType][info.ID]
	if hasPlugin {
		return fmt.Errorf("plugin %s is already registered", info.ID)
	}

	client, closeConn, err := m.pluginFactory(info)
	if err != nil {
		return err
	}

	m.pluginInstances[info.PluginType][info.ID] = &PluginInstance{
		info:      info,
		client:    client,
		closeConn: closeConn,
	}

	return nil
}

func (m *managerImpl) GetPlugin(pluginType PluginType, id string) *PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.pluginInstances[pluginType] == nil {
		return nil
	}

	return m.pluginInstances[pluginType][id]
}

type PluginFactory func(*PluginInstanceInfo) (interface{}, func(), error)

type PluginInstance struct {
	info *PluginInstanceInfo
	// type assertions should be carried out by callers at runtime
	// to derive the actual client interface based on the plugin type.
	client    interface{}
	closeConn func()
}

type PluginInstanceInfo struct {
	PluginType      PluginType
	ProtocolVersion int32
	ID              string
	InstanceID      string
	TCPPort         int
	UnixSocketPath  string
}
