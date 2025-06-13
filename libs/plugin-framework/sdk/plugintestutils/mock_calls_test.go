package plugintestutils

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type MockCallsSuite struct {
	suite.Suite
}

type serviceMock struct {
	MockCalls
}

func (m *serviceMock) SaveResource(arg1 string, arg2 int) {
	m.RegisterCall(arg1, arg2)
}

func (s *MockCallsSuite) Test_assertions_for_derived_method_name_call() {
	service := &serviceMock{}
	service.SaveResource("testArg1", 242)

	service.AssertCalled(&s.Suite, "SaveResource")
	service.AssertCalledWith(
		&s.Suite,
		"SaveResource",
		/* callIndex */ 0,
		"testArg1",
		242,
	)

	service.AssertNotCalled(&s.Suite, "DeleteResource")
}

func (s *MockCallsSuite) Test_assertions_for_named_call() {
	mockCalls := &MockCalls{}
	mockCalls.RegisterNamedCall("UpdateResource", "testArg1", 504)

	mockCalls.AssertCalled(&s.Suite, "UpdateResource")
	mockCalls.AssertCalledWith(
		&s.Suite,
		"UpdateResource",
		/* callIndex */ 0,
		"testArg1",
		504,
	)
	mockCalls.AssertNotCalled(&s.Suite, "CreateResource")
}

func TestMockCallsSuite(t *testing.T) {
	suite.Run(t, new(MockCallsSuite))
}
