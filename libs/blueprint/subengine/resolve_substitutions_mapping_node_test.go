package subengine

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type SubstitutionMappingNodeResolverTestSuite struct {
	SubResolverTestContainer
	suite.Suite
}

const (
	resolveInMappingNodeFixtureName = "resolve-in-mapping-node"
)

func (s *SubstitutionMappingNodeResolverTestSuite) SetupSuite() {
	s.populateSpecFixtureSchemas(
		map[string]string{
			resolveInMappingNodeFixtureName: "__testdata/sub-resolver/resolve-in-mapping-node-blueprint.yml",
		},
		&s.Suite,
	)
}

func (s *SubstitutionMappingNodeResolverTestSuite) SetupTest() {
	s.populateDependencies()
}

func TestSubstitutionMappingNodeResolverTestSuite(t *testing.T) {
	suite.Run(t, new(SubstitutionMappingNodeResolverTestSuite))
}
