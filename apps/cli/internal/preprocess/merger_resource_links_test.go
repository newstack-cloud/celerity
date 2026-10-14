package preprocess

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MergerResourceLinksTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func (s *MergerResourceLinksTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *MergerResourceLinksTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

const dataResourcesBlueprint = `
version: 2025-11-02
resources:
  api:
    type: "celerity/api"
    spec:
      name: my-api
  usersDatastore:
    type: "celerity/datastore"
    spec:
      name: users
  auditDatabase:
    type: "celerity/sqlDatabase"
    spec:
      engine: postgres
  filesBucket:
    type: "celerity/bucket"
    spec:
      name: files
`

func (s *MergerResourceLinksTestSuite) manifestWithRefs(refs ...any) *HandlerManifest {
	annotations := map[string]any{
		"celerity.handler.http.method": "GET",
		"celerity.handler.http.path":   "/users",
	}
	if refs != nil {
		annotations[resourceRefAnnotation] = refs
	}

	return &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "usersHandler",
				ClassName:    "UsersController",
				MethodName:   "list",
				SourceFile:   "users.ts",
				HandlerType:  "http",
				Annotations:  annotations,
				Spec: HandlerSpec{
					HandlerName:  "Users-list",
					CodeLocation: "users.ts",
					Handler:      "UsersController.list",
				},
			},
		},
	}
}

func (s *MergerResourceLinksTestSuite) selectorLabels(
	result *schema.Blueprint,
	name string,
) map[string]string {
	resource, ok := result.Resources.Values[name]
	s.Require().True(ok, "expected resource %s", name)
	if resource.LinkSelector == nil || resource.LinkSelector.ByLabel == nil {
		return map[string]string{}
	}
	return resource.LinkSelector.ByLabel.Values
}

func (s *MergerResourceLinksTestSuite) labels(
	result *schema.Blueprint,
	name string,
) map[string]string {
	resource, ok := result.Resources.Values[name]
	s.Require().True(ok, "expected resource %s", name)
	if resource.Metadata == nil || resource.Metadata.Labels == nil {
		return map[string]string{}
	}
	return resource.Metadata.Labels.Values
}

// The handler is the linked-from resource for data access, so the selector
// belongs on the handler and the matching label on the resource. This is the
// opposite direction to API and consumer linking.
func (s *MergerResourceLinksTestSuite) Test_selector_goes_on_the_handler_and_label_on_the_resource() {
	bp := s.loadBlueprint(dataResourcesBlueprint)

	result, err := Merge(bp, s.manifestWithRefs("usersDatastore"), s.logger)
	s.Require().NoError(err)

	labelKey := resourceUseLabelPrefix + "usersDatastore"
	s.Assert().Equal("usersDatastore", s.selectorLabels(result, "usersHandler")[labelKey])
	s.Assert().Equal("usersDatastore", s.labels(result, "usersDatastore")[labelKey])
}

// byLabel matches on any single pair, so a distinct key per resource is what
// lets one handler reach several resources without reaching all of them.
func (s *MergerResourceLinksTestSuite) Test_several_refs_produce_a_key_per_resource() {
	bp := s.loadBlueprint(dataResourcesBlueprint)

	result, err := Merge(
		bp,
		s.manifestWithRefs("usersDatastore", "auditDatabase"),
		s.logger,
	)
	s.Require().NoError(err)

	selector := s.selectorLabels(result, "usersHandler")
	s.Assert().Equal("usersDatastore", selector[resourceUseLabelPrefix+"usersDatastore"])
	s.Assert().Equal("auditDatabase", selector[resourceUseLabelPrefix+"auditDatabase"])

	// The unreferenced resource stays unlabelled, so the handler cannot reach it.
	s.Assert().NotContains(
		s.labels(result, "filesBucket"),
		resourceUseLabelPrefix+"filesBucket",
	)
}

func (s *MergerResourceLinksTestSuite) Test_no_refs_leaves_the_handler_selector_untouched() {
	bp := s.loadBlueprint(dataResourcesBlueprint)

	result, err := Merge(bp, s.manifestWithRefs(), s.logger)
	s.Require().NoError(err)

	for key := range s.selectorLabels(result, "usersHandler") {
		s.Assert().NotContains(key, resourceUseLabelPrefix)
	}
}

func (s *MergerResourceLinksTestSuite) Test_unknown_resource_ref_is_skipped() {
	bp := s.loadBlueprint(dataResourcesBlueprint)

	result, err := Merge(
		bp,
		s.manifestWithRefs("usersDatastore", "notARealResource"),
		s.logger,
	)
	s.Require().NoError(err)

	selector := s.selectorLabels(result, "usersHandler")
	s.Assert().Equal("usersDatastore", selector[resourceUseLabelPrefix+"usersDatastore"])
	s.Assert().NotContains(selector, resourceUseLabelPrefix+"notARealResource")
}

// A ref to something a handler cannot hold an outbound link to is a mistake, and
// wiring it would produce a link the transformer has no case for.
func (s *MergerResourceLinksTestSuite) Test_ref_to_an_unlinkable_type_is_skipped() {
	bp := s.loadBlueprint(dataResourcesBlueprint)

	result, err := Merge(bp, s.manifestWithRefs("api"), s.logger)
	s.Require().NoError(err)

	s.Assert().NotContains(
		s.selectorLabels(result, "usersHandler"),
		resourceUseLabelPrefix+"api",
	)
	s.Assert().NotContains(s.labels(result, "api"), resourceUseLabelPrefix+"api")
}

// JSON decoding into map[string]any yields []any, which is what the annotation
// actually arrives as from the extraction CLI.
func (s *MergerResourceLinksTestSuite) Test_refs_arriving_as_any_slice() {
	bp := s.loadBlueprint(dataResourcesBlueprint)
	manifest := s.manifestWithRefs()
	manifest.Handlers[0].Annotations[resourceRefAnnotation] = []any{"usersDatastore"}

	result, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	s.Assert().Equal(
		"usersDatastore",
		s.selectorLabels(result, "usersHandler")[resourceUseLabelPrefix+"usersDatastore"],
	)
}

func (s *MergerResourceLinksTestSuite) Test_preserves_an_existing_link_selector() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersHandler:
    type: "celerity/handler"
    linkSelector:
      byLabel:
        application: orders
    spec:
      handlerName: Users-list
  usersDatastore:
    type: "celerity/datastore"
    spec:
      name: users
`)

	result, err := Merge(bp, s.manifestWithRefs("usersDatastore"), s.logger)
	s.Require().NoError(err)

	selector := s.selectorLabels(result, "usersHandler")
	s.Assert().Equal("orders", selector["application"])
	s.Assert().Equal("usersDatastore", selector[resourceUseLabelPrefix+"usersDatastore"])
}

func (s *MergerResourceLinksTestSuite) Test_guard_handlers_are_wired_too() {
	bp := s.loadBlueprint(dataResourcesBlueprint)
	manifest := &HandlerManifest{
		Version: "1",
		GuardHandlers: []GuardHandler{
			{
				ResourceName: "adminGuard_check",
				GuardName:    "admin",
				SourceFile:   "admin-guard.ts",
				GuardType:    "class",
				ClassName:    "AdminGuard",
				Annotations: map[string]any{
					"celerity.handler.guard.custom": "admin",
					resourceRefAnnotation:           []any{"usersDatastore"},
				},
				Spec: HandlerSpec{
					HandlerName:  "AdminGuard-check",
					CodeLocation: "admin-guard.ts",
					Handler:      "AdminGuard.check",
				},
			},
		},
	}

	result, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	s.Assert().Equal(
		"usersDatastore",
		s.selectorLabels(result, "adminGuard_check")[resourceUseLabelPrefix+"usersDatastore"],
	)
}

func TestMergerResourceLinksTestSuite(t *testing.T) {
	suite.Run(t, new(MergerResourceLinksTestSuite))
}
