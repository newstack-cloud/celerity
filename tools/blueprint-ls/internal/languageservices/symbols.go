package languageservices

import (
	"fmt"
	"strings"

	"github.com/two-hundred/celerity/tools/blueprint-ls/internal/blueprint"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// SymbolService is a service that provides functionality
// for document symbols for an LSP client.
type SymbolService struct {
	state  *State
	logger *zap.Logger
}

// NewSymbolService creates a new service for document symbols.
func NewSymbolService(
	state *State,
	logger *zap.Logger,
) *SymbolService {
	return &SymbolService{
		state,
		logger,
	}
}

// GetDocumentSymbols returns the symbols in the given blueprint schema.
func (s *SymbolService) GetDocumentSymbols(
	docURI lsp.URI,
	content string,
) ([]lsp.DocumentSymbol, error) {

	symbols := []lsp.DocumentSymbol{}

	yamlNode := s.state.GetDocumentYAMLNode(docURI)
	if yamlNode == nil {
		var err error
		yamlNode, err = blueprint.ParseYAMLNode(content)
		if err != nil {
			return nil, err
		}
		s.state.SetDocumentYAMLNode(docURI, yamlNode)
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	numberOfLines := len(lines)
	s.collectDocumentSymbols("document", yamlNode, &symbols, nil, numberOfLines)

	return symbols, nil
}

func (s *SymbolService) collectDocumentSymbols(
	name string,
	node *yaml.Node,
	symbols *[]lsp.DocumentSymbol,
	next *yaml.Node,
	numberOfLinesInDocument int,
) {
	if node.Kind == yaml.DocumentNode {
		symbolRange := yamlNodeToLSPRange(node, nil)
		symbolRange.End.Line = lsp.UInteger(numberOfLinesInDocument)
		symbol := lsp.DocumentSymbol{
			Name:           name,
			Kind:           lsp.SymbolKindFile,
			Range:          symbolRange,
			SelectionRange: symbolRange,
			Children:       []lsp.DocumentSymbol{},
		}
		for i, child := range node.Content {
			childNext := next
			if i+1 < len(node.Content) {
				childNext = node.Content[i+1]
			}
			s.collectDocumentSymbols(
				"content",
				child,
				&symbol.Children,
				childNext,
				numberOfLinesInDocument,
			)
		}
		*symbols = append(*symbols, symbol)
		return
	}

	if node.Kind == yaml.MappingNode {
		symbolRange := yamlNodeToLSPRange(node, next)
		if next == nil {
			symbolRange.End.Line = lsp.UInteger(numberOfLinesInDocument)
		}

		symbol := lsp.DocumentSymbol{
			Name:           name,
			Kind:           lsp.SymbolKindObject,
			Range:          symbolRange,
			SelectionRange: symbolRange,
			Children:       []lsp.DocumentSymbol{},
		}

		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			childNext := next
			if i+2 < len(node.Content) {
				childNext = node.Content[i+2]
			}

			s.collectDocumentSymbols(
				key.Value,
				value,
				&symbol.Children,
				childNext,
				numberOfLinesInDocument,
			)
		}

		*symbols = append(*symbols, symbol)
		return
	}

	if node.Kind == yaml.SequenceNode {
		symbolRange := yamlNodeToLSPRange(node, next)
		if next == nil {
			symbolRange.End.Line = lsp.UInteger(numberOfLinesInDocument)
		}

		symbol := lsp.DocumentSymbol{
			Name:           name,
			Kind:           lsp.SymbolKindArray,
			Range:          symbolRange,
			SelectionRange: symbolRange,
			Children:       []lsp.DocumentSymbol{},
		}

		for i := 0; i < len(node.Content); i += 1 {
			item := node.Content[i]

			childNext := next
			if i+1 < len(node.Content) {
				childNext = node.Content[i+1]
			}

			s.collectDocumentSymbols(
				fmt.Sprintf("[%d]", i),
				item,
				&symbol.Children,
				childNext,
				numberOfLinesInDocument,
			)
		}

		*symbols = append(*symbols, symbol)
		return
	}

	if node.Kind == yaml.ScalarNode {
		symbolRange := yamlNodeToLSPRange(node, next)
		if next == nil {
			symbolRange.End.Line = lsp.UInteger(numberOfLinesInDocument)
		}

		symbolKind := determineYAMLScalarSymbolKind(node)

		symbol := lsp.DocumentSymbol{
			Name:           name,
			Kind:           symbolKind,
			Range:          symbolRange,
			SelectionRange: symbolRange,
		}

		*symbols = append(*symbols, symbol)
	}
}

func yamlNodeToLSPRange(node *yaml.Node, next *yaml.Node) lsp.Range {
	start := lsp.Position{
		// yaml.v3 package uses 1-based line and column numbers,
		// LSP uses 0-based line and column numbers.
		Line:      lsp.UInteger(node.Line - 1),
		Character: lsp.UInteger(node.Column - 1),
	}

	end := lsp.Position{}
	if next != nil {
		end.Line = lsp.UInteger(next.Line - 1)
		end.Character = lsp.UInteger(next.Column - 1)
	}

	return lsp.Range{
		Start: start,
		End:   end,
	}
}

func determineYAMLScalarSymbolKind(node *yaml.Node) lsp.SymbolKind {
	switch node.Tag {
	case "!!int":
		return lsp.SymbolKindNumber
	case "!!bool":
		return lsp.SymbolKindBoolean
	case "!!null":
		return lsp.SymbolKindNull
	case "!!float":
		return lsp.SymbolKindNumber
	default:
		return lsp.SymbolKindString
	}
}
