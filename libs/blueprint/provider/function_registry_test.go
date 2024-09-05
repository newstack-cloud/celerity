package provider

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	. "gopkg.in/check.v1"
)

type FunctionRegistryTestSuite struct {
	funcRegistry       FunctionRegistry
	testSubstrFunction *testSubstrFunction
}

var _ = Suite(&FunctionRegistryTestSuite{})

func (s *FunctionRegistryTestSuite) SetUpTest(c *C) {
	testSubstrFunc := newTestSubstrFunction()

	providers := map[string]Provider{
		"test": &testProvider{
			functions: map[string]Function{
				"test_substr": newTestSubstrFunction(),
			},
			namespace: "test",
		},
	}

	s.testSubstrFunction = testSubstrFunc.(*testSubstrFunction)
	s.funcRegistry = NewFunctionRegistry(providers)
}

func (s *FunctionRegistryTestSuite) Test_call_function(c *C) {
	output, err := s.funcRegistry.Call(
		context.TODO(),
		"test_substr",
		&FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args: []any{
					"hello world",
					int64(6),
					int64(11),
				},
				callCtx: &functionCallContextMock{
					registry:  s.funcRegistry,
					callStack: function.NewStack(),
				},
			},
		},
	)
	c.Assert(err, IsNil)
	c.Assert(output.ResponseData, Equals, "world")
}

func (s *FunctionRegistryTestSuite) Test_get_definition(c *C) {
	output, err := s.funcRegistry.GetDefinition(
		context.TODO(),
		"test_substr",
		&FunctionGetDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.Definition, DeepEquals, s.testSubstrFunction.definition)

	// Second time should be cached and produce the same result.
	output, err = s.funcRegistry.GetDefinition(
		context.TODO(),
		"test_substr",
		&FunctionGetDefinitionInput{},
	)
	c.Assert(err, IsNil)
	c.Assert(output.Definition, DeepEquals, s.testSubstrFunction.definition)
}

func (s *FunctionRegistryTestSuite) Test_has_function(c *C) {
	hasFunc, err := s.funcRegistry.HasFunction(context.TODO(), "test_substr")
	c.Assert(err, IsNil)
	c.Assert(hasFunc, Equals, true)

	hasFunc, err = s.funcRegistry.HasFunction(context.TODO(), "test_substr_not_exist")
	c.Assert(err, IsNil)
	c.Assert(hasFunc, Equals, false)
}

func (s *FunctionRegistryTestSuite) Test_duplicate_function_conflict(c *C) {
	// Register a provider with a duplicate function name.
	s.funcRegistry.(*functionRegistryFromProviders).providers["test_duplicate"] = &testProvider{
		functions: map[string]Function{
			"test_substr": newTestSubstrFunction(),
		},
		namespace: "test_duplicate",
	}

	_, err := s.funcRegistry.Call(
		context.TODO(),
		"test_substr",
		&FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args: []any{
					"hello world",
					int64(6),
					int64(11),
				},
				callCtx: &functionCallContextMock{
					registry:  s.funcRegistry,
					callStack: function.NewStack(),
				},
			},
		},
	)
	c.Assert(err, NotNil)
	runErr, isRunErr := err.(*errors.RunError)
	c.Assert(isRunErr, Equals, true)
	c.Assert(runErr.ReasonCode, Equals, ErrorReasonCodeFunctionAlreadyProvided)
}
