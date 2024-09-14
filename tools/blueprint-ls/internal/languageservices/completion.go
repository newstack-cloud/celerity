package languageservices

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/resourcehelpers"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/source"
	"github.com/two-hundred/ls-builder/common"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

var (
	// Pattern for matching "type:" before the cursor position
	// in determining the type of completion items to provide.
	typePattern = regexp.MustCompile(`type:\s*$`)

	// Pattern for matching "field:" before the cursor position
	// in determining the type of completion items to provide.
	fieldPattern = regexp.MustCompile(`field:\s*$`)

	// Pattern for matching "operator:" before the cursor position
	// in determining the type of completion items to provide.
	operatorPattern = regexp.MustCompile(`operator:\s*$`)

	// Pattern for matching ".variables." before the cursor position
	// in determining the type of completion items to provide.
	variableRefPattern = regexp.MustCompile(`variables\.$`)
)

// CompletionService is a service that provides functionality
// for completion suggestions.
type CompletionService struct {
	resourceRegistry      resourcehelpers.Registry
	dataSourceRegistry    provider.DataSourceRegistry
	customVarTypeRegistry provider.CustomVariableTypeRegistry
	state                 *State
	logger                *zap.Logger
}

// NewCompletionService creates a new service for completion suggestions.
func NewCompletionService(
	resourceRegistry resourcehelpers.Registry,
	dataSourceRegistry provider.DataSourceRegistry,
	customVarTypeRegistry provider.CustomVariableTypeRegistry,
	state *State,
	logger *zap.Logger,
) *CompletionService {
	return &CompletionService{
		resourceRegistry:      resourceRegistry,
		dataSourceRegistry:    dataSourceRegistry,
		customVarTypeRegistry: customVarTypeRegistry,
		state:                 state,
		logger:                logger,
	}
}

// GetCompletionItems returns completion items for a given position in a document.
func (s *CompletionService) GetCompletionItems(
	ctx *common.LSPContext,
	content string,
	tree *schema.TreeNode,
	blueprint *schema.Blueprint,
	params *lsp.TextDocumentPositionParams,
) ([]*lsp.CompletionItem, error) {

	// The last element in the collected list is the element with the shortest
	// range that contains the position.
	collected := []*schema.TreeNode{}
	collectElementsAtPosition(tree, params.Position, s.logger, &collected)

	return s.getCompletionItems(
		ctx,
		content,
		blueprint,
		&params.Position,
		collected,
	)
}

func (s *CompletionService) getCompletionItems(
	ctx *common.LSPContext,
	content string,
	blueprint *schema.Blueprint,
	position *lsp.Position,
	collected []*schema.TreeNode,
) ([]*lsp.CompletionItem, error) {

	if len(collected) == 0 {
		return []*lsp.CompletionItem{}, nil
	}

	// Work backwards through the collected elements to find the first element
	// of a type that supports completion items.
	var node *schema.TreeNode
	var elementType string
	i := len(collected) - 1
	for node == nil && i >= 0 {
		pathParts := strings.Split(collected[i].Path, "/")
		node, elementType = s.matchCompletionElement(
			collected,
			i,
			pathParts,
			content,
			position,
		)
		i -= 1
	}

	switch elementType {
	case "resourceType":
		return s.getResourceTypeCompletionItems(ctx)
	case "dataSourceType":
		return s.getDataSourceTypeCompletionItems(ctx)
	case "variableType":
		return s.getVariableTypeCompletionItems(ctx)
	case "valueType":
		return s.getValueTypeCompletionItems()
	case "dataSourceFieldType":
		return s.getDataSourceFieldTypeCompletionItems()
	case "dataSourceFilterField":
		return s.getDataSourceFilterFieldCompletionItems(ctx, node, blueprint)
	case "dataSourceFilterOperator":
		return s.getDataSourceFilterOperatorCompletionItems(position, content, node)
	case "exportType":
		return s.getExportTypeCompletionItems()
	default:
		return []*lsp.CompletionItem{}, nil
	}
}

func (s *CompletionService) matchCompletionElement(
	collected []*schema.TreeNode,
	index int,
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) (*schema.TreeNode, string) {

	if s.isResourceType(pathParts, sourceContent, position) {
		return collected[index], "resourceType"
	}

	if s.isDataSourceType(pathParts, sourceContent, position) {
		return collected[index], "dataSourceType"
	}

	if s.isVariableType(pathParts, sourceContent, position) {
		return collected[index], "variableType"
	}

	if s.isValueType(pathParts, sourceContent, position) {
		return collected[index], "valueType"
	}

	// Check potential new fields first to avoid getting the wrong completion type
	// in data source filter fields and operators.
	if s.isNewDataSourceFilterField(pathParts, sourceContent, position) {
		return collected[index], "dataSourceFilterField"
	}

	if s.isNewDataSourceFilterOperator(pathParts, sourceContent, position) {
		return collected[index], "dataSourceFilterOperator"
	}

	if s.isDataSourceFieldType(pathParts, sourceContent, position) {
		return collected[index], "dataSourceFieldType"
	}

	if s.isDataSourceFilterField(pathParts) {
		return collected[index], "dataSourceFilterField"
	}

	if s.isDataSourceFilterOperator(pathParts) {
		return collected[index], "dataSourceFilterOperator"
	}

	if s.isExportType(pathParts, sourceContent, position) {
		return collected[index], "exportType"
	}

	if s.isStringSubVariable(pathParts, sourceContent, position) {
		return collected[index], "stringSubVariableRef"
	}

	return nil, ""
}

func (s *CompletionService) isResourceType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingResourceType := len(pathParts) == 4 &&
		pathParts[1] == "resources" &&
		pathParts[3] == "type"

	isInNewResourceType := len(pathParts) == 3 &&
		pathParts[1] == "resources" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingResourceType || isInNewResourceType
}

func (s *CompletionService) isDataSourceType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingDataSourceType := len(pathParts) == 4 &&
		pathParts[1] == "datasources" &&
		pathParts[3] == "type"

	isInNewDataSourceType := len(pathParts) == 3 &&
		pathParts[1] == "datasources" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingDataSourceType || isInNewDataSourceType
}

func (s *CompletionService) isVariableType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingVarType := len(pathParts) == 4 &&
		pathParts[1] == "variables" &&
		pathParts[3] == "type"

	isInNewVarType := len(pathParts) == 3 &&
		pathParts[1] == "variables" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingVarType || isInNewVarType
}

func (s *CompletionService) isValueType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingValType := len(pathParts) == 4 &&
		pathParts[1] == "values" &&
		pathParts[3] == "type"

	isInNewValType := len(pathParts) == 3 &&
		pathParts[1] == "values" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingValType || isInNewValType
}

func (s *CompletionService) isDataSourceFieldType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingExportType := len(pathParts) == 5 &&
		pathParts[1] == "datasources" &&
		pathParts[4] == "type"

	isInNewExportType := len(pathParts) == 4 &&
		pathParts[1] == "datasources" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingExportType || isInNewExportType
}

func (s *CompletionService) isNewDataSourceFilterField(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	return len(pathParts) >= 4 &&
		pathParts[1] == "datasources" &&
		pathParts[3] == "filter" &&
		s.isPrecededBy(position, fieldPattern, sourceContent)
}

func (s *CompletionService) isDataSourceFilterField(
	pathParts []string,
) bool {
	return len(pathParts) == 5 &&
		pathParts[1] == "datasources" &&
		pathParts[3] == "filter" &&
		pathParts[4] == "field"
}

func (s *CompletionService) isNewDataSourceFilterOperator(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	return len(pathParts) >= 4 &&
		pathParts[1] == "datasources" &&
		pathParts[3] == "filter" &&
		s.isPrecededBy(position, operatorPattern, sourceContent)
}

func (s *CompletionService) isDataSourceFilterOperator(
	pathParts []string,
) bool {
	return len(pathParts) == 5 &&
		pathParts[1] == "datasources" &&
		pathParts[3] == "filter" &&
		pathParts[4] == "operator"
}

func (s *CompletionService) isExportType(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	isInExistingExportType := len(pathParts) == 4 &&
		pathParts[1] == "exports" &&
		pathParts[3] == "type"

	isInNewExportType := len(pathParts) == 3 &&
		pathParts[1] == "exports" &&
		s.isPrecededBy(position, typePattern, sourceContent)

	return isInExistingExportType || isInNewExportType
}

func (s *CompletionService) isStringSubVariable(
	pathParts []string,
	sourceContent string,
	position *lsp.Position,
) bool {
	return slices.Contains(pathParts, "stringSubs") &&
		s.isPrecededBy(position, variableRefPattern, sourceContent)
}

func (s *CompletionService) isPrecededBy(
	position *lsp.Position,
	expectedBeforePattern *regexp.Regexp,
	sourceContent string,
) bool {
	if position.Line == 0 {
		return false
	}

	index := position.IndexIn(sourceContent, s.state.GetPositionEncodingKind())
	sourceContentBefore := sourceContent[:index]
	return expectedBeforePattern.Match([]byte(sourceContentBefore))
}

func (s *CompletionService) getResourceTypeCompletionItems(
	ctx *common.LSPContext,
) ([]*lsp.CompletionItem, error) {

	resourceTypes, err := s.resourceRegistry.ListResourceTypes(ctx.Context)
	if err != nil {
		return nil, err
	}

	completionItems := []*lsp.CompletionItem{}
	resourceTypeDetail := "Resource type"
	for _, resourceType := range resourceTypes {
		enumKind := lsp.CompletionItemKindEnum
		completionItems = append(completionItems, &lsp.CompletionItem{
			Label:      resourceType,
			Detail:     &resourceTypeDetail,
			Kind:       &enumKind,
			InsertText: &resourceType,
			Data:       map[string]interface{}{"completionType": "resourceType"},
		})
	}

	return completionItems, nil
}

func (s *CompletionService) getDataSourceTypeCompletionItems(
	ctx *common.LSPContext,
) ([]*lsp.CompletionItem, error) {

	dataSourceTypes, err := s.dataSourceRegistry.ListDataSourceTypes(ctx.Context)
	if err != nil {
		return nil, err
	}

	completionItems := []*lsp.CompletionItem{}
	dataSourceTypeDetail := "Data source type"
	for _, dataSourceType := range dataSourceTypes {
		enumKind := lsp.CompletionItemKindEnum
		completionItems = append(completionItems, &lsp.CompletionItem{
			Label:      dataSourceType,
			Detail:     &dataSourceTypeDetail,
			Kind:       &enumKind,
			InsertText: &dataSourceType,
			Data:       map[string]interface{}{"completionType": "dataSourceType"},
		})
	}

	return completionItems, nil
}

func (s *CompletionService) getDataSourceFilterFieldCompletionItems(
	ctx *common.LSPContext,
	node *schema.TreeNode,
	blueprint *schema.Blueprint,
) ([]*lsp.CompletionItem, error) {

	if node == nil || blueprint.DataSources == nil || len(blueprint.DataSources.Values) == 0 {
		return []*lsp.CompletionItem{}, nil
	}

	dataSourceName := getDataSourceName(node)
	if dataSourceName == "" {
		return []*lsp.CompletionItem{}, nil
	}

	dataSource, hasDataSource := blueprint.DataSources.Values[dataSourceName]
	if !hasDataSource || dataSource.Type == nil {
		return []*lsp.CompletionItem{}, nil
	}

	filterFieldsOutput, err := s.dataSourceRegistry.GetFilterFields(
		ctx.Context,
		string(dataSource.Type.Value),
		&provider.DataSourceGetFilterFieldsInput{},
	)
	if err != nil {
		return nil, err
	}

	completionItems := []*lsp.CompletionItem{}
	filterFieldDetail := "Data source filter field"
	for _, filterField := range filterFieldsOutput.Fields {
		enumKind := lsp.CompletionItemKindEnum
		completionItems = append(completionItems, &lsp.CompletionItem{
			Label:      filterField,
			Detail:     &filterFieldDetail,
			Kind:       &enumKind,
			InsertText: &filterField,
			Data:       map[string]interface{}{"completionType": "dataSourceFilterField"},
		})
	}

	return completionItems, nil
}

func getDataSourceName(node *schema.TreeNode) string {
	if node == nil {
		return ""
	}

	pathParts := strings.Split(node.Path, "/")
	if len(pathParts) > 2 && pathParts[1] == "datasources" {
		return pathParts[2]
	}

	return ""
}

func (s *CompletionService) getVariableTypeCompletionItems(
	ctx *common.LSPContext,
) ([]*lsp.CompletionItem, error) {
	variableTypeDetail := "Variable type"
	enumKind := lsp.CompletionItemKindEnum

	typeItems := []*lsp.CompletionItem{}
	for _, coreType := range schema.CoreVariableTypes {
		coreTypeStr := string(coreType)
		typeItems = append(
			typeItems,
			&lsp.CompletionItem{
				Label:      coreTypeStr,
				Detail:     &variableTypeDetail,
				Kind:       &enumKind,
				InsertText: &coreTypeStr,
				Data:       map[string]interface{}{"completionType": "variableType"},
			},
		)
	}

	customTypes, err := s.customVarTypeRegistry.ListCustomVariableTypes(ctx.Context)
	if err != nil {
		s.logger.Error("Failed to list custom variable types, returning core types only", zap.Error(err))
		return typeItems, nil
	}

	for _, customType := range customTypes {
		typeItems = append(typeItems, &lsp.CompletionItem{
			Label:      customType,
			Detail:     &variableTypeDetail,
			Kind:       &enumKind,
			InsertText: &customType,
			Data:       map[string]interface{}{"completionType": "variableType"},
		})
	}

	return typeItems, nil
}

func (s *CompletionService) getValueTypeCompletionItems() ([]*lsp.CompletionItem, error) {
	valueTypeDetail := "Value type"
	enumKind := lsp.CompletionItemKindEnum

	typeItems := []*lsp.CompletionItem{}
	for _, valueType := range schema.ValueTypes {
		valueTypeStr := string(valueType)
		typeItems = append(
			typeItems,
			&lsp.CompletionItem{
				Label:      valueTypeStr,
				Detail:     &valueTypeDetail,
				Kind:       &enumKind,
				InsertText: &valueTypeStr,
				Data:       map[string]interface{}{"completionType": "valueType"},
			},
		)
	}

	return typeItems, nil
}

func (s *CompletionService) getDataSourceFieldTypeCompletionItems() ([]*lsp.CompletionItem, error) {
	fieldTypeDetail := "Data source field type"
	enumKind := lsp.CompletionItemKindEnum

	typeItems := []*lsp.CompletionItem{}
	for _, fieldType := range schema.DataSourceFieldTypes {
		fieldTypeStr := string(fieldType)
		typeItems = append(
			typeItems,
			&lsp.CompletionItem{
				Label:      fieldTypeStr,
				Detail:     &fieldTypeDetail,
				Kind:       &enumKind,
				InsertText: &fieldTypeStr,
				Data:       map[string]interface{}{"completionType": "dataSourceFieldType"},
			},
		)
	}

	return typeItems, nil
}

func (s *CompletionService) getDataSourceFilterOperatorCompletionItems(
	position *lsp.Position,
	sourceContent string,
	operatorElementNode *schema.TreeNode,
) ([]*lsp.CompletionItem, error) {
	filterOperatorDetail := "Data source filter operator"
	enumKind := lsp.CompletionItemKindEnum
	positionEncodingKind := s.state.GetPositionEncodingKind()

	operatorElementPosition := operatorElementNode.Range.Start

	filterOpItems := []*lsp.CompletionItem{}
	for _, filterOperator := range schema.DataSourceFilterOperators {
		filterOperatorStr := fmt.Sprintf("\"%s\"", string(filterOperator))
		// Due to filters containing white space characters, which are trigger characters
		// for completion, we need to do a custom edit instead of the default insertText
		// approach.
		edit := lsp.TextEdit{
			NewText: filterOperatorStr,
			Range: getOperatorInsertRange(
				position,
				filterOperatorStr,
				sourceContent,
				positionEncodingKind,
				operatorElementPosition,
			),
		}
		filterOpItems = append(
			filterOpItems,
			&lsp.CompletionItem{
				Label:      filterOperatorStr,
				Detail:     &filterOperatorDetail,
				Kind:       &enumKind,
				InsertText: &filterOperatorStr,
				TextEdit:   edit,
				Data:       map[string]interface{}{"completionType": "dataSourceFilterOperator"},
			},
		)
	}

	return filterOpItems, nil
}

func (s *CompletionService) getExportTypeCompletionItems() ([]*lsp.CompletionItem, error) {
	exportTypeDetail := "Export type"
	enumKind := lsp.CompletionItemKindEnum

	typeItems := []*lsp.CompletionItem{}
	for _, exportType := range schema.ExportTypes {
		exportTypeStr := string(exportType)
		typeItems = append(
			typeItems,
			&lsp.CompletionItem{
				Label:      exportTypeStr,
				Detail:     &exportTypeDetail,
				Kind:       &enumKind,
				InsertText: &exportTypeStr,
				Data:       map[string]interface{}{"completionType": "exportType"},
			},
		)
	}

	return typeItems, nil
}

func getOperatorInsertRange(
	position *lsp.Position,
	insertText string,
	sourceContent string,
	positionEncodingKind lsp.PositionEncodingKind,
	operatorElementPosition *source.Meta,
) *lsp.Range {
	index := position.IndexIn(sourceContent, positionEncodingKind)
	sourceContentBefore := sourceContent[:index]
	charCount := utf8.RuneCountInString(insertText)

	if operatorPattern.Match([]byte(sourceContentBefore)) {
		return &lsp.Range{
			Start: lsp.Position{
				Line:      position.Line,
				Character: position.Character,
			},
			End: lsp.Position{
				Line:      position.Line,
				Character: position.Character + lsp.UInteger(charCount),
			},
		}
	}

	start := lsp.Position{
		Line:      lsp.UInteger(operatorElementPosition.Line) - 1,
		Character: lsp.UInteger(operatorElementPosition.Column) - 1,
	}

	return &lsp.Range{
		Start: start,
		End: lsp.Position{
			Line:      start.Line,
			Character: start.Character + lsp.UInteger(charCount),
		},
	}
}

// ResolveCompletionItem resolves extra information such as detailed
// descriptions for a completion item.
func (s *CompletionService) ResolveCompletionItem(
	ctx *common.LSPContext,
	item *lsp.CompletionItem,
	completionType string,
) (*lsp.CompletionItem, error) {
	switch completionType {
	case "resourceType":
		return s.resolveResourceTypeCompletionItem(ctx, item)
	case "dataSourceType":
		return s.resolveDataSourceTypeCompletionItem(ctx, item)
	case "variableType":
		return s.resolveVariableTypeCompletionItem(ctx, item)
	default:
		return item, nil
	}
}

func (s *CompletionService) resolveResourceTypeCompletionItem(
	ctx *common.LSPContext,
	item *lsp.CompletionItem,
) (*lsp.CompletionItem, error) {
	resourceType := item.Label
	descriptionOutput, err := s.resourceRegistry.GetTypeDescription(
		ctx.Context,
		resourceType,
		&provider.ResourceGetTypeDescriptionInput{},
	)
	if err != nil {
		return nil, err
	}

	if descriptionOutput.MarkdownDescription != "" {
		item.Documentation = lsp.MarkupContent{
			Kind:  lsp.MarkupKindMarkdown,
			Value: descriptionOutput.MarkdownDescription,
		}
	} else if descriptionOutput.PlainTextDescription != "" {
		item.Documentation = descriptionOutput.PlainTextDescription
	}

	return item, nil
}

func (s *CompletionService) resolveDataSourceTypeCompletionItem(
	ctx *common.LSPContext,
	item *lsp.CompletionItem,
) (*lsp.CompletionItem, error) {
	dataSourceType := item.Label
	descriptionOutput, err := s.dataSourceRegistry.GetTypeDescription(
		ctx.Context,
		dataSourceType,
		&provider.DataSourceGetTypeDescriptionInput{},
	)
	if err != nil {
		return nil, err
	}

	if descriptionOutput.MarkdownDescription != "" {
		item.Documentation = lsp.MarkupContent{
			Kind:  lsp.MarkupKindMarkdown,
			Value: descriptionOutput.MarkdownDescription,
		}
	} else if descriptionOutput.PlainTextDescription != "" {
		item.Documentation = descriptionOutput.PlainTextDescription
	}

	return item, nil
}

func (s *CompletionService) resolveVariableTypeCompletionItem(
	ctx *common.LSPContext,
	item *lsp.CompletionItem,
) (*lsp.CompletionItem, error) {
	variableType := item.Label
	if slices.Contains(schema.CoreVariableTypes, schema.VariableType(variableType)) {
		return item, nil
	}

	descriptionOutput, err := s.customVarTypeRegistry.GetDescription(
		ctx.Context,
		variableType,
		&provider.CustomVariableTypeGetDescriptionInput{},
	)
	if err != nil {
		return nil, err
	}

	if descriptionOutput.MarkdownDescription != "" {
		item.Documentation = lsp.MarkupContent{
			Kind:  lsp.MarkupKindMarkdown,
			Value: descriptionOutput.MarkdownDescription,
		}
	} else if descriptionOutput.PlainTextDescription != "" {
		item.Documentation = descriptionOutput.PlainTextDescription
	}

	return item, nil
}
