package testprovider

import (
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/plugin-framework/sdk/providerv1"
)

const (
	optionT2Nano    = "t2.nano"
	optionT2Micro   = "t2.micro"
	optionT2Small   = "t2.small"
	optionT2Medium  = "t2.medium"
	optionT2Large   = "t2.large"
	optionT2XLarge  = "t2.xlarge"
	optionT2X2Large = "t2.2xlarge"
)

func customVarTypeEC2InstanceType() provider.CustomVariableType {
	return &providerv1.CustomVariableTypeDefinition{
		Type:                 "aws/ec2/instanceType",
		Label:                "AWS EC2 Instance Type",
		FormattedDescription: "An EC2 instance type.",
		CustomVarTypeOptions: map[string]*provider.CustomVariableTypeOption{
			optionT2Nano: {
				Label: optionT2Nano,
				Value: core.ScalarFromString(optionT2Nano),
			},
			optionT2Micro: {
				Label: optionT2Micro,
				Value: core.ScalarFromString(optionT2Micro),
			},
			optionT2Small: {
				Label: optionT2Small,
				Value: core.ScalarFromString(optionT2Small),
			},
			optionT2Medium: {
				Label: optionT2Medium,
				Value: core.ScalarFromString(optionT2Medium),
			},
			optionT2Large: {
				Label: optionT2Large,
				Value: core.ScalarFromString(optionT2Large),
			},
			optionT2XLarge: {
				Label: optionT2XLarge,
				Value: core.ScalarFromString(optionT2XLarge),
			},
			optionT2X2Large: {
				Label: optionT2X2Large,
				Value: core.ScalarFromString(optionT2X2Large),
			},
		},
	}
}
