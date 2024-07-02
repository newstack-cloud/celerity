package substitutions

import (
	"fmt"
	"strconv"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
)

// Parser provides an implementation of a parser
// for valid substitution strings
// (i.e. the contents of a ${..} block)
type Parser struct {
	pos int
	// A stack of positions in the sequence where a token
	// evaluation started, this allows for state.pos updates
	// to be reverted when a token evaluation fails.
	startPosStack     []int
	tokens            []*token
	parentSourceStart *source.Meta
	outputLineInfo    bool
}

// NewParser creates a new substitution parser.
func NewParser(tokens []*token, parentSourceStart *source.Meta, outputLineInfo bool) *Parser {
	return &Parser{
		pos:               0,
		startPosStack:     []int{},
		tokens:            tokens,
		parentSourceStart: parentSourceStart,
		outputLineInfo:    outputLineInfo,
	}
}

func (p *Parser) Parse() (*Substitution, error) {
	return p.substitition()
}

// substitution = functionCall | variableRef | datasourceRef | childRef | resourceRef | literal ;
func (p *Parser) substitition() (*Substitution, error) {
	var err error
	var funcCall *SubstitutionFunction
	if funcCall, err = p.functionCall(); funcCall != nil {
		return &Substitution{
			Function:   funcCall,
			SourceMeta: funcCall.SourceMeta,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var varRef *SubstitutionVariable
	if varRef, err = p.variableReference(); varRef != nil {
		return &Substitution{
			Variable:   varRef,
			SourceMeta: varRef.SourceMeta,
		}, nil
	}
	// Errors are only returned in the case of a parse error where
	// there is match for the start of a production rule and subsequent
	// tokens do not match the expected tokens for that production rule.
	// For example, for the variable reference `variables.$!.invalid`,
	// "variables" is matched as a keyword but ".$!.invalid" is not a valid
	// variable name accessor. In this case, an error is returned as the
	// consumed "variables" keyword can only match the start of a
	// variable reference; because of this, we know there is definitely a parse
	// error when the subsequent tokens do not match the rule as there are no other
	// possible symbols that can follow "variables".
	if err != nil {
		return nil, err
	}

	var datasourceRef *SubstitutionDataSourceProperty
	if datasourceRef, err = p.datasourceReference(); datasourceRef != nil {
		return &Substitution{
			DataSourceProperty: datasourceRef,
			SourceMeta:         datasourceRef.SourceMeta,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var childRef *SubstitutionChild
	if childRef, err = p.childReference(); childRef != nil {
		return &Substitution{
			Child:      childRef,
			SourceMeta: childRef.SourceMeta,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var resourceRef *SubstitutionResourceProperty
	if resourceRef, err = p.resourceReference(); resourceRef != nil {
		return &Substitution{
			ResourceProperty: resourceRef,
			SourceMeta:       resourceRef.SourceMeta,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var boolLiteral *bool
	if boolLiteral = p.boolLiteral(); boolLiteral != nil {
		boolSourceMeta := p.sourceMeta(p.previous())
		return &Substitution{
			BoolValue:  boolLiteral,
			SourceMeta: boolSourceMeta,
		}, nil
	}

	var floatLiteral *float64
	if floatLiteral = p.floatLiteral(); floatLiteral != nil {
		floatSourceMeta := p.sourceMeta(p.previous())
		return &Substitution{
			FloatValue: floatLiteral,
			SourceMeta: floatSourceMeta,
		}, nil
	}

	var intLiteral *int64
	if intLiteral = p.intLiteral(); intLiteral != nil {
		intSourceMeta := p.sourceMeta(p.previous())
		return &Substitution{
			IntValue:   intLiteral,
			SourceMeta: intSourceMeta,
		}, nil
	}

	var stringLiteral *string
	if stringLiteral = p.stringLiteral(); stringLiteral != nil {
		prevToken := p.previous()
		strSourceMeta := p.sourceMeta(prevToken)
		return &Substitution{
			StringValue: stringLiteral,
			SourceMeta:  strSourceMeta,
		}, nil
	}

	token := p.currentToken()
	line := token.relativeLine + 1
	if p.parentSourceStart != nil {
		line = token.relativeLine + p.parentSourceStart.Line
	}

	col := token.relativeCol + 1
	if p.parentSourceStart != nil {
		col = token.relativeCol + p.parentSourceStart.Column
	}

	return nil, errParseError(
		token,
		"failed to parse substitution, found unexpected or missing token",
		line,
		col,
	)
}

// variableRef = "variables" , nameAccessor ;
func (p *Parser) variableReference() (*SubstitutionVariable, error) {
	varKeywordToken, err := p.consume(tokenKeywordVariables, "expected variables keyword")
	if err != nil {
		return nil, nil
	}

	name := p.nameAccessor()
	if name == nil {
		return nil, p.error(
			p.peek(),
			"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) after variables keyword",
		)
	}

	return &SubstitutionVariable{
		VariableName: *name,
		SourceMeta:   p.sourceMeta(varKeywordToken),
	}, nil
}

// datasourceRef = "datasources" , nameAccessor , nameAccessor , [ indexAccessor ] ;
func (p *Parser) datasourceReference() (*SubstitutionDataSourceProperty, error) {
	datasourceKeywordToken, err := p.consume(
		tokenKeywordDatasources,
		"expected datasources keyword",
	)
	if err != nil {
		return nil, nil
	}

	name := p.nameAccessor()
	if name == nil {
		return nil, p.error(
			p.peek(),
			"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) after datasources keyword",
		)
	}

	fieldName := p.nameAccessor()
	if fieldName == nil {
		return nil, p.error(
			p.peek(),
			"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) for data source field name",
		)
	}

	indexAccessorPart, err := p.indexAccessor()
	if err != nil {
		return nil, err
	}
	if indexAccessorPart != nil {
		return &SubstitutionDataSourceProperty{
			DataSourceName:    *name,
			FieldName:         *fieldName,
			PrimitiveArrIndex: indexAccessorPart,
			SourceMeta:        p.sourceMeta(datasourceKeywordToken),
		}, nil
	}

	return &SubstitutionDataSourceProperty{
		DataSourceName: *name,
		FieldName:      *fieldName,
		SourceMeta:     p.sourceMeta(datasourceKeywordToken),
	}, nil
}

// childRef = "children" , nameAccessor , { nameAccessor | indexAccessor }- ;
func (p *Parser) childReference() (*SubstitutionChild, error) {
	childrenKeywordToken, err := p.consume(
		tokenKeywordChildren,
		"expected children keyword",
	)
	if err != nil {
		return nil, nil
	}

	childBlueprintName := p.nameAccessor()
	if childBlueprintName == nil {
		return nil, p.error(
			p.peek(),
			"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) after children keyword",
		)
	}

	exportedFieldName := p.nameAccessor()
	if exportedFieldName == nil {
		return nil, p.error(
			p.peek(),
			"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) for child blueprint exported field name",
		)
	}

	path, errors := p.propertyPath(exportedFieldName)
	if len(errors) > 0 {
		return nil, errParseErrorMultiple("failed to parse child reference", errors)
	}

	return &SubstitutionChild{
		ChildName:  *childBlueprintName,
		Path:       path,
		SourceMeta: p.sourceMeta(childrenKeywordToken),
	}, nil
}

// resourceRef = resourceName , [ ( nameAccessor , { namAccessor | indexAccessor } ) ] ;
func (p *Parser) resourceReference() (*SubstitutionResourceProperty, error) {
	firstPartToken := p.currentToken()
	resourceName, err := p.resourceName()
	if err != nil {
		return nil, err
	}
	if resourceName == nil {
		return nil, nil
	}

	propName := p.nameAccessor()
	if propName == nil {
		return &SubstitutionResourceProperty{
			ResourceName: *resourceName,
			Path:         []*SubstitutionPathItem{},
			SourceMeta:   p.sourceMeta(firstPartToken),
		}, nil
	}

	path, errors := p.propertyPath(propName)
	if len(errors) > 0 {
		return nil, errParseErrorMultiple("failed to parse resource reference", errors)
	}

	return &SubstitutionResourceProperty{
		ResourceName: *resourceName,
		Path:         path,
		SourceMeta:   p.sourceMeta(firstPartToken),
	}, nil
}

// resourceName = ( "resources." , nameAccessor ) | name ;
func (p *Parser) resourceName() (*string, error) {
	if p.match(tokenKeywordResources) {
		name := p.nameAccessor()
		if name == nil {
			return nil, p.error(
				p.peek(),
				"expected a valid name accessor (i.e. [\"{name}\"] or .{name}) after resources keyword",
			)
		}

		return name, nil
	}

	return p.name(), nil
}

func (p *Parser) propertyPath(topLevelName *string) ([]*SubstitutionPathItem, []error) {
	path := []*SubstitutionPathItem{}
	if topLevelName != nil {
		path = append(path, &SubstitutionPathItem{
			FieldName: *topLevelName,
		})
	}

	errors := []error{}
	isValidPathItem := true
	for isValidPathItem && !p.isAtEnd() {
		name := p.nameAccessor()
		if name != nil {
			path = append(path, &SubstitutionPathItem{
				FieldName: *name,
			})
			continue
		}

		index, err := p.indexAccessor()
		if err != nil {
			errors = append(errors, err)
		}
		if index != nil {
			path = append(path, &SubstitutionPathItem{
				PrimitiveArrIndex: index,
			})
		} else {
			isValidPathItem = false
		}
	}
	return path, errors
}

// indexAccessor = "[" , [ intLiteral ] , "]" ;
func (p *Parser) indexAccessor() (*int64, error) {
	// As an index accessor is not the only rule that can start with a "[",
	// we need to save the current position in the sequence so that we can revert
	// back in the case that a "[" token is not followed by an int literal.
	p.savePos()
	if p.match(tokenOpenBracket) {
		index := p.intLiteral()

		if !p.match(tokenCloseBracket) {
			// The next token could be a name string literal, so we can't return
			// an error here and we need to trackback to allow another rule (e.g. name accessor)
			// to match on the opening bracket.
			p.backtrack()
			return nil, nil
		}

		finalIndex := index
		if index == nil {
			zeroIndex := int64(0)
			// If we have an open bracket followed by a closing bracket (i.e. [])
			// then we have an empty index accessor, so we default to an index of 0
			// as per the spec.
			// There is an argument to be made that this is semantics that should
			// not be handled by the parser; however, there is no other way to represent
			// an empty index accessor in the AST, as nil is used to represent an omitted
			// optional value.
			finalIndex = &zeroIndex
		}

		p.popPos()
		return finalIndex, nil
	}

	p.popPos()
	return nil, nil
}

func (p *Parser) intLiteral() *int64 {
	if p.match(tokenIntLiteral) {
		// We are safe to ignore this error as the lexer will only
		// extract valid integers as int literal tokens.
		value, _ := strconv.ParseInt(p.previous().value, 10, 64)
		return &value
	}
	return nil
}

func (p *Parser) floatLiteral() *float64 {
	if p.match(tokenFloatLiteral) {
		// We are safe to ignore this error as the lexer will only
		// extract valid floats as float literal tokens.
		value, _ := strconv.ParseFloat(p.previous().value, 64)
		return &value
	}
	return nil
}

func (p *Parser) boolLiteral() *bool {
	if p.match(tokenBoolLiteral) {
		// We are safe to ignore this error as the lexer will only
		// extract valid booleans as bool literal tokens.
		value, _ := strconv.ParseBool(p.previous().value)
		return &value
	}
	return nil
}

// functionCall = name , "(" , [ substitution , { "," , substitution } ] , ")" ;
func (p *Parser) functionCall() (*SubstitutionFunction, error) {
	// As a function call is not the only rule that can start with an identifier,
	// we need to save the current position in the sequence so that we can revert
	// to it if the identifier is not followed by an open parenthesis.
	p.savePos()
	if !p.match(tokenIdent) {
		return nil, nil
	}

	funcNameToken := p.previous()
	// Identifiers can match as the start of a function call or
	// a resource reference, so an error will not be returned
	// if the next token is not an open parenthesis.
	if !p.match(tokenOpenParen) {
		// Allow the identifier to be matched as the start of a different rule
		// (e.g. resource reference)
		p.backtrack()
		return nil, nil
	}

	args := []*Substitution{}
	errors := []error{}
	hasMoreFuncArgs := true
	i := 0
	for hasMoreFuncArgs && !p.isAtEnd() {
		if p.match(tokenCloseParen) {
			hasMoreFuncArgs = false
			continue
		}

		if i > 0 {
			_, err := p.consume(
				tokenComma,
				fmt.Sprintf("expected a comma after function argument %d", i-1),
			)
			if err != nil {
				errors = append(errors, err)
			}
		}

		arg, err := p.substitition()
		if err != nil {
			errors = append(errors, err)
			hasMoreFuncArgs = false
		} else if arg != nil {
			args = append(args, arg)
		} else {
			hasMoreFuncArgs = false
		}

		i += 1
	}

	p.popPos()

	if len(errors) > 0 {
		return nil, errParseErrorMultiple("failed to parse function call", errors)
	}

	return &SubstitutionFunction{
		FunctionName: SubstitutionFunctionName(funcNameToken.value),
		Arguments:    args,
		SourceMeta:   p.sourceMeta(funcNameToken),
	}, nil
}

// nameAccessor = ( "." , name ) | ( "[" , nameStringLiteral , "]" ) ;
func (p *Parser) nameAccessor() *string {
	// As a name accessor is not the only rule that can start with a "[",
	// we need to save the current position in the sequence so that we can revert
	// back in the case that a "[" token is not followed by a name string literal.
	p.savePos()
	if p.match(tokenPeriod) {
		return p.name()
	}

	if !p.match(tokenOpenBracket) {
		return nil
	}

	name := p.nameStringLiteral()
	if name == nil {
		p.backtrack()
		return nil
	}

	p.popPos()

	if p.match(tokenCloseBracket) {
		return name
	}

	return nil
}

func (p *Parser) name() *string {
	if p.match(tokenIdent) {
		return &p.previous().value
	}
	return nil
}

func (p *Parser) nameStringLiteral() *string {
	if p.match(tokenNameStringLiteral) {
		return &p.previous().value
	}
	return nil
}

func (p *Parser) stringLiteral() *string {
	if p.match(tokenStringLiteral) {
		return &p.previous().value
	}
	return nil
}

func (p *Parser) match(types ...tokenType) bool {
	for _, currentType := range types {
		if p.check(currentType) {
			p.advance()
			return true
		}
	}

	return false
}

func (p *Parser) consume(tType tokenType, errorMessage string) (*token, error) {
	if p.check(tType) {
		return p.advance(), nil
	}

	return nil, p.error(p.peek(), errorMessage)
}

func (p *Parser) error(t *token, message string) error {
	line := t.relativeLine + 1
	if p.parentSourceStart != nil {
		line = t.relativeLine + p.parentSourceStart.Line
	}

	col := t.relativeCol + 1
	if p.parentSourceStart != nil {
		col = t.relativeCol + p.parentSourceStart.Column
	}

	return errParseError(t, message, line, col)
}

func (p *Parser) check(tokenType tokenType) bool {
	if p.isAtEnd() {
		return false
	}
	return p.peek().tokenType == tokenType
}

func (p *Parser) advance() *token {
	if !p.isAtEnd() {
		p.pos += 1
	}
	return p.previous()
}

func (p *Parser) peek() *token {
	return p.tokens[p.pos]
}

func (p *Parser) previous() *token {
	return p.tokens[p.pos-1]
}

func (p *Parser) currentToken() *token {
	if !p.isAtEnd() {
		return p.peek()
	}

	if p.pos > 0 {
		return p.previous()
	}

	return nil
}

func (p *Parser) isAtEnd() bool {
	return p.pos >= len(p.tokens)
}

func (p *Parser) savePos() {
	p.startPosStack = append(p.startPosStack, p.pos)
}

func (p *Parser) backtrack() {
	if len(p.startPosStack) > 0 {
		p.pos = p.startPosStack[len(p.startPosStack)-1]
		p.startPosStack = p.startPosStack[:len(p.startPosStack)-1]
	}
}

func (p *Parser) popPos() {
	if len(p.startPosStack) > 0 {
		p.startPosStack = p.startPosStack[:len(p.startPosStack)-1]
	}
}

func (p *Parser) sourceMeta(token *token) *source.Meta {
	if !p.outputLineInfo {
		return nil
	}

	if token == nil {
		return nil
	}

	line := token.relativeLine + 1
	if p.parentSourceStart != nil {
		line = token.relativeLine + p.parentSourceStart.Line
	}

	col := token.relativeCol + 1
	if p.parentSourceStart != nil {
		col = token.relativeCol + p.parentSourceStart.Column
	}

	return &source.Meta{
		Line:   line,
		Column: col,
	}
}
