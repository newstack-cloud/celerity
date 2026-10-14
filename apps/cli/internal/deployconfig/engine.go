package deployconfig

import (
	"net"
	"net/url"

	"github.com/newstack-cloud/deploy-cli-sdk/config"
)

// IsRemoteEngine reports whether the deploy engine the CLI will talk to runs
// outside the user's local filesystem and so cannot read files the CLI has
// written to disk. Callers use this to decide whether filesystem paths must be
// replaced with remote URLs, for uploaded build manifests and artifact buckets.
//
// "Remote" requires both a tcp connect-protocol and a non-loopback
// endpoint host. A tcp connection to localhost/127.0.0.1/::1 is still
// a local engine for filesystem-sharing purposes.
func IsRemoteEngine(confProvider *config.Provider) bool {
	protocol, _ := confProvider.GetString("connectProtocol")
	if protocol != "tcp" {
		return false
	}
	endpoint, _ := confProvider.GetString("engineEndpoint")
	if endpoint == "" {
		return false
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	return !isLoopbackHost(u.Hostname())
}

func isLoopbackHost(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
