package substitutions

import (
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/libs/blueprint/source"
	"gopkg.in/yaml.v3"
)

// DetermineYAMLSourceStartMeta is a utility function that determines the source start meta
// to use as the accurate starting point for counting lines and columns for interpolated
// substitutions in YAML documents.
//
// For "literal" style blocks, "|\s*\n" must be accounted for.
// For "folded" style blocks, ">\s*\n" must be accounted for.
func DetermineYAMLSourceStartMeta(node *yaml.Node, sourceMeta *source.Meta) *source.Meta {
	if node.Kind != yaml.ScalarNode {
		return sourceMeta
	}

	if node.Style == yaml.LiteralStyle {
		return &source.Meta{
			Position: source.Position{
				Line:   sourceMeta.Line + 1,
				Column: sourceMeta.Column,
			},
			EndPosition: sourceMeta.EndPosition,
		}
	}

	if node.Style == yaml.FoldedStyle {
		return &source.Meta{
			Position: source.Position{
				Line:   sourceMeta.Line + 1,
				Column: sourceMeta.Column,
			},
			EndPosition: sourceMeta.EndPosition,
		}
	}

	return sourceMeta
}

// ContainsSubstitution checks if a string contains a ${..} substitution.
func ContainsSubstitution(str string) bool {
	openIndex := strings.Index(str, "${")
	closeIndex := strings.Index(str, "}")
	return openIndex > -1 && closeIndex > openIndex
}

func GetYAMLNodePrecedingCharCount(node *yaml.Node) int {
	if node.Kind == yaml.ScalarNode &&
		node.Style == yaml.DoubleQuotedStyle || node.Style == yaml.SingleQuotedStyle {
		return 1
	}

	return 0
}

// RenderFieldPath renders a field path with the given current path and field name.
func RenderFieldPath(currentPath, fieldName string) string {
	if currentPath == "" {
		return fieldName
	}
	if NamePattern.MatchString(fieldName) {
		return fmt.Sprintf("%s.%s", currentPath, fieldName)
	}

	return fmt.Sprintf("%s[\"%s\"]", currentPath, fieldName)
}
