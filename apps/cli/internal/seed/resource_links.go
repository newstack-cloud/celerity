package seed

import (
	"encoding/json"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
)

// resourceLink describes a single resource in the CELERITY_RESOURCE_LINKS topology.
type resourceLink struct {
	Type      string `json:"type"`
	ConfigKey string `json:"configKey"`
}

// blueprintTypeToLinkType maps blueprint resource type strings to the link type
// names expected by the SDK's resource layers.
var blueprintTypeToLinkType = map[string]string{
	"celerity/datastore":   "datastore",
	"celerity/sqlDatabase": "sqlDatabase",
	"celerity/bucket":      "bucket",
	"celerity/queue":       "queue",
	"celerity/topic":       "topic",
	"celerity/config":      "config",
	"celerity/cache":       "cache",
}

// ResourceLinksJSON generates the CELERITY_RESOURCE_LINKS JSON string from
// a blueprint. This env var tells the SDK about the resource topology so
// each resource layer (datastore, bucket, queue, topic, cache, sql-database)
// can discover and initialise its resources.
//
// The output maps resource names to {type, configKey} objects:
//
//	{"usersDatastore": {"type":"datastore","configKey":"users"}, ...}
//
// The configKey is derived from spec.name (falling back to the resource name).
func ResourceLinksJSON(bp *schema.Blueprint) (string, error) {
	if bp.Resources == nil {
		return "", nil
	}

	links := map[string]resourceLink{}
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil {
			continue
		}

		linkType, ok := blueprintTypeToLinkType[resource.Type.Value]
		if !ok {
			continue
		}

		configKey := extractSpecStringField(resource, "name")
		if configKey == "" {
			configKey = name
		}

		links[name] = resourceLink{
			Type:      linkType,
			ConfigKey: configKey,
		}
	}

	if len(links) == 0 {
		return "", nil
	}

	data, err := json.Marshal(links)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
