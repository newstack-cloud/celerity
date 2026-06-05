use super::core::Parser;
use crate::errors::ParseError;
use crate::expr_transform::{expr_to_mapping_node, object_to_string_or_substitutions_map};
use crate::mapping::MappingNode;
use crate::parser::expr::{Expr, ObjectField};
use crate::schema::NamedMap;
use crate::source::{Position, Span};
use crate::substitution::StringOrSubstitutions;
use crate::tokens::TokenType;

impl Parser {
    /// `metadata { … }` envelope: advances the keyword, runs the block, and
    /// returns the keyword-anchored span.
    pub(crate) fn parse_metadata_keyword_block<F>(
        &mut self,
        parse_field: F,
    ) -> Result<Span, ParseError>
    where
        F: FnMut(&mut Self) -> Result<(), ParseError>,
    {
        let open = self.advance(); // 'metadata'
        let block = self.parse_brace_block(parse_field)?;
        Ok(Span::new(open.start, block.end.unwrap_or(block.start)))
    }

    /// `= <interpolated-string>` for a `displayName`.
    pub(crate) fn parse_display_name_field(&mut self) -> Result<StringOrSubstitutions, ParseError> {
        self.expect(TokenType::Assign)?;
        self.parse_interpolated_string()
    }

    /// `= { key = expr, … }` for `annotations`.
    pub(crate) fn parse_annotations_field(
        &mut self,
    ) -> Result<NamedMap<StringOrSubstitutions>, ParseError> {
        let entries = self.parse_object_literal_assignment("annotations")?;
        Ok(object_to_string_or_substitutions_map(entries))
    }

    /// `= <expr>` for `custom` - any expression that maps to a MappingNode.
    pub(crate) fn parse_custom_field(&mut self) -> Result<MappingNode, ParseError> {
        self.expect(TokenType::Assign)?;
        let e = self.parse_expr()?;
        Ok(expr_to_mapping_node(e))
    }

    /// Consumes `= <expr>` and requires the value to be an object literal,
    /// returning its entries (for fields whose schema is a string-keyed map).
    pub(crate) fn parse_object_literal_assignment(
        &mut self,
        field: &str,
    ) -> Result<Vec<ObjectField>, ParseError> {
        self.expect(TokenType::Assign)?;
        let e = self.parse_expr()?;
        match e {
            Expr::Object { entries, .. } => Ok(entries),
            other => {
                let pos = other.span().map(|s| s.start).unwrap_or(Position::new(1, 1));
                Err(self.error_at(pos, format!("{field:?} must be an object literal")))
            }
        }
    }

    /// Reads the `key =` of a field assignment, returning the key and its span.
    pub(crate) fn parse_field_assignment(&mut self) -> Result<(String, Span), ParseError> {
        let (key, span) = self.parse_field_key()?;
        self.expect(TokenType::Assign)?;
        Ok((key, span))
    }

    /// Parses a `{ key = expr ... }` block as a free-form object mapping node
    /// (top-level metadata, resource `spec`, include `variables`/`metadata`).
    pub(crate) fn parse_free_form_map_block(&mut self) -> Result<MappingNode, ParseError> {
        let object = self.parse_object_expr()?;
        Ok(expr_to_mapping_node(object))
    }

    /// `{ entry <sep> entry … }` — the brace + separator mechanics shared by
    /// object literals and declaration blocks. Entries are separated by commas
    /// or newlines, with an optional trailing separator. `parse_entry` receives
    /// the parser so it can call `&mut self` methods. Returns the `{…}` span.
    pub(crate) fn parse_brace_block<F>(&mut self, mut parse_entry: F) -> Result<Span, ParseError>
    where
        F: FnMut(&mut Self) -> Result<(), ParseError>,
    {
        let open = self.expect(TokenType::LeftBrace)?;
        self.consume_separators(); // leading newlines/commas after '{'
        while !matches!(self.peek_type(), TokenType::RightBrace | TokenType::Eof) {
            parse_entry(self)?;
            if !self.consume_separators() {
                break; // no separator -> next must be '}'
            }
        }
        let close = self.expect(TokenType::RightBrace)?;
        Ok(Span::new(open.start, close.end))
    }

    /// `[ element , element … ]` — the same mechanics for array literals.
    /// Newlines inside `[ ]` are insignificant (the cursor skips them while the
    /// grouping depth is raised), so `consume_separators` mostly eats commas
    /// here. An optional trailing comma is permitted. Returns the `[…]` span.
    pub(crate) fn parse_array<F>(&mut self, mut parse_element: F) -> Result<Span, ParseError>
    where
        F: FnMut(&mut Self) -> Result<(), ParseError>,
    {
        let open = self.expect(TokenType::LeftBracket)?;
        self.consume_separators();
        while !matches!(self.peek_type(), TokenType::RightBracket | TokenType::Eof) {
            parse_element(self)?;
            if !self.consume_separators() {
                break;
            }
        }
        let close = self.expect(TokenType::RightBracket)?;
        Ok(Span::new(open.start, close.end))
    }
}
