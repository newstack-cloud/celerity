package seed

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/bluelink/libs/blueprint/substitutions"
)

// HandlerScopeFilename is the file the CLI writes so a handler can resolve
// which of the application's resources it actually uses.
//
// The resource links file describes what each resource is, which is a property
// of the application and identical for every handler. This file records which
// of those a given handler touches, which is a property of the handler.
//
// One bundle serves every function, so the resource links file necessarily
// describes the whole application. With nothing else to go on, the SDK's
// resource layers took it as their scope and every handler initialised every
// resource type present anywhere in the app. A handler using nothing but its own
// config still resolved the internal resources namespace, and failed at cold
// start on `Config namespace "resources" not registered`.
const HandlerScopeFilename = "handler-scope.json"

// The per-handler resource list the metadata extractor derives by walking the
// dependency injection graph, including through services a handler only reaches
// indirectly.
const resourceRefAnnotationKey = "celerity.handler.resource.ref"

// HandlerScopeJSON maps each handler's id to the resources it uses.
//
// Keyed by handler id, the value of spec.handler such as
// "app-module.StatsController.check", because that is what reaches the running
// function as CELERITY_HANDLER_ID and so the only thing a handler can identify
// itself by at cold start.
//
// An entry with an empty list is meaningful and is written, recording that the
// handler reaches nothing and so must not initialise anything. That differs from
// having no entry at all, which happens for an older bundle or a handler whose
// id could not be determined. The SDK treats a missing entry as the whole
// application being in scope, degrading to the behaviour that existed before
// this file rather than to a handler that silently reaches nothing.
func HandlerScopeJSON(bp *schema.Blueprint) (string, error) {
	if bp == nil || bp.Resources == nil {
		return "", nil
	}

	scope := map[string][]string{}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != "celerity/handler" {
			continue
		}

		handlerID := extractSpecStringField(resource, "handler")
		if handlerID == "" {
			continue
		}

		refs := annotationStringList(resource, resourceRefAnnotationKey)
		sort.Strings(refs)
		scope[handlerID] = refs
	}

	if len(scope) == 0 {
		return "", nil
	}

	data, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Annotations are string scalars, so the merger joins a list with commas when it
// writes one (see formatAnnotationValue) and this reverses that. An empty slice
// rather than nil is returned so an absent annotation serialises as [] and keeps
// the "reaches nothing" case distinguishable from an absent handler.
func annotationStringList(resource *schema.Resource, key string) []string {
	if resource.Metadata == nil || resource.Metadata.Annotations == nil {
		return []string{}
	}

	node, ok := resource.Metadata.Annotations.Values[key]
	if !ok || node == nil {
		return []string{}
	}

	joined, err := substitutions.SubstitutionsToString("", node)
	if err != nil || joined == "" {
		return []string{}
	}

	refs := []string{}
	for _, part := range strings.Split(joined, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	return refs
}
