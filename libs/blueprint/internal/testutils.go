package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint/errors"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type FunctionRegistryMock struct {
	Functions map[string]provider.Function
	CallStack function.Stack
}

func (f *FunctionRegistryMock) Call(
	ctx context.Context,
	functionName string,
	input *provider.FunctionCallInput,
) (*provider.FunctionCallOutput, error) {
	fnc, ok := f.Functions[functionName]
	if !ok {
		return nil, function.NewFuncCallError(
			fmt.Sprintf("function %s not found", functionName),
			function.FuncCallErrorCodeFunctionNotFound,
			input.CallContext.CallStackSnapshot(),
		)
	}
	f.CallStack.Push(&function.Call{
		FunctionName: functionName,
		Location:     nil,
	})
	output, err := fnc.Call(ctx, input)
	f.CallStack.Pop()
	return output, err
}

func (f *FunctionRegistryMock) GetDefinition(
	ctx context.Context,
	functionName string,
	input *provider.FunctionGetDefinitionInput,
) (*provider.FunctionGetDefinitionOutput, error) {
	fnc, ok := f.Functions[functionName]
	if !ok {
		return nil, fmt.Errorf("function %s not found", functionName)
	}
	defOutput, err := fnc.GetDefinition(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

func (f *FunctionRegistryMock) HasFunction(ctx context.Context, functionName string) (bool, error) {
	_, ok := f.Functions[functionName]
	return ok, nil
}

type ResourceRegistryMock struct {
	Resources map[string]provider.Resource
}

func (r *ResourceRegistryMock) HasResourceType(ctx context.Context, resourceType string) (bool, error) {
	_, ok := r.Resources[resourceType]
	return ok, nil
}

func (r *ResourceRegistryMock) GetSpecDefinition(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceGetSpecDefinitionInput,
) (*provider.ResourceGetSpecDefinitionOutput, error) {
	res, ok := r.Resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", resourceType)
	}
	defOutput, err := res.GetSpecDefinition(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

func (r *ResourceRegistryMock) CustomValidate(
	ctx context.Context,
	resourceType string,
	input *provider.ResourceValidateInput,
) (*provider.ResourceValidateOutput, error) {
	res, ok := r.Resources[resourceType]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", resourceType)
	}
	defOutput, err := res.CustomValidate(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

type DataSourceRegistryMock struct {
	DataSources map[string]provider.DataSource
}

func (r *DataSourceRegistryMock) HasDataSourceType(ctx context.Context, dataSourceType string) (bool, error) {
	_, ok := r.DataSources[dataSourceType]
	return ok, nil
}

func (r *DataSourceRegistryMock) GetSpecDefinition(
	ctx context.Context,
	dataSourceType string,
	input *provider.DataSourceGetSpecDefinitionInput,
) (*provider.DataSourceGetSpecDefinitionOutput, error) {
	res, ok := r.DataSources[dataSourceType]
	if !ok {
		return nil, fmt.Errorf("data source %s not found", dataSourceType)
	}
	defOutput, err := res.GetSpecDefinition(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

func (r *DataSourceRegistryMock) GetFilterFields(
	ctx context.Context,
	dataSourceType string,
	input *provider.DataSourceGetFilterFieldsInput,
) (*provider.DataSourceGetFilterFieldsOutput, error) {
	res, ok := r.DataSources[dataSourceType]
	if !ok {
		return nil, fmt.Errorf("data source %s not found", dataSourceType)
	}
	defOutput, err := res.GetFilterFields(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

func (r *DataSourceRegistryMock) CustomValidate(
	ctx context.Context,
	dataSourceType string,
	input *provider.DataSourceValidateInput,
) (*provider.DataSourceValidateOutput, error) {
	res, ok := r.DataSources[dataSourceType]
	if !ok {
		return nil, fmt.Errorf("data source %s not found", dataSourceType)
	}
	defOutput, err := res.CustomValidate(ctx, input)
	if err != nil {
		return nil, err
	}
	return defOutput, nil
}

// UnpackLoadError recursively unpacks a LoadError that can contain child errors.
// This will recursively unpack the first child error until it reaches the last child error.
func UnpackLoadError(err error) (*errors.LoadError, bool) {
	loadErr, ok := err.(*errors.LoadError)
	if ok && len(loadErr.ChildErrors) > 0 {
		return UnpackLoadError(loadErr.ChildErrors[0])
	}
	return loadErr, ok
}

// UnpackError recursively unpacks a LoadError that can contain child errors.
// This is to be used when the terminating error is not a LoadError.
func UnpackError(err error) (error, bool) {
	loadErr, ok := err.(*errors.LoadError)
	if ok && len(loadErr.ChildErrors) > 0 {
		return UnpackLoadError(loadErr.ChildErrors[0])
	}
	return err, ok
}

// Thursday, 7th September 2023 14:43:44
const CurrentTimeUnixMock int64 = 1694097824

type ClockMock struct{}

func (c *ClockMock) Now() time.Time {
	return time.Unix(CurrentTimeUnixMock, 0)
}
