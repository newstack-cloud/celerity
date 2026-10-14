package preprocess

import (
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"go.uber.org/zap"
)

// resourceUseLabelPrefix prefixes the label that wires a handler to a resource
// it reaches.
//
// One key per resource rather than one shared key, because a link selector
// matches on any single label pair. A shared key with different values would
// match nothing, and a shared key with a shared value would reach every
// resource in the application rather than the ones the handler uses.
const resourceUseLabelPrefix = "celerity.resource.use."

// linkableResourceTypes are the resource types a handler can hold an outbound
// link to.
//
// A handler reaching an API or another handler is not a data-access link and
// has no transformer case, so a reference to one is a mistake in the
// application rather than something to wire.
var linkableResourceTypes = map[string]bool{
	"celerity/bucket":      true,
	"celerity/cache":       true,
	"celerity/datastore":   true,
	"celerity/queue":       true,
	"celerity/sqlDatabase": true,
	"celerity/topic":       true,
}

// Wires each handler to the resources it reaches.
//
// The direction is the opposite of API and consumer linking. There the API owns
// the selector and the handlers carry the label, while here the handler owns
// the selector and the resource carries the label, because the handler is the
// linked-from side of a data-access link and nothing on the resource records
// which handlers use it.
//
// The labels are what a transformer turns into the handler's resource layers
// and its IAM grants, so a resource left unlabelled is one the handler has no
// permission to reach.
func linkHandlersToResources(bp *schema.Blueprint, manifest *HandlerManifest, logger *zap.Logger) {
	if bp.Resources == nil {
		return
	}

	for _, refs := range handlerResourceRefs(manifest) {
		handler, ok := bp.Resources.Values[refs.handlerName]
		if !ok {
			continue
		}
		for _, resourceName := range refs.resourceNames {
			linkHandlerToResource(bp, handler, refs.handlerName, resourceName, logger)
		}
	}
}

func linkHandlerToResource(
	bp *schema.Blueprint,
	handler *schema.Resource,
	handlerName string,
	resourceName string,
	logger *zap.Logger,
) {
	target, ok := bp.Resources.Values[resourceName]
	if !ok {
		// A reference to something the blueprint does not declare. Skipped
		// rather than refused, because extraction reads names out of source and
		// a renamed resource should not stop a dev session starting.
		logger.Warn("handler references a resource the blueprint does not declare",
			zap.String("handler", handlerName),
			zap.String("resource", resourceName),
		)
		return
	}
	if target.Type == nil || !linkableResourceTypes[target.Type.Value] {
		logger.Warn("handler references a resource it cannot link to",
			zap.String("handler", handlerName),
			zap.String("resource", resourceName),
		)
		return
	}

	key := resourceUseLabelPrefix + resourceName
	addLinkSelectorLabel(handler, key, resourceName)
	addResourceLabel(target, key, resourceName)
}

type handlerResourceRef struct {
	handlerName   string
	resourceNames []string
}

// Reads the resource references off every handler in the manifest, guards
// included, since a guard runs before the handler it protects and reaches
// resources of its own.
func handlerResourceRefs(manifest *HandlerManifest) []handlerResourceRef {
	var refs []handlerResourceRef

	add := func(resourceName string, annotations map[string]any) {
		names := resourceRefsFrom(annotations)
		if len(names) > 0 {
			refs = append(refs, handlerResourceRef{handlerName: resourceName, resourceNames: names})
		}
	}

	for _, h := range manifest.Handlers {
		add(h.ResourceName, h.Annotations)
	}
	for _, h := range manifest.FunctionHandlers {
		add(h.ResourceName, h.Annotations)
	}
	for _, h := range manifest.GuardHandlers {
		add(h.ResourceName, h.Annotations)
	}

	return refs
}

// Both []string and []any are accepted because the manifest is decoded into
// map[string]any, so a JSON array of strings arrives as []any whatever the
// extraction CLI wrote.
func resourceRefsFrom(annotations map[string]any) []string {
	raw, ok := annotations[resourceRefAnnotation]
	if !ok {
		return nil
	}

	switch refs := raw.(type) {
	case []string:
		return refs
	case []any:
		names := make([]string, 0, len(refs))
		for _, ref := range refs {
			if name, ok := ref.(string); ok && name != "" {
				names = append(names, name)
			}
		}
		return names
	case string:
		// A single reference written unwrapped.
		if refs != "" {
			return []string{refs}
		}
	}
	return nil
}

func addLinkSelectorLabel(resource *schema.Resource, key, value string) {
	if resource.LinkSelector == nil {
		resource.LinkSelector = &schema.LinkSelector{}
	}
	if resource.LinkSelector.ByLabel == nil {
		resource.LinkSelector.ByLabel = &schema.StringMap{}
	}
	if resource.LinkSelector.ByLabel.Values == nil {
		resource.LinkSelector.ByLabel.Values = map[string]string{}
	}
	resource.LinkSelector.ByLabel.Values[key] = value
}

func addResourceLabel(resource *schema.Resource, key, value string) {
	if resource.Metadata == nil {
		resource.Metadata = &schema.Metadata{}
	}
	if resource.Metadata.Labels == nil {
		resource.Metadata.Labels = &schema.StringMap{}
	}
	if resource.Metadata.Labels.Values == nil {
		resource.Metadata.Labels.Values = map[string]string{}
	}
	resource.Metadata.Labels.Values[key] = value
}
