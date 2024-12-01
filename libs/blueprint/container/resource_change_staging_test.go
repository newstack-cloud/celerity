package container

import (
	"context"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
)

type ResourceChangeStagerTestSuite struct {
	resourceChangeStager *defaultResourceChangeStager
	suite.Suite
}

func (s *ResourceChangeStagerTestSuite) SetupSuite() {
	s.resourceChangeStager = &defaultResourceChangeStager{}
}

func (s *ResourceChangeStagerTestSuite) Test_stage_changes_for_existing_resource() {
	changes, err := s.resourceChangeStager.StageChanges(
		context.Background(),
		s.resourceInfoFixture1(),
		&internal.ExampleComplexResource{},
		[]string{
			"resources.complexResource.spec.itemConfig.endpoints[2]",
			"resources.complexResource.spec.itemConfig.endpoints[4]",
			"resources.complexResource.metadata.annotations[\"test.annotation.v1\"]",
			"resources.complexResource.metadata.custom.url",
		},
		nil,
	)
	s.Require().NoError(err)

	err = cupaloy.Snapshot(internal.NormaliseResourceChanges(changes, false /* excludeResourceInfo */))
	s.Require().NoError(err)
}

func (s *ResourceChangeStagerTestSuite) Test_stage_changes_for_new_resource() {
	changes, err := s.resourceChangeStager.StageChanges(
		context.Background(),
		s.resourceInfoFixture2(),
		&internal.ExampleComplexResource{},
		[]string{
			"resources.complexResource.spec.itemConfig.endpoints[3]",
			"resources.complexResource.metadata.annotations[\"test.annotation.v1\"]",
			"resources.complexResource.metadata.custom.url",
		},
		nil,
	)
	s.Require().NoError(err)

	err = cupaloy.Snapshot(internal.NormaliseResourceChanges(changes, false /* excludeResourceInfo */))
	s.Require().NoError(err)
}

func (s *ResourceChangeStagerTestSuite) Test_does_not_produce_changes_for_fields_exceeding_max_depth() {
	changes, err := s.resourceChangeStager.StageChanges(
		context.Background(),
		s.resourceInfoFixture3(),
		&internal.ExampleComplexResource{},
		[]string{},
		nil,
	)
	s.Require().NoError(err)

	err = cupaloy.Snapshot(internal.NormaliseResourceChanges(changes, true /* excludeResourceInfo */))
	s.Require().NoError(err)
}

func TestResourceChangeStagerTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceChangeStagerTestSuite))
}
