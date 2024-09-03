package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type FilterFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&FilterFunctionTestSuite{})

func (s *FilterFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{
				"has_prefix": NewHasPrefixFunction(),
			},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *FilterFunctionTestSuite) Test_filters_items_correctly(c *C) {

	filterFunc := NewFilterFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "filter",
	})
	output, err := filterFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]interface{}{"core_a", "core_b", "aux_1", "aux_2", "core_c", "aux_3"},
				provider.FunctionRuntimeInfo{
					FunctionName: "has_prefix",
					PartialArgs: []any{
						"core_",
					},
					ArgsOffset: 1,
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputSlice, isStrSlice := output.ResponseData.([]interface{})
	c.Assert(isStrSlice, Equals, true)
	c.Assert(outputSlice, DeepEquals, []interface{}{"core_a", "core_b", "core_c"})
}

func (s *FilterFunctionTestSuite) Test_returns_func_error_for_invalid_item_in_list(c *C) {
	filterFunc := NewFilterFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "filter",
	})
	_, err := filterFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				// 5 is not a string, the has_prefix function should return an error.
				[]interface{}{"item_a", 5, "item_c"},
				provider.FunctionRuntimeInfo{
					FunctionName: "has_prefix",
					PartialArgs: []any{
						"item_",
					},
					ArgsOffset: 1,
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(funcErr.Message, Equals, "argument at index 0 is of type int, but target is of type string")
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "has_prefix",
		},
		{
			FunctionName: "filter",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidArgumentType)
}

func (s *FilterFunctionTestSuite) Test_returns_func_error_for_invalid_args_offset(c *C) {
	filterFunc := NewFilterFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "filter",
	})
	_, err := filterFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				[]interface{}{"item_a", "item_c"},
				provider.FunctionRuntimeInfo{
					FunctionName: "has_prefix",
					PartialArgs: []any{
						"item_",
					},
					// 20 is not a valid args offset for the has_prefix function.
					ArgsOffset: 20,
				},
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
		"invalid args offset defined for the partially applied \"has_prefix\""+
			" function, this is an issue with the function used to create the function value passed into filter",
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "filter",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidArgsOffset)
}
