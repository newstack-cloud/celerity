package preprocess

import (
	"sort"

	"go.uber.org/zap"
)

const (
	// consumerRouteAnnotation carries the value of the consumer's routing field
	// that selects one handler, set by the extraction CLI from
	// @MessageHandler("<route>").
	consumerRouteAnnotation = "celerity.handler.consumer.route"
	// handlerTagAnnotation is what the deploy target stamps on the function as
	// CELERITY_HANDLER_TAG, and what the runtime addresses a handler by.
	handlerTagAnnotation = "celerity.handler.tag"
	// resourceRefAnnotation names the application resources a handler reaches,
	// which is what a deploy target turns into the handler's resource layers
	// and its IAM grants.
	//
	// Collapsing a routed consumer takes the union across the handlers it
	// replaces, since the single remaining function runs all of their code and
	// so needs every resource any of them reached.
	resourceRefAnnotation = "celerity.handler.resource.ref"
)

// CollapseRoutedConsumers replaces the per-method handler resources of a routed
// consumer with a single resource for the consumer as a whole.
//
// Extraction produces one handler resource per @MessageHandler, which a deploy
// target turns into one function each, and for a consumer that is wrong. Every
// one of those functions consumes the same queue and the source hands each
// message to whichever function polls first, so the declared routing key never
// applies and messages are processed by an arbitrary handler. Nothing reports
// it, because a handler that receives a message it was not routed cannot tell
// the difference.
//
// Collapsing here rather than in the deploy target keeps one handler resource to
// one function everywhere downstream. The runtime routes the batch in-process,
// which it can do because every handler is in the same bundle.
//
// The operation is idempotent, since a manifest that has already been collapsed
// has one handler per consumer and is returned unchanged.
func CollapseRoutedConsumers(manifest *HandlerManifest, logger *zap.Logger) *HandlerManifest {
	if manifest == nil {
		return nil
	}

	groups, order := groupConsumerHandlers(manifest.Handlers)
	if len(order) == 0 {
		return manifest
	}

	collapsed := make([]ClassHandler, 0, len(manifest.Handlers))
	done := map[string]bool{}

	for _, handler := range manifest.Handlers {
		source := consumerSourceOf(handler)
		group := groups[source]

		// A consumer with a single handler has nothing to route between, so it
		// keeps the resource it already had and the target keeps emitting exactly
		// what it emitted before.
		if source == "" || len(group) < 2 {
			collapsed = append(collapsed, handler)
			continue
		}

		if done[source] {
			continue
		}
		done[source] = true

		merged := collapseConsumerGroup(source, group)
		collapsed = append(collapsed, merged)

		if logger != nil {
			logger.Debug("collapsed routed consumer handlers",
				zap.String("consumer", source),
				zap.String("resource", merged.ResourceName),
				zap.Int("handlers", len(group)),
			)
		}
	}

	out := *manifest
	out.Handlers = collapsed
	return &out
}

// First-seen order is kept so the collapsed manifest is stable between runs.
func groupConsumerHandlers(handlers []ClassHandler) (map[string][]ClassHandler, []string) {
	groups := map[string][]ClassHandler{}
	order := []string{}

	for _, handler := range handlers {
		source := consumerSourceOf(handler)
		if source == "" {
			continue
		}
		if _, seen := groups[source]; !seen {
			order = append(order, source)
		}
		groups[source] = append(groups[source], handler)
	}
	return groups, order
}

// The handler tag stays the bare consumer name, which is what the runtime
// matches handlers against, so this only has to be a distinct resource name.
func collapsedResourceName(source string) string {
	return source + "_handler"
}

func consumerSourceOf(handler ClassHandler) string {
	if handler.HandlerType != "consumer" {
		return ""
	}
	source, _ := handler.Annotations[consumerSourceAnnotation].(string)
	return source
}

// Builds the single handler resource that stands for a whole consumer.
func collapseConsumerGroup(source string, group []ClassHandler) ClassHandler {
	first := group[0]

	merged := ClassHandler{
		// Suffixed rather than named after the consumer, because the blueprint
		// already has a celerity/consumer resource under that name and merging a
		// handler onto it would turn the consumer itself into the handler, taking
		// its resource links with it.
		ResourceName: collapsedResourceName(source),
		ClassName:    first.ClassName,
		// No single method, since this resource stands for every @MessageHandler
		// on the class and naming one of them would misreport what the function
		// runs.
		MethodName:  "",
		SourceFile:  first.SourceFile,
		HandlerType: "consumer",
		Annotations: collapsedAnnotations(source, group),
		Spec: HandlerSpec{
			HandlerName:  first.ClassName,
			CodeLocation: first.Spec.CodeLocation,
			// Addresses the consumer rather than a method. The runtime resolves a
			// handler id ahead of routing, so an id naming one method would
			// dispatch the whole batch to it and skip routing entirely.
			Handler: consumerHandlerRef(first),
			Timeout: maxTimeout(group),
		},
	}
	return merged
}

// Merges the group's annotations onto the consumer.
//
// The route is dropped, because it distinguished the resources that no longer
// exist and leaving it would declare that this function serves one route. The
// tag becomes the consumer's own name, which is how a function is marked as
// serving the whole consumer rather than one handler.
func collapsedAnnotations(source string, group []ClassHandler) map[string]any {
	annotations := map[string]any{}

	// Carry anything the whole group agrees on, so a per-consumer setting applied
	// to every handler survives. Anything the handlers disagree on describes one
	// handler rather than the consumer, and cannot be represented once collapsed.
	for key, value := range group[0].Annotations {
		if key == consumerRouteAnnotation || key == handlerTagAnnotation ||
			key == resourceRefAnnotation {
			continue
		}
		if agreesAcrossGroup(group, key, value) {
			annotations[key] = value
		}
	}

	annotations[consumerSourceAnnotation] = source
	annotations[handlerTagAnnotation] = source

	if refs := unionResourceRefs(group); len(refs) > 0 {
		annotations[resourceRefAnnotation] = refs
	}
	return annotations
}

func agreesAcrossGroup(group []ClassHandler, key string, value any) bool {
	first, ok := value.(string)
	if !ok {
		// Only scalars are compared; a structured annotation is left to the
		// explicit handling above rather than guessed at.
		if _, isBool := value.(bool); !isBool {
			return false
		}
	}

	for _, handler := range group[1:] {
		other, exists := handler.Annotations[key]
		if !exists {
			return false
		}
		if boolValue, isBool := value.(bool); isBool {
			if otherBool, ok := other.(bool); !ok || otherBool != boolValue {
				return false
			}
			continue
		}
		if otherString, ok := other.(string); !ok || otherString != first {
			return false
		}
	}
	return true
}

// Collects every resource any handler in the group uses. The function runs all
// of them, so granting only what one handler declared would leave the others
// unable to reach what they use.
func unionResourceRefs(group []ClassHandler) []any {
	seen := map[string]bool{}
	for _, handler := range group {
		for _, ref := range refsOf(handler) {
			seen[ref] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	refs := make([]any, 0, len(names))
	for _, name := range names {
		refs = append(refs, name)
	}
	return refs
}

func refsOf(handler ClassHandler) []string {
	raw, ok := handler.Annotations[resourceRefAnnotation]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []any:
		refs := make([]string, 0, len(value))
		for _, entry := range value {
			if name, ok := entry.(string); ok && name != "" {
				refs = append(refs, name)
			}
		}
		return refs
	case []string:
		return value
	case string:
		if value == "" {
			return nil
		}
		return []string{value}
	}
	return nil
}

// Reduces a handler reference to the class that owns it, so
// `app-module.UserEventsConsumer.onUserCreated` becomes
// `app-module.UserEventsConsumer`. The module segment is what the target reads
// to find the bundle entry point, and dropping the method keeps the reference
// from resolving to a single handler at runtime.
func consumerHandlerRef(handler ClassHandler) string {
	ref := handler.Spec.Handler
	if ref == "" {
		return ""
	}
	if handler.MethodName == "" {
		return ref
	}
	suffix := "." + handler.MethodName
	if len(ref) > len(suffix) && ref[len(ref)-len(suffix):] == suffix {
		return ref[:len(ref)-len(suffix)]
	}
	return ref
}

// The longest timeout any handler in the group asked for. One function serves
// all of them, so a shorter value would cut short a handler whose author had
// asked for longer.
func maxTimeout(group []ClassHandler) *int {
	var longest *int
	for _, handler := range group {
		if handler.Spec.Timeout == nil {
			continue
		}
		if longest == nil || *handler.Spec.Timeout > *longest {
			value := *handler.Spec.Timeout
			longest = &value
		}
	}
	return longest
}
