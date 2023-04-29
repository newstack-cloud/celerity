package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/core"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) {
	TestingT(t)
}

////////////////////////////////////////////////////////////////////////////////
// Test blueprint params implementing the core.BlueprintParams interface.
////////////////////////////////////////////////////////////////////////////////

type testBlueprintParams struct {
	providerConfig     map[string]map[string]*core.ScalarValue
	contextVariables   map[string]*core.ScalarValue
	blueprintVariables map[string]*core.ScalarValue
}

func (p *testBlueprintParams) ProviderConfig(namespace string) map[string]*core.ScalarValue {
	return p.providerConfig[namespace]
}

func (p *testBlueprintParams) ContextVariable(name string) *core.ScalarValue {
	return p.contextVariables[name]
}

func (p *testBlueprintParams) BlueprintVariable(name string) *core.ScalarValue {
	return p.blueprintVariables[name]
}

////////////////////////////////////////////////////////////////////////////////
// Test custom variable types implementing the provider.CustomVariableType interface.
////////////////////////////////////////////////////////////////////////////////

type testEC2InstanceTypeCustomVariableType struct{}

func (t *testEC2InstanceTypeCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	t2nano := "t2.nano"
	t2micro := "t2.micro"
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := "t2.large"
	t2xlarge := "t2.xlarge"
	t22xlarge := "t2.2xlarge"
	return map[string]*core.ScalarValue{
		t2nano: {
			StringValue: &t2nano,
		},
		t2micro: {
			StringValue: &t2micro,
		},
		t2small: {
			StringValue: &t2small,
		},
		t2medium: {
			StringValue: &t2medium,
		},
		t2large: {
			StringValue: &t2large,
		},
		t2xlarge: {
			StringValue: &t2xlarge,
		},
		t22xlarge: {
			StringValue: &t22xlarge,
		},
	}, nil
}

type testInvalidEC2InstanceTypeCustomVariableType struct{}

func (t *testInvalidEC2InstanceTypeCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	// Invalid due to mixed scalar types.
	t2nano := "t2.nano"
	t2micro := 54039
	t2small := "t2.small"
	t2medium := "t2.medium"
	t2large := 32192.49
	t2xlarge := "t2.xlarge"
	t22xlarge := true
	return map[string]*core.ScalarValue{
		t2nano: {
			StringValue: &t2nano,
		},
		"t2.micro": {
			IntValue: &t2micro,
		},
		t2small: {
			StringValue: &t2small,
		},
		t2medium: {
			StringValue: &t2medium,
		},
		"t2.large": {
			FloatValue: &t2large,
		},
		t2xlarge: {
			StringValue: &t2xlarge,
		},
		"t2.2xlarge": {
			BoolValue: &t22xlarge,
		},
	}, nil
}

type testFailToLoadOptionsCustomVariableType struct{}

func (t *testFailToLoadOptionsCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	return nil, errors.New("failed to load options")
}

type testRegionCustomVariableType struct{}

func (t *testRegionCustomVariableType) Options(
	ctx context.Context,
	params core.BlueprintParams,
) (map[string]*core.ScalarValue, error) {
	usEast1 := "us-east-1"
	usEast2 := "us-east-2"
	usWest1 := "us-west-1"
	usWest2 := "us-west-2"
	euWest1 := "eu-west-1"
	euWest2 := "eu-west-2"
	euCentral1 := "eu-central-1"

	return map[string]*core.ScalarValue{
		usEast1: {
			StringValue: &usEast1,
		},
		usEast2: {
			StringValue: &usEast2,
		},
		usWest1: {
			StringValue: &usWest1,
		},
		usWest2: {
			StringValue: &usWest2,
		},
		euWest1: {
			StringValue: &euWest1,
		},
		euWest2: {
			StringValue: &euWest2,
		},
		euCentral1: {
			StringValue: &euCentral1,
		},
	}, nil
}
