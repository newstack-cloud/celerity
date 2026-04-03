package seed

import (
	"encoding/json"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/suite"
)

type ResourceLinksTestSuite struct {
	suite.Suite
}

func (s *ResourceLinksTestSuite) loadBlueprint(yamlContent string) *schema.Blueprint {
	bp, err := schema.LoadString(yamlContent, schema.YAMLSpecFormat)
	s.Require().NoError(err, "failed to load test blueprint")
	return bp
}

func (s *ResourceLinksTestSuite) Test_resource_links_for_all_resource_types() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  usersDatastore:
    type: "celerity/datastore"
    spec:
      name: users
  auditDb:
    type: "celerity/sqlDatabase"
    spec:
      name: audit
  filesBucket:
    type: "celerity/bucket"
    spec:
      name: files
  taskQueue:
    type: "celerity/queue"
    spec:
      name: taskQueue
  userEventsTopic:
    type: "celerity/topic"
    spec:
      name: userEvents
  appConfig:
    type: "celerity/config"
    spec:
      name: appConfig
  appCache:
    type: "celerity/cache"
    spec:
      name: appCache
`)
	jsonStr, err := ResourceLinksJSON(bp)
	s.Require().NoError(err)
	s.Require().NotEmpty(jsonStr)

	var links map[string]resourceLink
	s.Require().NoError(json.Unmarshal([]byte(jsonStr), &links))

	s.Assert().Equal("datastore", links["usersDatastore"].Type)
	s.Assert().Equal("users", links["usersDatastore"].ConfigKey)

	s.Assert().Equal("sqlDatabase", links["auditDb"].Type)
	s.Assert().Equal("audit", links["auditDb"].ConfigKey)

	s.Assert().Equal("bucket", links["filesBucket"].Type)
	s.Assert().Equal("files", links["filesBucket"].ConfigKey)

	s.Assert().Equal("queue", links["taskQueue"].Type)
	s.Assert().Equal("taskQueue", links["taskQueue"].ConfigKey)

	s.Assert().Equal("topic", links["userEventsTopic"].Type)
	s.Assert().Equal("userEvents", links["userEventsTopic"].ConfigKey)

	s.Assert().Equal("config", links["appConfig"].Type)
	s.Assert().Equal("appConfig", links["appConfig"].ConfigKey)

	s.Assert().Equal("cache", links["appCache"].Type)
	s.Assert().Equal("appCache", links["appCache"].ConfigKey)
}

func (s *ResourceLinksTestSuite) Test_resource_links_falls_back_to_resource_name() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myBucket:
    type: "celerity/bucket"
    spec:
      description: "no name field"
`)
	jsonStr, err := ResourceLinksJSON(bp)
	s.Require().NoError(err)

	var links map[string]resourceLink
	s.Require().NoError(json.Unmarshal([]byte(jsonStr), &links))

	s.Assert().Equal("myBucket", links["myBucket"].ConfigKey)
}

func (s *ResourceLinksTestSuite) Test_resource_links_ignores_non_linkable_types() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources:
  myApi:
    type: "celerity/api"
    spec:
      protocols: ["http"]
  myHandler:
    type: "celerity/handler"
    spec:
      handlerName: test
  myVpc:
    type: "celerity/vpc"
    spec:
      name: main
  mySchedule:
    type: "celerity/schedule"
    spec:
      schedule: "rate(5m)"
  myConsumer:
    type: "celerity/consumer"
    spec:
      sourceId: "celerity::queue::test"
`)
	jsonStr, err := ResourceLinksJSON(bp)
	s.Require().NoError(err)
	s.Assert().Empty(jsonStr)
}

func (s *ResourceLinksTestSuite) Test_resource_links_empty_blueprint() {
	bp := s.loadBlueprint(`
version: 2025-11-02
resources: {}
`)
	jsonStr, err := ResourceLinksJSON(bp)
	s.Require().NoError(err)
	s.Assert().Empty(jsonStr)
}

func TestResourceLinksTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceLinksTestSuite))
}
