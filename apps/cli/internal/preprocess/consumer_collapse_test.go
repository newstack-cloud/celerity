package preprocess

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ConsumerCollapseSuite struct {
	suite.Suite
}

func consumerHandler(consumer, method, route string, refs ...string) ClassHandler {
	annotations := map[string]any{
		"celerity.handler.consumer": true,
		consumerSourceAnnotation:    consumer,
		handlerTagAnnotation:        consumer + "::" + method,
	}
	if route != "" {
		annotations[consumerRouteAnnotation] = route
	}
	if len(refs) > 0 {
		entries := make([]any, 0, len(refs))
		for _, ref := range refs {
			entries = append(entries, ref)
		}
		annotations[resourceRefAnnotation] = entries
	}

	return ClassHandler{
		ResourceName: consumer + "_" + method,
		ClassName:    "UserEventsConsumer",
		MethodName:   method,
		SourceFile:   "/app/src/app-module.ts",
		HandlerType:  "consumer",
		Annotations:  annotations,
		Spec: HandlerSpec{
			HandlerName:  "UserEventsConsumer-" + method,
			CodeLocation: "./src",
			Handler:      "app-module.UserEventsConsumer." + method,
		},
	}
}

func (s *ConsumerCollapseSuite) routedManifest() *HandlerManifest {
	return &HandlerManifest{
		Handlers: []ClassHandler{
			consumerHandler("userEventsConsumer", "onUserCreated", "user.created", "auditDatabase"),
			consumerHandler("userEventsConsumer", "onUserUpdated", "user.updated", "auditDatabase"),
			consumerHandler("userEventsConsumer", "onUserDeleted", "user.deleted", "usersDatastore"),
			consumerHandler("userEventsConsumer", "onUnknownEvent", ""),
		},
	}
}

func (s *ConsumerCollapseSuite) Test_collapses_a_routed_consumer_to_one_resource() {
	// The defect: four resources became four functions on one queue, so the source
	// handed each message to whichever polled first and the route never applied.
	out := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())

	s.Require().Len(out.Handlers, 1)
	s.Assert().Equal("userEventsConsumer_handler", out.Handlers[0].ResourceName)
	s.Assert().Equal("consumer", out.Handlers[0].HandlerType)
}

func (s *ConsumerCollapseSuite) Test_does_not_collide_with_the_consumer_resource() {
	// The blueprint already declares celerity/consumer "userEventsConsumer".
	// Merging the handler onto that name turned the consumer into the handler and
	// gave it the handler's resource links, which failed the blueprint load with
	// "No abstract link definition found for link type: celerity/consumer ->
	// celerity/sqlDatabase".
	out := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())

	s.Assert().NotEqual("userEventsConsumer", out.Handlers[0].ResourceName)
	s.Assert().Equal("userEventsConsumer", out.Handlers[0].Annotations[consumerSourceAnnotation])
}

func (s *ConsumerCollapseSuite) Test_tags_the_collapsed_resource_with_the_consumer() {
	out := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())
	annotations := out.Handlers[0].Annotations

	// The tag stays the one way a function says what it serves; naming the
	// consumer is how it says it serves all of it.
	s.Assert().Equal("userEventsConsumer", annotations[handlerTagAnnotation])
	s.Assert().Equal("userEventsConsumer", annotations[consumerSourceAnnotation])
	s.Assert().NotContains(annotations, consumerRouteAnnotation)
}

func (s *ConsumerCollapseSuite) Test_handler_ref_addresses_the_class_not_a_method() {
	// The runtime resolves a handler id ahead of routing, so an id naming one
	// method would send the whole batch there and skip routing entirely.
	out := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())

	s.Assert().Equal("app-module.UserEventsConsumer", out.Handlers[0].Spec.Handler)
	s.Assert().Empty(out.Handlers[0].MethodName)
}

func (s *ConsumerCollapseSuite) Test_takes_the_union_of_resource_refs() {
	// One function runs every handler, so it needs what all of them reach.
	out := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())

	s.Assert().ElementsMatch(
		[]any{"auditDatabase", "usersDatastore"},
		out.Handlers[0].Annotations[resourceRefAnnotation],
	)
}

func (s *ConsumerCollapseSuite) Test_leaves_a_single_handler_consumer_alone() {
	// Nothing to route between, so it keeps the resource it had and the target
	// emits exactly what it emitted before.
	manifest := &HandlerManifest{
		Handlers: []ClassHandler{
			consumerHandler("taskConsumer", "handle", ""),
		},
	}

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Require().Len(out.Handlers, 1)
	s.Assert().Equal("taskConsumer_handle", out.Handlers[0].ResourceName)
	s.Assert().Equal("taskConsumer::handle", out.Handlers[0].Annotations[handlerTagAnnotation])
}

func (s *ConsumerCollapseSuite) Test_leaves_non_consumer_handlers_untouched() {
	manifest := &HandlerManifest{
		Handlers: []ClassHandler{
			{
				ResourceName: "usersController_listUsers",
				ClassName:    "UsersController",
				MethodName:   "listUsers",
				HandlerType:  "http",
				Annotations:  map[string]any{"celerity.handler.http": true},
				Spec:         HandlerSpec{Handler: "app-module.UsersController.listUsers"},
			},
			consumerHandler("userEventsConsumer", "onUserCreated", "user.created"),
			consumerHandler("userEventsConsumer", "onUserUpdated", "user.updated"),
		},
	}

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Require().Len(out.Handlers, 2)
	s.Assert().Equal("usersController_listUsers", out.Handlers[0].ResourceName)
	s.Assert().Equal("userEventsConsumer_handler", out.Handlers[1].ResourceName)
}

func (s *ConsumerCollapseSuite) Test_keeps_separate_consumers_separate() {
	manifest := &HandlerManifest{
		Handlers: []ClassHandler{
			consumerHandler("userEventsConsumer", "onUserCreated", "user.created"),
			consumerHandler("userEventsConsumer", "onUserUpdated", "user.updated"),
			consumerHandler("orderEventsConsumer", "onPlaced", "order.placed"),
			consumerHandler("orderEventsConsumer", "onShipped", "order.shipped"),
		},
	}

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Require().Len(out.Handlers, 2)
	s.Assert().Equal("userEventsConsumer_handler", out.Handlers[0].ResourceName)
	s.Assert().Equal("orderEventsConsumer_handler", out.Handlers[1].ResourceName)
}

func (s *ConsumerCollapseSuite) Test_is_idempotent() {
	// Called both in Merge and by the resolver, so collapsing twice must not
	// differ from collapsing once.
	once := CollapseRoutedConsumers(s.routedManifest(), zap.NewNop())
	twice := CollapseRoutedConsumers(once, zap.NewNop())

	s.Assert().Equal(once.Handlers, twice.Handlers)
}

func (s *ConsumerCollapseSuite) Test_takes_the_longest_timeout_in_the_group() {
	// A shorter value would cut short a handler whose author asked for longer.
	short, long := 10, 45
	manifest := s.routedManifest()
	manifest.Handlers[0].Spec.Timeout = &short
	manifest.Handlers[1].Spec.Timeout = &long

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Require().NotNil(out.Handlers[0].Spec.Timeout)
	s.Assert().Equal(45, *out.Handlers[0].Spec.Timeout)
}

func (s *ConsumerCollapseSuite) Test_keeps_an_annotation_the_whole_group_agrees_on() {
	manifest := s.routedManifest()
	for i := range manifest.Handlers {
		manifest.Handlers[i].Annotations["celerity.handler.vpc"] = "private"
	}

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Assert().Equal("private", out.Handlers[0].Annotations["celerity.handler.vpc"])
}

func (s *ConsumerCollapseSuite) Test_drops_an_annotation_the_group_disagrees_on() {
	// It described one handler, and one function cannot honour two values.
	manifest := s.routedManifest()
	manifest.Handlers[0].Annotations["celerity.handler.memory"] = "512"
	manifest.Handlers[1].Annotations["celerity.handler.memory"] = "1024"

	out := CollapseRoutedConsumers(manifest, zap.NewNop())

	s.Assert().NotContains(out.Handlers[0].Annotations, "celerity.handler.memory")
}

func (s *ConsumerCollapseSuite) Test_handles_a_nil_manifest() {
	s.Assert().Nil(CollapseRoutedConsumers(nil, zap.NewNop()))
}

func TestConsumerCollapseSuite(t *testing.T) {
	suite.Run(t, new(ConsumerCollapseSuite))
}
