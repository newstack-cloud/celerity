package core

import (
	"fmt"
	"strings"
)

func escapeRegexpSpecialChars(input string) string {
	// Escape special characters in the input string for use in a regular expression.
	// This is a simple implementation that escapes common regex special characters.
	specialChars := `\.*+?^${}()|[]`
	output := input
	for _, char := range specialChars {
		output = strings.ReplaceAll(input, string(char), fmt.Sprintf("\\%s", string(char)))
	}
	return output
}
