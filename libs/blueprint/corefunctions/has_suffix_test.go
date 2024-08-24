package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type HasSuffixFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&HasSuffixFunctionTestSuite{})

func (s *HasSuffixFunctionTestSuite) SetUpTest(c *C) {
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

func (s *HasSuffixFunctionTestSuite) Test_has_prefix(c *C) {
	hasSuffixFunc := NewHasSuffixFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "has_suffix",
	})
	output, err := hasSuffixFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"https://example.com/config",
				"/config",
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

func (s *HasSuffixFunctionTestSuite) Test_returns_func_error_for_invalid_input(c *C) {
	hasSuffixFunc := NewHasSuffixFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "has_suffix",
	})
	_, err := hasSuffixFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"https://example.com/config/2",
				// A string is expected here, not an integer.
				785043,
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(funcErr.Message, Equals, "argument at index 1 is of type int, but target is of type string")
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "has_suffix",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidArgumentType)
}
