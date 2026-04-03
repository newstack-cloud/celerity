package preprocess

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MergerTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func (s *MergerTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *MergerTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *MergerTestSuite) Test_Merge_creates_new_handler_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  api:
    type: "celerity/api"
    spec:
      name: my-api
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "helloHandler",
				ClassName:    "HelloController",
				MethodName:   "handle",
				SourceFile:   "hello.ts",
				HandlerType:  "http",
				Annotations: map[string]any{
					"celerity.handler.http.method": "GET",
					"celerity.handler.http.path":   "/hello",
				},
				Spec: HandlerSpec{
					HandlerName:  "Test-Hello-v1",
					CodeLocation: "hello.ts",
					Handler:      "HelloController.handle",
				},
			},
		},
	}

	result, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handler, ok := result.Resources.Values["helloHandler"]
	s.Require().True(ok, "expected helloHandler resource to be created")
	s.Assert().Equal(resourceTypeHandler, handler.Type.Value)
	s.Assert().Equal("Test-Hello-v1", core.StringValue(handler.Spec.Fields["handlerName"]))
	s.Assert().Equal("hello.ts", core.StringValue(handler.Spec.Fields["codeLocation"]))
}

func (s *MergerTestSuite) Test_Merge_fills_missing_fields_on_existing_resource() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  helloHandler:
    type: "celerity/handler"
    spec:
      handlerName: Test-Hello-v1
      runtime: nodejs24.x
      timeout: 30
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "helloHandler",
				Annotations: map[string]any{
					"celerity.handler.http.method": "GET",
					"celerity.handler.http.path":   "/hello",
				},
				Spec: HandlerSpec{
					HandlerName:  "Test-Hello-v1",
					CodeLocation: "hello.ts",
					Handler:      "HelloController.handle",
				},
			},
		},
	}

	result, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handler := result.Resources.Values["helloHandler"]
	s.Require().NotNil(handler)

	// Existing spec value preserved.
	s.Assert().Equal("Test-Hello-v1", core.StringValue(handler.Spec.Fields["handlerName"]))
	// Extracted value fills in missing field.
	s.Assert().Equal("hello.ts", core.StringValue(handler.Spec.Fields["codeLocation"]))
	// Blueprint timeout preserved (infrastructure config takes precedence).
	s.Assert().Equal(30, core.IntValue(handler.Spec.Fields["timeout"]))
}

func (s *MergerTestSuite) Test_Merge_empty_blueprint_with_multiple_handlers() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "handler1",
				Annotations:  map[string]any{"celerity.handler.http.method": "GET"},
				Spec:         HandlerSpec{HandlerName: "H1", CodeLocation: "h1.ts", Handler: "H1.handle"},
			},
			{
				ResourceName: "handler2",
				Annotations:  map[string]any{"celerity.handler.http.method": "POST"},
				Spec:         HandlerSpec{HandlerName: "H2", CodeLocation: "h2.ts", Handler: "H2.create"},
			},
		},
		FunctionHandlers: []FunctionHandler{
			{
				ResourceName: "handler3",
				Annotations:  map[string]any{"celerity.handler.http.method": "DELETE"},
				Spec:         HandlerSpec{HandlerName: "H3", CodeLocation: "h3.ts", Handler: "deleteHandler"},
			},
		},
	}

	result, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)
	s.Assert().Len(result.Resources.Values, 3)
}

func (s *MergerTestSuite) Test_WriteMerged_yaml_format() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  hello:
    type: "celerity/handler"
    spec:
      handlerName: Hello
`)
	dir := s.T().TempDir()
	outPath, err := WriteMerged(bp, schema.YAMLSpecFormat, dir)
	s.Require().NoError(err)

	s.Assert().Equal(filepath.Join(dir, "merged.blueprint.yaml"), outPath)

	data, err := os.ReadFile(outPath)
	s.Require().NoError(err)
	s.Assert().NotEmpty(data)
}

func (s *MergerTestSuite) Test_WriteMerged_jsonc_format() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  hello:
    type: "celerity/handler"
    spec:
      handlerName: Hello
`)
	dir := s.T().TempDir()
	outPath, err := WriteMerged(bp, schema.JWCCSpecFormat, dir)
	s.Require().NoError(err)

	s.Assert().Equal(filepath.Join(dir, "merged.blueprint.jsonc"), outPath)

	data, err := os.ReadFile(outPath)
	s.Require().NoError(err)
	s.Assert().NotEmpty(data)
}

func (s *MergerTestSuite) Test_ManifestEqual_detects_new_handler() {
	m1 := &HandlerManifest{
		Handlers: []ClassHandler{{ResourceName: "a"}},
	}
	m2 := &HandlerManifest{
		Handlers: []ClassHandler{{ResourceName: "a"}, {ResourceName: "b"}},
	}
	s.Assert().False(m1.Equal(m2))
}

func (s *MergerTestSuite) Test_ManifestEqual_same_handlers() {
	m1 := &HandlerManifest{
		Handlers: []ClassHandler{
			{ResourceName: "a", Annotations: map[string]any{"method": "GET"}},
		},
	}
	m2 := &HandlerManifest{
		Handlers: []ClassHandler{
			{ResourceName: "a", Annotations: map[string]any{"method": "GET"}},
		},
	}
	s.Assert().True(m1.Equal(m2))
}

func (s *MergerTestSuite) Test_ManifestEqual_changed_annotation() {
	m1 := &HandlerManifest{
		Handlers: []ClassHandler{
			{ResourceName: "a", Annotations: map[string]any{"method": "GET"}},
		},
	}
	m2 := &HandlerManifest{
		Handlers: []ClassHandler{
			{ResourceName: "a", Annotations: map[string]any{"method": "POST"}},
		},
	}
	s.Assert().False(m1.Equal(m2))
}

func (s *MergerTestSuite) Test_Merge_links_consumer_handler_to_consumer_resource() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  orderEvents:
    type: "celerity/consumer"
    spec:
      sourceId: "celerity::topic::orders"
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "processOrder",
				ClassName:    "OrderProcessor",
				MethodName:   "handle",
				SourceFile:   "orders.ts",
				HandlerType:  "consumer",
				Annotations: map[string]any{
					"celerity.handler.consumer.source": "orderEvents",
				},
				Spec: HandlerSpec{
					HandlerName:  "processOrder",
					CodeLocation: "orders.ts",
					Handler:      "OrderProcessor.handle",
				},
			},
		},
	}

	merged, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handlerResource, ok := merged.Resources.Values["processOrder"]
	s.Require().True(ok, "expected processOrder handler resource")
	s.Require().NotNil(handlerResource.Metadata)
	s.Require().NotNil(handlerResource.Metadata.Labels)
	s.Assert().Contains(handlerResource.Metadata.Labels.Values, "sourceConsumer")

	consumerResource := merged.Resources.Values["orderEvents"]
	s.Require().NotNil(consumerResource.LinkSelector)
	s.Require().NotNil(consumerResource.LinkSelector.ByLabel)
	s.Assert().Equal("orderEvents", consumerResource.LinkSelector.ByLabel.Values["sourceConsumer"])
}

func (s *MergerTestSuite) Test_Merge_links_schedule_handler_to_schedule_resource() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  dailyCleanup:
    type: "celerity/schedule"
    spec:
      schedule: "rate(1d)"
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "cleanupHandler",
				ClassName:    "CleanupJob",
				MethodName:   "run",
				SourceFile:   "cleanup.ts",
				HandlerType:  "schedule",
				Annotations: map[string]any{
					"celerity.handler.schedule.source": "dailyCleanup",
				},
				Spec: HandlerSpec{
					HandlerName:  "cleanupHandler",
					CodeLocation: "cleanup.ts",
					Handler:      "CleanupJob.run",
				},
			},
		},
	}

	merged, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handlerResource := merged.Resources.Values["cleanupHandler"]
	s.Require().NotNil(handlerResource.Metadata)
	s.Require().NotNil(handlerResource.Metadata.Labels)
	s.Assert().Contains(handlerResource.Metadata.Labels.Values, "sourceSchedule")

	scheduleResource := merged.Resources.Values["dailyCleanup"]
	s.Require().NotNil(scheduleResource.LinkSelector)
}

func (s *MergerTestSuite) Test_Merge_handler_with_custom_guards_annotation() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
      auth:
        guards:
          jwtGuard:
            type: jwt
            issuer: "https://example.com"
`)
	manifest := &HandlerManifest{
		Version: "1",
		Handlers: []ClassHandler{
			{
				ResourceName: "getUsers",
				ClassName:    "UsersController",
				MethodName:   "getAll",
				SourceFile:   "users.ts",
				HandlerType:  "http",
				Annotations: map[string]any{
					"celerity.handler.http.method":  "GET",
					"celerity.handler.http.path":    "/users",
					"celerity.handler.guard.custom": []interface{}{"jwtGuard"},
				},
				Spec: HandlerSpec{
					HandlerName:  "getUsers",
					CodeLocation: "users.ts",
					Handler:      "UsersController.getAll",
				},
			},
		},
	}

	merged, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handler := merged.Resources.Values["getUsers"]
	s.Require().NotNil(handler)
	authNode := handler.Spec.Fields["auth"]
	if authNode != nil && authNode.Fields != nil {
		guardsNode := authNode.Fields["guards"]
		if guardsNode != nil && guardsNode.Scalar != nil {
			s.Assert().Equal("jwtGuard", core.StringValue(guardsNode))
		}
	}
}

func (s *MergerTestSuite) Test_Merge_with_function_handlers() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	manifest := &HandlerManifest{
		Version: "1",
		FunctionHandlers: []FunctionHandler{
			{
				ResourceName: "createUser",
				ExportName:   "createUser",
				SourceFile:   "users.ts",
				Annotations: map[string]any{
					"celerity.handler.http.method": "POST",
					"celerity.handler.http.path":   "/users",
				},
				Spec: HandlerSpec{
					HandlerName:  "createUser",
					CodeLocation: "users.ts",
					Handler:      "createUser",
				},
			},
		},
	}

	merged, err := Merge(bp, manifest, s.logger)
	s.Require().NoError(err)

	handler, ok := merged.Resources.Values["createUser"]
	s.Require().True(ok)
	s.Assert().Equal("celerity/handler", handler.Type.Value)
}

func (s *MergerTestSuite) Test_WriteMerged_creates_directory_if_needed() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	dir := s.T().TempDir()
	outputDir := filepath.Join(dir, "nested", "subdir")

	path, err := WriteMerged(bp, schema.YAMLSpecFormat, outputDir)
	s.Require().NoError(err)
	s.Assert().Contains(path, "merged.blueprint.yaml")

	_, err = os.Stat(path)
	s.Assert().NoError(err)
}

func TestMergerTestSuite(t *testing.T) {
	suite.Run(t, new(MergerTestSuite))
}
