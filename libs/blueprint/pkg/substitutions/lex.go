package substitutions

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/pkg/source"
)

type lexState struct {
	candidateToken    string
	prevChar          rune
	tokens            []*token
	parentSourceStart *source.Meta
	// Used to assign line and column numbers to tokens,
	// lex errors will not be reported with line and column,
	// the parent source context will have to suffice for mapping
	// locations to lex errors.
	relativeLineInfo *source.Meta
}

type tokenType string

const (
	tokenOpenBracket        tokenType = "openBracket"
	tokenCloseBracket       tokenType = "closeBracket"
	tokenOpenParen          tokenType = "openParen"
	tokenCloseParen         tokenType = "closeParen"
	tokenComma              tokenType = "comma"
	tokenPeriod             tokenType = "period"
	tokenIntLiteral         tokenType = "intLiteral"
	tokenFloatLiteral       tokenType = "floatLiteral"
	tokenBoolLiteral        tokenType = "boolLiteral"
	tokenStringLiteral      tokenType = "stringLiteral"
	tokenNameStringLiteral  tokenType = "nameStringLiteral"
	tokenIdent              tokenType = "identifier"
	tokenKeywordVariables   tokenType = "keywordVariables"
	tokenKeywordDatasources tokenType = "keywordDatasources"
	tokenKeywordResources   tokenType = "keywordResources"
	tokenKeywordChildren    tokenType = "keywordChildren"
)

type token struct {
	tokenType    tokenType
	value        string
	relativeLine int
	relativeCol  int
}

var (
	whiteSpacePattern           = regexp.MustCompile(`\s`)
	boolPattern                 = regexp.MustCompile(`^(true|false)`)
	lexStringLiteralNamePattern = regexp.MustCompile(`([A-Za-z0-9_-]|\.)+`)
)

func lex(sequence string, parentSourceStart *source.Meta) ([]*token, error) {
	lexState := &lexState{
		candidateToken:    "",
		prevChar:          ' ',
		tokens:            []*token{},
		parentSourceStart: parentSourceStart,
		relativeLineInfo: &source.Meta{
			Line:   0,
			Column: 0,
		},
	}

	// TODO: iterate over runes instead of bytes
	// decode rune increment by rune width
	errors := []error{}
	i := 0
	for i < len(sequence) {
		lexUpdateLine(lexState, sequence[i])

		if whiteSpacePattern.MatchString(string(sequence[i])) {
			i += 1
			lexState.relativeLineInfo.Column += 1
			// Use continue to avoid using a complex if else chain
			// for each possible token type.
			continue
		}

		isPunctuation := checkPunctuation(string(sequence[i]), lexState)
		if isPunctuation {
			i += 1
			lexState.relativeLineInfo.Column += 1
			continue
		}

		charsConsumed := checkNumber(sequence, i, lexState)
		if charsConsumed > 0 {
			i += charsConsumed
			lexState.relativeLineInfo.Column += charsConsumed
			continue
		}

		charsConsumed = checkBoolLiteral(sequence, i, lexState)
		if charsConsumed > 0 {
			i += charsConsumed
			lexState.relativeLineInfo.Column += charsConsumed
			continue
		}

		charsConsumed = checkIdentifierOrKeyword(sequence, i, lexState)
		if charsConsumed > 0 {
			i += charsConsumed
			lexState.relativeLineInfo.Column += charsConsumed
			continue
		}

		charsConsumed, err := lexCheckStringLiteral(sequence, i, lexState)
		if err != nil {
			errors = append(errors, err)
		}
		if charsConsumed > 0 {
			i += charsConsumed
			lexState.relativeLineInfo.Column += charsConsumed
			continue
		}

		errors = append(errors, errLexUnexpectedChar(i, sequence[i]))
		i += 1
	}

	if len(errors) > 0 {
		return lexState.tokens, errLexMultiple("lexical analysis failed for substitution", errors)
	}

	return lexState.tokens, nil
}

func checkPunctuation(char string, state *lexState) bool {
	switch char {
	case "[":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenOpenBracket,
			value:        "[",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	case "]":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenCloseBracket,
			value:        "]",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	case "(":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenOpenParen,
			value:        "(",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	case ")":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenCloseParen,
			value:        ")",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	case ",":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenComma,
			value:        ",",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	case ".":
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenPeriod,
			value:        ".",
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return true
	default:
		return false
	}
}

func checkNumber(sequence string, startPos int, state *lexState) int {
	if sequence[startPos] == '-' || (sequence[startPos] >= '0' && sequence[startPos] <= '9') {
		charsConsumed := takeFloatLiteral(state, sequence, startPos)
		if charsConsumed > 0 {
			return charsConsumed
		}

		charsConsumed = takeIntLiteral(state, sequence, startPos)
		return charsConsumed
	}
	return 0
}

func checkIdentifierOrKeyword(sequence string, startPos int, state *lexState) int {
	if isIdentStartChar(sequence[startPos]) {
		return takeIdentifierOrKeyword(state, sequence, startPos+1, sequence[startPos])
	}
	return 0
}

func lexCheckStringLiteral(sequence string, startPos int, state *lexState) (int, error) {
	if sequence[startPos] == '"' {
		return takeStringLiteral(state, sequence, startPos+1)
	}
	return 0, nil
}

func checkBoolLiteral(sequence string, startPos int, state *lexState) int {
	if sequence[startPos] == 't' || sequence[startPos] == 'f' {
		return takeBoolLiteral(state, sequence, startPos)
	}
	return 0
}

func isIdentStartChar(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_'
}

func isIdentChar(char byte) bool {
	return isIdentStartChar(char) || (char >= '0' && char <= '9') || char == '-'
}

func takeFloatLiteral(state *lexState, sequence string, startPos int) int {
	inPossibleFloat := true
	i := startPos
	sign := ""
	intPart := ""
	passedDecimalPoint := false
	fractionalPart := ""

	for inPossibleFloat && i < len(sequence) {
		if sequence[i] == '-' && i == startPos {
			sign = "-"
		} else if sequence[i] == '.' && !passedDecimalPoint {
			passedDecimalPoint = true
		} else if sequence[i] >= '0' && sequence[i] <= '9' {
			updateFloatParts(&intPart, &fractionalPart, passedDecimalPoint, sequence[i])
		} else {
			inPossibleFloat = false
		}

		i += 1
	}

	if !passedDecimalPoint || intPart == "" || fractionalPart == "" {
		// A float literal can not be taken from the current position
		// in the sequence.
		return 0
	}

	value := fmt.Sprintf("%s%s.%s", sign, intPart, fractionalPart)
	state.tokens = append(state.tokens, &token{
		tokenType:    tokenFloatLiteral,
		value:        value,
		relativeLine: state.relativeLineInfo.Line,
		relativeCol:  state.relativeLineInfo.Column,
	})

	return len(value)
}

func updateFloatParts(intPart *string, fractionalPart *string, passedDecimalPoint bool, char byte) {
	if passedDecimalPoint {
		*fractionalPart += string(char)
	} else {
		*intPart += string(char)
	}
}

func takeIntLiteral(state *lexState, sequence string, startPos int) int {
	inPossibleInt := true
	i := startPos
	value := ""

	for inPossibleInt && i < len(sequence) {
		if sequence[i] >= '0' && sequence[i] <= '9' {
			value += string(sequence[i])
		} else {
			inPossibleInt = false
		}

		i += 1
	}

	state.tokens = append(state.tokens, &token{
		tokenType:    tokenIntLiteral,
		value:        value,
		relativeLine: state.relativeLineInfo.Line,
		relativeCol:  state.relativeLineInfo.Column,
	})

	return len(value)
}

func takeStringLiteral(state *lexState, sequence string, startPos int) (int, error) {
	inStringLiteral := true
	i := startPos
	prevChar := ' '
	value := ""
	for inStringLiteral && i < len(sequence) {
		if sequence[i] == '"' && prevChar != '\\' {
			inStringLiteral = false
		} else {
			value += string(sequence[i])
		}
		prevChar = rune(sequence[i])
		i += 1
	}

	if inStringLiteral && i == len(sequence) {
		return i - startPos, errLexUnexpectedEndOfInput("string literal")
	}

	// Differentiate between a string literal and a name string literal
	// to allow the parser to catch errors when unexpected characters are used
	// in a string that is used as a name in a [".."] accessor.
	prevTokenOpenBracket := len(state.tokens) > 0 && state.tokens[len(state.tokens)-1].tokenType == tokenOpenBracket
	if prevTokenOpenBracket && lexStringLiteralNamePattern.MatchString(value) {
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenNameStringLiteral,
			value:        strings.Replace(value, "\\\"", "\"", -1),
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
		return len(value) + 2, nil
	}

	state.tokens = append(state.tokens, &token{
		tokenType:    tokenStringLiteral,
		value:        strings.Replace(value, "\\\"", "\"", -1),
		relativeLine: state.relativeLineInfo.Line,
		relativeCol:  state.relativeLineInfo.Column,
	})

	// Add 2 to account for the quotes.
	return len(value) + 2, nil
}

func takeIdentifierOrKeyword(state *lexState, sequence string, restStartPos int, startChar byte) int {
	inPossibleIdent := true
	i := restStartPos
	value := string(startChar)
	for inPossibleIdent && i < len(sequence) {
		if isIdentChar(sequence[i]) {
			value += string(sequence[i])
		} else {
			inPossibleIdent = false
		}
		i += 1
	}

	tType := deriveIdentOrKeywordTokenType(value)
	state.tokens = append(state.tokens, &token{
		tokenType:    tType,
		value:        value,
		relativeLine: state.relativeLineInfo.Line,
		relativeCol:  state.relativeLineInfo.Column,
	})

	return len(value)
}

func deriveIdentOrKeywordTokenType(value string) tokenType {
	switch value {
	case "variables":
		return tokenKeywordVariables
	case "datasources":
		return tokenKeywordDatasources
	case "resources":
		return tokenKeywordResources
	case "children":
		return tokenKeywordChildren
	default:
		return tokenIdent
	}
}

func takeBoolLiteral(state *lexState, sequence string, startPos int) int {
	subSequence := sequence[startPos:]
	value := boolPattern.FindString(subSequence)
	if len(value) > 0 {
		state.tokens = append(state.tokens, &token{
			tokenType:    tokenBoolLiteral,
			value:        value,
			relativeLine: state.relativeLineInfo.Line,
			relativeCol:  state.relativeLineInfo.Column,
		})
	}

	return len(value)
}

func lexUpdateLine(state *lexState, char byte) {
	if char == '\n' {
		state.relativeLineInfo.Line += 1
		state.relativeLineInfo.Column = 1
	}
}
