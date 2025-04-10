package pluginservicev1

import (
	"fmt"
	"slices"
	"strings"
	sync "sync"
)

// Manager is an interface that defines the methods for
// registering and deregistering plugins along with
// retrieving plugin instances to interact with.
// The caller is responsible for type assertions to derive
// the actual client interface based on the plugin type.
type Manager interface {
	// RegisterPlugin registers a plugin with the host system
	// that the manager represents.
	RegisterPlugin(info *PluginInstanceInfo) error
	// DeregisterPlugin deregisters a plugin from the host system
	// that the manager represents.
	DeregisterPlugin(pluginType PluginType, id string) error
	// GetPlugin retrieves a plugin instance based on the plugin type
	// and the plugin ID.
	GetPlugin(pluginType PluginType, id string) *PluginInstance
	// GetPlugins retrieves all plugin instances for a given plugin type.
	GetPlugins(pluginType PluginType) []*PluginInstance
}

type managerImpl struct {
	pluginTypeProtocolVersions map[PluginType]string
	pluginInstances            map[PluginType]map[string]*PluginInstance
	pluginFactory              PluginFactory
	mu                         sync.RWMutex
}

// NewManager creates a new instance of a plugin manager
// that enables the deploy engine to interact with plugins
// and backs the "pluginservice" gRPC service that plugins
// can register with.
func NewManager(protocolVersions map[PluginType]string, pluginFactory PluginFactory) Manager {
	return &managerImpl{
		pluginTypeProtocolVersions: protocolVersions,
		pluginInstances:            make(map[PluginType]map[string]*PluginInstance),
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

	if !slices.Contains(info.ProtocolVersions, hostProtocolVersion) {
		protocolVersionsString := strings.Join(info.ProtocolVersions, ", ")
		return fmt.Errorf(
			"plugin protocol versions %q are not supported, expected %s",
			protocolVersionsString,
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
		Info:      info,
		Client:    client,
		CloseConn: closeConn,
	}

	return nil
}

func (m *managerImpl) DeregisterPlugin(pluginType PluginType, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instancesForType, hasPluginType := m.pluginInstances[pluginType]
	if !hasPluginType {
		return fmt.Errorf("plugin type %d is not supported", pluginType)
	}

	delete(instancesForType, id)
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

func (m *managerImpl) GetPlugins(pluginType PluginType) []*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instancesForType, hasPluginType := m.pluginInstances[pluginType]
	if !hasPluginType {
		return []*PluginInstance{}
	}

	instances := make([]*PluginInstance, 0, len(instancesForType))
	for _, instance := range instancesForType {
		instances = append(instances, instance)
	}

	return instances
}

type PluginFactory func(*PluginInstanceInfo) (any, func(), error)

// PluginInstance represents an instance of a plugin
// that has been registered with the host system.
type PluginInstance struct {
	Info *PluginInstanceInfo
	// type assertions should be carried out by callers at runtime
	// to derive the actual client interface based on the plugin type.
	Client    any
	CloseConn func()
}

// PluginInstanceInfo represents the information about a plugin instance
// that is registered with the host system.
type PluginInstanceInfo struct {
	PluginType PluginType
	// ProtocolVersions contains the protocol versions that
	// the plugin supports.
	// Currently, the only supported protocol version is "1.0".
	ProtocolVersions []string
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
	// The ID of an instance of a plugin that has been loaded
	// by the host system.
	InstanceID     string
	TCPPort        int
	UnixSocketPath string
}
