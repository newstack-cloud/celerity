package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type EqFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&EqFunctionTestSuite{})

func (s *EqFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *EqFunctionTestSuite) Test_equals_1(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"This is a string",
				"This is a string",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, true)
}

func (s *EqFunctionTestSuite) Test_equals_2(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"This is a string",
				"This is a different string",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, false)
}

func (s *EqFunctionTestSuite) Test_equals_3(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				map[string]interface{}{
					"key": "value",
				},
				map[string]interface{}{
					"key": "value",
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, true)
}

func (s *EqFunctionTestSuite) Test_equals_4(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				map[string]interface{}{
					"key1": "value1",
				},
				map[string]interface{}{
					"key1": "value1",
					"key2": []interface{}{1, 2, 3},
				},
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, false)
}

func (s *EqFunctionTestSuite) Test_equals_comparable_1(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				comparableInt(9),
				comparableInt(9),
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, true)
}

func (s *EqFunctionTestSuite) Test_equals_comparable_2(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	output, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				comparableInt(9),
				comparableInt(15),
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputBool, isBool := output.ResponseData.(bool)
	c.Assert(isBool, Equals, true)
	c.Assert(outputBool, Equals, false)
}

func (s *EqFunctionTestSuite) Test_returns_func_error_for_comparison_type_mismatch(c *C) {
	eqFunc := NewEqFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "eq",
	})
	_, err := eqFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"This is a string",
				// An integer cannot be compared to a string.
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
		"expected both values to be of the same type, got string and int",
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "eq",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidArgumentType)
}
