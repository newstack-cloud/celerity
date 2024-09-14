package providerhelpers

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/corefunctions"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type coreProvider struct {
	functions    map[string]provider.Function
	functionList []string
}

// NewCoreProvider returns a new instance of the core provider
// that contains all the core functions as per the blueprint
// specification.
func NewCoreProvider(
	linkStateRetriever corefunctions.LinkStateRetriever,
	blueprintInstanceIDRetriever corefunctions.BlueprintInstanceIDRetriever,
	resolveWorkingDir corefunctions.WorkingDirResolver,
	clock core.Clock,
) provider.Provider {
	functions := map[string]provider.Function{
		"fromjson":     corefunctions.NewFromJSONFunction(),
		"fromjson_g":   corefunctions.NewFromJSON_G_Function(),
		"jsondecode":   corefunctions.NewJSONDecodeFunction(),
		"len":          corefunctions.NewLenFunction(),
		"substr":       corefunctions.NewSubstrFunction(),
		"substr_g":     corefunctions.NewSubstr_G_Function(),
		"replace":      corefunctions.NewReplaceFunction(),
		"replace_g":    corefunctions.NewReplace_G_Function(),
		"trim":         corefunctions.NewTrimFunction(),
		"trimprefix":   corefunctions.NewTrimPrefixFunction(),
		"trimprefix_g": corefunctions.NewTrimPrefix_G_Function(),
		"trimsuffix":   corefunctions.NewTrimSuffixFunction(),
		"trimsuffix_g": corefunctions.NewTrimSuffix_G_Function(),
		"split":        corefunctions.NewSplitFunction(),
		"split_g":      corefunctions.NewSplit_G_Function(),
		"join":         corefunctions.NewJoinFunction(),
		"index":        corefunctions.NewIndexFunction(),
		"last_index":   corefunctions.NewLastIndexFunction(),
		"to_upper":     corefunctions.NewToUpperFunction(),
		"to_lower":     corefunctions.NewToLowerFunction(),
		"has_prefix":   corefunctions.NewHasPrefixFunction(),
		"has_prefix_g": corefunctions.NewHasPrefix_G_Function(),
		"has_suffix":   corefunctions.NewHasSuffixFunction(),
		"has_suffix_g": corefunctions.NewHasSuffix_G_Function(),
		"contains":     corefunctions.NewContainsFunction(),
		"contains_g":   corefunctions.NewContains_G_Function(),
		"list":         corefunctions.NewListFunction(),
		"object":       corefunctions.NewObjectFunction(),
		"keys":         corefunctions.NewKeysFunction(),
		"vals":         corefunctions.NewValsFunction(),
		"map":          corefunctions.NewMapFunction(),
		"filter":       corefunctions.NewFilterFunction(),
		"reduce":       corefunctions.NewReduceFunction(),
		"sort":         corefunctions.NewSortFunction(),
		"flatmap":      corefunctions.NewFlatMapFunction(),
		"compose":      corefunctions.NewComposeFunction(),
		"pipe":         corefunctions.NewPipeFunction(),
		"getattr":      corefunctions.NewGetAttrFunction(),
		"getelem":      corefunctions.NewGetElemFunction(),
		"link": corefunctions.NewLinkFunction(
			linkStateRetriever,
			blueprintInstanceIDRetriever,
		),
		"and":      corefunctions.NewAndFunction(),
		"or":       corefunctions.NewOrFunction(),
		"not":      corefunctions.NewNotFunction(),
		"eq":       corefunctions.NewEqFunction(),
		"gt":       corefunctions.NewGtFunction(),
		"ge":       corefunctions.NewGeFunction(),
		"lt":       corefunctions.NewLtFunction(),
		"le":       corefunctions.NewLeFunction(),
		"cwd":      corefunctions.NewCWDFunction(resolveWorkingDir),
		"datetime": corefunctions.NewDateTimeFunction(clock),
	}
	return &coreProvider{
		functions:    functions,
		functionList: funcMapKeys(functions),
	}
}

func (p *coreProvider) Namespace(ctx context.Context) (string, error) {
	return "core", nil
}

func (p *coreProvider) Resource(ctx context.Context, resourceType string) (provider.Resource, error) {
	return nil, fmt.Errorf(
		"resource type %q not found in core provider, "+
			"only functions are made available by the core provider",
		resourceType,
	)
}

func (p *coreProvider) DataSource(ctx context.Context, dataSourceType string) (provider.DataSource, error) {
	return nil, fmt.Errorf(
		"data source type %q not found in core provider, "+
			"only functions are made available by the core provider",
		dataSourceType,
	)
}

func (p *coreProvider) Link(ctx context.Context, resourceTypeA string, resourceTypeB string) (provider.Link, error) {
	return nil, fmt.Errorf(
		"link between resource types %q and %q not found in core provider, "+
			"only functions are made available by the core provider",
		resourceTypeA,
		resourceTypeB,
	)
}

func (p *coreProvider) CustomVariableType(ctx context.Context, customVariableType string) (provider.CustomVariableType, error) {
	return nil, fmt.Errorf(
		"custom variable type %q not found in core provider, "+
			"only functions are made available by the core provider",
		customVariableType,
	)
}

func (p *coreProvider) ListResourceTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *coreProvider) ListDataSourceTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *coreProvider) ListFunctions(ctx context.Context) ([]string, error) {
	return p.functionList, nil
}

func (p *coreProvider) ListCustomVariableTypes(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (p *coreProvider) Function(ctx context.Context, functionName string) (provider.Function, error) {
	function, ok := p.functions[functionName]
	if !ok {
		return nil, fmt.Errorf(
			"function %q not found in core provider",
			functionName,
		)
	}
	return function, nil
}

func funcMapKeys(m map[string]provider.Function) []string {
	keys := []string{}
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
