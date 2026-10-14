package blueprint

import (
	"slices"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
)

// toolchainResourceKinds maps a blueprint resource type to the resource kind
// name a Celerity SDK links an implementation for.
//
// Only the kinds an SDK provides a client for. celerity/config is absent
// because configuration reaches a handler through the config store rather
// than through a resource layer, and the API, handler and VPC types describe
// the application rather than something it reaches.
var toolchainResourceKinds = map[string]string{
	"celerity/bucket":      "bucket",
	"celerity/cache":       "cache",
	"celerity/datastore":   "datastore",
	"celerity/queue":       "queue",
	"celerity/sqlDatabase": "sqlDatabase",
	"celerity/topic":       "topic",
}

// ResourceKinds returns the sorted, deduplicated resource kinds a blueprint
// declares.
//
// A compiled language cannot discover these at startup the way an interpreted
// one can, so the build has to link an implementation for each kind ahead of
// time. Deriving them from the blueprint is what keeps that out of the
// application's own source.
func ResourceKinds(bp *schema.Blueprint) []string {
	if bp == nil || bp.Resources == nil {
		return nil
	}

	seen := map[string]bool{}
	for _, resource := range bp.Resources.Values {
		if resource == nil || resource.Type == nil {
			continue
		}
		if kind, ok := toolchainResourceKinds[resource.Type.Value]; ok {
			seen[kind] = true
		}
	}
	return sortedKeysOf(seen)
}

// SQLEngines returns the sorted, deduplicated engines of a blueprint's SQL
// databases, so a driver is linked for each.
func SQLEngines(bp *schema.Blueprint) []string {
	if bp == nil || bp.Resources == nil {
		return nil
	}

	seen := map[string]bool{}
	for _, resource := range bp.Resources.Values {
		if resource == nil || resource.Type == nil {
			continue
		}
		if resource.Type.Value != "celerity/sqlDatabase" {
			continue
		}
		if engine := specStringField(resource, "engine"); engine != "" {
			seen[engine] = true
		}
	}
	return sortedKeysOf(seen)
}

// sortedKeysOf returns the set's keys in order, so a generated file and the
// command line that produced it reproduce between runs.
func sortedKeysOf(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
