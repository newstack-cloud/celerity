package subengine

import (
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

func newTestAWSProvider() provider.Provider {
	return &internal.ProviderMock{
		NamespaceValue: "aws",
		Resources: map[string]provider.Resource{
			"aws/dynamodb/table":  &internal.DynamoDBTableResource{},
			"aws/lambda/function": &internal.LambdaFunctionResource{},
		},
		Links: map[string]provider.Link{},
		CustomVariableTypes: map[string]provider.CustomVariableType{
			"aws/ec2/instanceType": &internal.InstanceTypeCustomVariableType{},
		},
		DataSources: map[string]provider.DataSource{
			"aws/vpc": &internal.VPCDataSource{},
		},
	}
}
