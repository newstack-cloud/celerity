package seed

import (
	"encoding/json"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type HandlerScopeTestSuite struct {
	suite.Suite
}

func TestHandlerScopeTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerScopeTestSuite))
}

func (s *HandlerScopeTestSuite) scopeOf(yamlContent string) map[string][]string {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err)

	payload, err := HandlerScopeJSON(bp)
	s.Require().NoError(err)

	scope := map[string][]string{}
	if payload != "" {
		s.Require().NoError(json.Unmarshal([]byte(payload), &scope))
	}
	return scope
}

// Keyed by handler id, because that is what reaches the function as
// CELERITY_HANDLER_ID and so the only thing it can identify itself by.
func (s *HandlerScopeTestSuite) Test_scope_is_keyed_by_handler_id() {
	scope := s.scopeOf(`
version: 2025-11-02
resources:
  statsAggregate:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.resource.ref: "appCache"
    spec:
      handlerName: scheduledAggregate
      handler: app-module.StatsController.scheduledAggregate
      runtime: nodejs24.x
`)

	s.Equal(
		map[string][]string{
			"app-module.StatsController.scheduledAggregate": {"appCache"},
		},
		scope,
	)
}

// The extractor derives refs by walking the injection graph, so a handler that
// reaches a database only through a service still declares it. Several refs
// arrive comma-joined, because annotations are string scalars.
func (s *HandlerScopeTestSuite) Test_multiple_refs_are_split() {
	scope := s.scopeOf(`
version: 2025-11-02
resources:
  auditCreate:
    type: "celerity/handler"
    metadata:
      annotations:
        celerity.handler.resource.ref: "auditDatabase,appConfig"
    spec:
      handlerName: createEntry
      handler: app-module.AuditController.createEntry
      runtime: nodejs24.x
`)

	s.Equal(
		[]string{"appConfig", "auditDatabase"},
		scope["app-module.AuditController.createEntry"],
	)
}

// The case the whole file exists for. A handler that reaches nothing gets an
// entry with an empty list, which is what stops it initialising every resource
// the application happens to declare. An empty list and a missing entry mean
// different things, since a missing entry falls back to the whole
// application.
func (s *HandlerScopeTestSuite) Test_handler_reaching_nothing_gets_an_empty_list() {
	scope := s.scopeOf(`
version: 2025-11-02
resources:
  health:
    type: "celerity/handler"
    spec:
      handlerName: check
      handler: app-module.HealthController.check
      runtime: nodejs24.x
`)

	refs, present := scope["app-module.HealthController.check"]
	s.True(present, "a handler with no refs must still appear, or it falls back to everything")
	s.Empty(refs)
}

// Only handlers carry scope; the resources they point at are described by the
// resource links file instead.
func (s *HandlerScopeTestSuite) Test_non_handler_resources_are_ignored() {
	scope := s.scopeOf(`
version: 2025-11-02
resources:
  appCache:
    type: "celerity/cache"
    spec:
      name: sessions
`)

	s.Empty(scope)
}
