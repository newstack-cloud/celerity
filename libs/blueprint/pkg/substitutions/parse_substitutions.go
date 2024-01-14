package substitutions

import (
	"strings"
)

type interpolationParseState struct {
	parsed             []*StringOrSubstitution
	inPossibleSub      bool
	inStringLiteral    bool
	potentialSub       string
	potentialNonSubStr string
	prevChar           rune
	errors             []error
}

// ParseSubstitutionValues parses a string that can contain interpolated
// references.
func ParseSubstitutionValues(substitutionContext, value string) ([]*StringOrSubstitution, error) {
	// This is hand-rolled to account for the fact that string literals
	// are supported in the spec for substitutions and they can contain
	// the "${" and "}" syntax, trying to do string splitting won't catch
	// this and regular expressions without lookaheads can't be used to express
	// this.

	// There are no openings for substitutions, the value is just a string literal.
	// This may not be the case if the string literal contains the "${" in which case
	// it will be caught in the process of evaulating every character in sequence.
	if !strings.Contains(value, "${") {
		return []*StringOrSubstitution{
			{
				StringValue: &value,
			},
		}, nil
	}

	state := &interpolationParseState{
		// To alleviate the frustration of having to deal with only seeing
		// one error at a time per substitution, all errors are collected.
		// The trade-off will not be significant in most use cases as string
		// values that can contain substitutions are not expected to be very long.
		errors:             []error{},
		parsed:             []*StringOrSubstitution{},
		inPossibleSub:      false,
		inStringLiteral:    false,
		potentialSub:       "",
		potentialNonSubStr: "",
		prevChar:           ' ',
	}

	for i := 0; i < len(value); i += 1 {
		isOpenSubBracket := checkOpenSubBracket(state, value, i)
		checkStringLiteral(state, value, i)
		isCloseSubBracket := checkCloseSubBracket(state, value, i, substitutionContext)

		state.prevChar = rune(value[i])
		if !isCloseSubBracket {
			state.potentialNonSubStr += string(value[i])
		}
		if state.inPossibleSub && !isOpenSubBracket {
			state.potentialSub += string(value[i])
		}
	}

	if len(state.potentialNonSubStr) > 0 {
		state.parsed = append(state.parsed, &StringOrSubstitution{
			StringValue: &state.potentialNonSubStr,
		})
	}

	if len(state.errors) > 0 {
		return nil, errSubstitutions(substitutionContext, state.errors)
	}

	return state.parsed, nil
}

func checkOpenSubBracket(state *interpolationParseState, value string, i int) bool {
	isOpenSubBracket := state.prevChar == '$' && value[i] == '{' && !state.inStringLiteral
	if isOpenSubBracket {
		// Start of a substitution
		state.inPossibleSub = true
		nonSubStr := state.potentialNonSubStr[:len(state.potentialNonSubStr)-1]
		if len(nonSubStr) > 0 {
			state.parsed = append(state.parsed, &StringOrSubstitution{
				StringValue: &nonSubStr,
			})
		}
	}
	return isOpenSubBracket
}

func checkStringLiteral(state *interpolationParseState, value string, i int) {
	if value[i] == '"' && state.prevChar != '\\' && state.inPossibleSub {
		state.inStringLiteral = !state.inStringLiteral
	}
}

func checkCloseSubBracket(state *interpolationParseState, value string, i int, substitutionContext string) bool {
	isCloseSubBracket := value[i] == '}' && state.inPossibleSub && !state.inStringLiteral
	if isCloseSubBracket {
		// End of a substitution
		parsedSub, err := ParseSubstitution(substitutionContext, state.potentialSub)
		if err != nil {
			state.errors = append(state.errors, err)
		} else {
			state.parsed = append(state.parsed, &StringOrSubstitution{
				SubstitutionValue: parsedSub,
			})
		}
		state.potentialSub = ""
		state.potentialNonSubStr = ""
		state.inPossibleSub = false
	}
	return isCloseSubBracket
}

// ParseSubstitution parses a string that represents a substitution
// that is the contents of an interpolated "${..}" block.
func ParseSubstitution(substitutionContext string, substitutionInput string) (*Substitution, error) {
	tokens, err := lex(substitutionInput)
	if err != nil {
		return nil, err
	}

	parser := NewParser(tokens)
	return parser.Parse()
}
