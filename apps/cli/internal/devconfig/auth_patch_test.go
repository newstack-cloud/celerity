package devconfig

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type AuthPatchTestSuite struct {
	suite.Suite
}

func TestAuthPatchTestSuite(t *testing.T) {
	suite.Run(t, new(AuthPatchTestSuite))
}

func (s *AuthPatchTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *AuthPatchTestSuite) Test_patches_jwt_issuer_on_api_resource() {
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
          myJwtGuard:
            type: jwt
            issuer: "https://auth.example.com"
            audience: "my-app"
`)
	PatchJWTIssuer(bp, 0)

	apiResource := bp.Resources.Values["myApi"]
	guard := apiResource.Spec.Fields["auth"].Fields["guards"].Fields["myJwtGuard"]
	issuer := core.StringValue(guard.Fields["issuer"])
	s.Assert().Equal("http://host.docker.internal:9099", issuer)
}

func (s *AuthPatchTestSuite) Test_patches_jwt_issuer_with_port_offset() {
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
          myJwtGuard:
            type: jwt
            issuer: "https://auth.example.com"
`)
	PatchJWTIssuer(bp, 100)

	apiResource := bp.Resources.Values["myApi"]
	guard := apiResource.Spec.Fields["auth"].Fields["guards"].Fields["myJwtGuard"]
	issuer := core.StringValue(guard.Fields["issuer"])
	s.Assert().Equal("http://host.docker.internal:9199", issuer)
}

func (s *AuthPatchTestSuite) Test_skips_non_jwt_guards() {
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
          myCustomGuard:
            type: custom
            issuer: "https://auth.example.com"
`)
	PatchJWTIssuer(bp, 0)

	apiResource := bp.Resources.Values["myApi"]
	guard := apiResource.Spec.Fields["auth"].Fields["guards"].Fields["myCustomGuard"]
	issuer := core.StringValue(guard.Fields["issuer"])
	s.Assert().Equal("https://auth.example.com", issuer)
}

func (s *AuthPatchTestSuite) Test_skips_non_api_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myQueue:
    type: "celerity/queue"
    spec:
      name: events
`)
	// Should not panic.
	PatchJWTIssuer(bp, 0)
}

func (s *AuthPatchTestSuite) Test_handles_nil_resources() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	PatchJWTIssuer(bp, 0)
}

func (s *AuthPatchTestSuite) Test_handles_api_without_auth() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols:
        - http
`)
	PatchJWTIssuer(bp, 0)
}
