package corefunctions

import (
	"context"
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type DateTimeFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
	clock       core.Clock
}

var _ = Suite(&DateTimeFunctionTestSuite{})

func (s *DateTimeFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
	s.clock = &internal.ClockMock{}
}

func (s *DateTimeFunctionTestSuite) Test_gets_current_time_unix_format(c *C) {
	dateTimeFunc := NewDateTimeFunction(s.clock)
	s.callStack.Push(&function.Call{
		FunctionName: "datetime",
	})
	output, err := dateTimeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"unix",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(string)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, fmt.Sprintf("%d", internal.CurrentTimeUnixMock))
}

func (s *DateTimeFunctionTestSuite) Test_gets_current_time_rfc3339_format(c *C) {
	dateTimeFunc := NewDateTimeFunction(s.clock)
	s.callStack.Push(&function.Call{
		FunctionName: "datetime",
	})
	output, err := dateTimeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"rfc3339",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(string)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, "2023-09-07T14:43:44Z")
}

func (s *DateTimeFunctionTestSuite) Test_gets_current_time_tag_format(c *C) {
	dateTimeFunc := NewDateTimeFunction(s.clock)
	s.callStack.Push(&function.Call{
		FunctionName: "datetime",
	})
	output, err := dateTimeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"tag",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(string)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, "2023-09-07--14-43-44")
}

func (s *DateTimeFunctionTestSuite) Test_gets_current_time_tagcompact_format(c *C) {
	dateTimeFunc := NewDateTimeFunction(s.clock)
	s.callStack.Push(&function.Call{
		FunctionName: "datetime",
	})
	output, err := dateTimeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"tagcompact",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(string)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, "20230907144344")
}

func (s *DateTimeFunctionTestSuite) Test_returns_func_error_when_unsupported_format_is_provided(c *C) {
	dateTimeFunc := NewDateTimeFunction(s.clock)
	s.callStack.Push(&function.Call{
		FunctionName: "datetime",
	})
	_, err := dateTimeFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				// rfc3339nano is not a supported format
				"rfc3339nano",
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
		fmt.Sprintf(
			"the requested date/time format is not supported by the \"datetime\" function, "+
				"supported formats include: %s", strings.Join(SupportedDateTimeFormats, ", "),
		),
	)
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "datetime",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeInvalidInput)
}
