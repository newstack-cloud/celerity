package utils

import (
	"fmt"
	"net"
	"os"
)

// ListenerConfig is a configuration for creating a network listener.
// This is primarily useful for setting up gRPC servers for the plugin system.
type ListenerConfig struct {
	// A pre-determined listener to use.
	// If this is set, TCPPort and UnixSocket will be ignored.
	Listener net.Listener
	// UnixSocket is the path to the Unix socket
	// to connect to. If this is set, TCPPort will be ignored.
	UnixSocket string
	// TCPPort is the port to connect to.
	// If this is set and the intention is to use TCP, UnixSocket should be empty
	// and Listener should be nil.
	TCPPort int
}

// CreateListener creates a network listener based on the configuration provided.
// This is useful for setting up gRPC servers for the plugin system.
// The behaviour is as follows (in order of priority):
// If a listener is provided, it will be used. If a Unix socket is provided, it will
// be used. If a TCP port is provided, it will be used.
//
// For TCP connections, the listener will be created on the `127.0.0.1`
// loopback address.
// This is meant for inter-process communication on the same host,
// you can opt out by providing a pre-determined listener but do so carefully
// as the plugin system should only communicate between processes on the same host.
func CreateListener(config *ListenerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	if config.UnixSocket != "" {
		if err := os.Remove(config.UnixSocket); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		return net.Listen("unix", config.UnixSocket)
	}

	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.TCPPort))
}
