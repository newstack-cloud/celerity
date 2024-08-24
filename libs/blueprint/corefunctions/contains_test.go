package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type ContainsFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&ContainsFunctionTestSuite{})

func (s *ContainsFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &functionRegistryMock{
			functions: map[string]provider.Function{},
			callStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *ContainsFunctionTestSuite) Test_string_contains_substring(c *C) {
	containsFunc := NewContainsFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "contains",
	})
	output, err := containsFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"This is a string that contains a substring",
				"contains",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(bool)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, true)
}

func (s *ContainsFunctionTestSuite) Test_array_contains_element_primitive(c *C) {
	containsFunc := NewContainsFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "contains",
	})
	output, err := containsFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]interface{}{1, 3, 5, 9, 10},
				9,
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(bool)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, true)
}

func (s *ContainsFunctionTestSuite) Test_array_contains_element_comparable(c *C) {
	containsFunc := NewContainsFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "contains",
	})
	output, err := containsFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]interface{}{comparableInt(1), comparableInt(5), comparableInt(9), comparableInt(10)},
				comparableInt(9),
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(bool)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, true)
}

func (s *ContainsFunctionTestSuite) Test_returns_func_error_for_invalid_input_string_search(c *C) {
	containsFunc := NewContainsFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "contains",
	})
	_, err := containsFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"https://example.com/config/2",
				// A string is expected here as the first argument "haystack" is a string
				// and the second argument "needle" should also be a string.
				785043,
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(
		funcErr.Message,
		Equals,
		"Invalid input type. Expected string for item to search for in a string search space, received int",
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "contains",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidInput)
}

func (s *ContainsFunctionTestSuite) Test_returns_func_error_for_invalid_input_search_space(c *C) {
	containsFunc := NewContainsFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "contains",
	})
	_, err := containsFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				// A string or an array is expected for the search space.
				struct{ Value string }{},
				"needle",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(
		funcErr.Message,
		Equals,
		"Invalid input type. Expected string or array for "+
			"search space, received struct { Value string }",
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "contains",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidInput)
}
