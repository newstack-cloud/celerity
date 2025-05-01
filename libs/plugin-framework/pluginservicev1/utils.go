package pluginservicev1

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// PluginTypeFromString converts a string to a PluginType.
// If the string is not recognized, PLUGIN_TYPE_NONE is returned.
func PluginTypeFromString(typeString string) PluginType {
	switch typeString {
	case "provider":
		return PluginType_PLUGIN_TYPE_PROVIDER
	case "transformer":
		return PluginType_PLUGIN_TYPE_TRANSFORMER
	default:
		return PluginType_PLUGIN_TYPE_NONE
	}
}

func hostSupportsProtocolVersions(
	hostProtocolVersion string,
	pluginProtocolVersions []string,
) (bool, error) {
	hostVersionParts, err := extractProtocolVersionParts(hostProtocolVersion)
	if err != nil {
		return false, err
	}

	extractedPluginVersions := []protocolVersionParts{}
	for _, version := range pluginProtocolVersions {
		protocolVersion, err := extractProtocolVersionParts(version)
		if err != nil {
			return false, err
		}
		extractedPluginVersions = append(extractedPluginVersions, protocolVersion)
	}

	majorVersionIndex := slices.IndexFunc(
		extractedPluginVersions,
		func(v protocolVersionParts) bool {
			return v.major == hostVersionParts.major
		},
	)
	if majorVersionIndex == -1 {
		// Major version of the host is not supported by the plugin.
		return false, nil
	}

	versionWithSameMajor := extractedPluginVersions[majorVersionIndex]

	// The protocol versions are backward-compatible for the same major version.
	return versionWithSameMajor.minor <= hostVersionParts.minor, nil
}

type protocolVersionParts struct {
	major int
	minor int
}

func extractProtocolVersionParts(version string) (protocolVersionParts, error) {
	// Split the version string by '.' and return the first part as an integer.
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		majorVersion, err := strconv.Atoi(parts[0])
		if err != nil {
			return protocolVersionParts{}, err
		}

		minorVersion, err := strconv.Atoi(parts[1])
		if err != nil {
			return protocolVersionParts{}, err
		}

		return protocolVersionParts{
			major: majorVersion,
			minor: minorVersion,
		}, nil
	}

	return protocolVersionParts{}, fmt.Errorf("invalid version format: %s", version)
}
