package providerv1

import (
	"fmt"
)

func errResourceTypeNotFound(resourceType string) error {
	return fmt.Errorf("resource type not implemented in provider plugin: %s", resourceType)
}

func errDataSourceTypeNotFound(dataSourceType string) error {
	return fmt.Errorf("data source type not implemented in provider plugin: %s", dataSourceType)
}

func errLinkTypeNotFound(linkType string) error {
	return fmt.Errorf("link type not implemented in provider plugin: %s", linkType)
}

func errCustomVariableTypeNotFound(customVariableType string) error {
	return fmt.Errorf("custom variable type not implemented in provider plugin: %s", customVariableType)
}

func errFunctionNotFound(functionName string) error {
	return fmt.Errorf("function not implemented in provider plugin: %s", functionName)
}

func errResourceCreateFunctionMissing(resourceType string) error {
	return fmt.Errorf(
		"create resource function missing in resource definition for resource type %q",
		resourceType,
	)
}

func errResourceUpdateFunctionMissing(resourceType string) error {
	return fmt.Errorf(
		"update resource function missing in resource definition for resource type %q",
		resourceType,
	)
}

func errResourceGetExternalStateFunctionMissing(resourceType string) error {
	return fmt.Errorf(
		"get external state function missing in resource definition for resource type %q",
		resourceType,
	)
}

func errResourceDestroyFunctionMissing(resourceType string) error {
	return fmt.Errorf(
		"destroy function missing in resource definition for resource type %q",
		resourceType,
	)
}

func errDataSourceFetchFunctionMissing(dataSourceType string) error {
	return fmt.Errorf(
		"fetch function missing in data source definition for data source type %q",
		dataSourceType,
	)
}

func errFunctionCallFunctionMissing(functionName string) error {
	return fmt.Errorf(
		"call function missing in function plugin definition for %q",
		functionName,
	)
}
