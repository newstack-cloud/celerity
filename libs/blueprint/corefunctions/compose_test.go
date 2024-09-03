package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type ComposeFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&ComposeFunctionTestSuite{})

func (s *ComposeFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{
				"_compose_exec": NewComposeExecFunction(),
				"getattr":       NewGetAttrFunction(),
				"_getattr_exec": NewGetAttrExecFunction(),
				"to_upper":      NewToUpperFunction(),
			},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *ComposeFunctionTestSuite) Test_composes_functions_together_and_executes_composed_function_correctly(c *C) {
	composeFunc := NewComposeFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "compose",
	})

	getAttrFuncOutput, err := s.callContext.registry.Call(
		context.TODO(),
		"getattr",
		&provider.FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args: []any{
					"id",
				},
				callCtx: s.callContext,
			},
			CallContext: s.callContext,
		},
	)
	c.Assert(err, IsNil)

	output, err := composeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]provider.FunctionRuntimeInfo{
					{
						FunctionName: "to_upper",
					},
					getAttrFuncOutput.FunctionInfo,
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)

	// Execute the composition.
	args := []any{
		map[string]interface{}{
			"id":    "test-id-10392",
			"label": "Test Label 10392",
		},
	}
	args = append(args, output.FunctionInfo.PartialArgs...)
	result, err := s.callContext.registry.Call(
		context.TODO(),
		output.FunctionInfo.FunctionName,
		&provider.FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args:    args,
				callCtx: s.callContext,
			},
			CallContext: s.callContext,
		},
	)
	c.Assert(err, IsNil)

	c.Assert(result.ResponseData, Equals, "TEST-ID-10392")
}

func (s *ComposeFunctionTestSuite) Test_composition_execution_fails_for_invalid_args_offset(c *C) {
	composeFunc := NewComposeFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "compose",
	})

	getAttrFuncOutput, err := s.callContext.registry.Call(
		context.TODO(),
		"getattr",
		&provider.FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args: []any{
					"id",
				},
				callCtx: s.callContext,
			},
			CallContext: s.callContext,
		},
	)
	c.Assert(err, IsNil)

	// 30 is not a valid args offset, _compose_exec expects 1 or 0.
	getAttrFuncOutput.FunctionInfo.ArgsOffset = 30
	output, err := composeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]provider.FunctionRuntimeInfo{
					{
						FunctionName: "to_upper",
					},
					getAttrFuncOutput.FunctionInfo,
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)

	// Execute the composition.
	args := []any{
		map[string]interface{}{
			"id":    "test-id-32392",
			"label": "Test Label 32392",
		},
	}
	args = append(args, output.FunctionInfo.PartialArgs...)
	_, err = s.callContext.registry.Call(
		context.TODO(),
		output.FunctionInfo.FunctionName,
		&provider.FunctionCallInput{
			Arguments: &functionCallArgsMock{
				args:    args,
				callCtx: s.callContext,
			},
			CallContext: s.callContext,
		},
	)
	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(
		funcErr.Message,
		Equals,
		"invalid args offset defined for the partially applied function \"_getattr_exec\"",
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "_compose_exec",
		},
		{
			FunctionName: "compose",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidArgsOffset)
}
