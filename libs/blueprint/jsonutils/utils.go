package jsonutils

import "strings"

// EscapeJSONString escapes a string to be used in a marshalled JSON string.
func EscapeJSONString(str string) string {
	escapedNewLine := strings.ReplaceAll(str, "\n", "\\n")
	escapedBackSlash := strings.ReplaceAll(escapedNewLine, "\\", "\\\\")
	escapedForwardSlash := strings.ReplaceAll(escapedBackSlash, "/", "\\/")
	escapedDoubleQuote := strings.ReplaceAll(escapedForwardSlash, "\"", "\\\"")
	escapedAmpersand := strings.ReplaceAll(escapedDoubleQuote, "&", "\\&")
	escapedReturn := strings.ReplaceAll(escapedAmpersand, "\r", "\\r")
	escapedTab := strings.ReplaceAll(escapedReturn, "\t", "\\t")
	escapedBackspace := strings.ReplaceAll(escapedTab, "\b", "\\b")
	return strings.ReplaceAll(escapedBackspace, "\f", "\\f")
}

// UnescapeJSONString unescapes a string to be used in unmarshalling a JSON string.
func UnescapeJSONString(str string) string {
	unescapedNewLine := strings.ReplaceAll(str, "\\n", "\n")
	unescapedBackSlash := strings.ReplaceAll(unescapedNewLine, "\\\\", "\\")
	unescapedForwardSlash := strings.ReplaceAll(unescapedBackSlash, "\\/", "/")
	unescapedDoubleQuote := strings.ReplaceAll(unescapedForwardSlash, "\\\"", "\"")
	unescapedAmpersand := strings.ReplaceAll(unescapedDoubleQuote, "\\&", "&")
	unescapedReturn := strings.ReplaceAll(unescapedAmpersand, "\\r", "\r")
	unescapedTab := strings.ReplaceAll(unescapedReturn, "\\t", "\t")
	unescapedBackspace := strings.ReplaceAll(unescapedTab, "\\b", "\b")
	return strings.ReplaceAll(unescapedBackspace, "\\f", "\f")
}
