use std::fmt;

use celerity_helpers::scanner::{Scanner, ScannerAction, ScannerError};
use serde::{
    de::{self, Visitor},
    Deserialize, Deserializer,
};

use crate::blueprint_with_subs::{
    StringOrSubstitution, StringOrSubstitutions, Substitution, SubstitutionVariableReference,
};

impl<'de> Deserialize<'de> for StringOrSubstitutions {
    fn deserialize<D>(deserializer: D) -> Result<StringOrSubstitutions, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(StringOrSubstitutionsVisitor)
    }
}

struct StringOrSubstitutionsVisitor;

impl<'de> Visitor<'de> for StringOrSubstitutionsVisitor {
    type Value = StringOrSubstitutions;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("a valid string or ${..} substitution")
    }

    fn visit_string<E>(self, value: String) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        parse_substitutions(&value)
    }

    fn visit_str<E>(self, value: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        parse_substitutions(value)
    }
}

struct InterpolationParseState {
    parsed: Vec<StringOrSubstitution>,
    in_possible_sub: bool,
    in_string_literal: bool,
    potential_sub: String,
    potential_non_sub_str: String,
    prev_char: char,
}

pub fn parse_substitutions<E>(value: &str) -> Result<StringOrSubstitutions, E>
where
    E: de::Error,
{
    // There are no openings for substitutions, the value is just a string literal.
    // This may not be the case if the string literal contains "${" in which case
    // it will be caught in the process of evaluating every character in sequence.
    if !value.contains("${") {
        return Ok(StringOrSubstitutions {
            values: vec![StringOrSubstitution::StringValue(value.to_string())],
        });
    }

    let mut state = InterpolationParseState {
        parsed: vec![],
        in_possible_sub: false,
        in_string_literal: false,
        potential_sub: "".to_string(),
        potential_non_sub_str: "".to_string(),
        prev_char: ' ',
    };

    for ch in value.chars() {
        let is_open_sub_bracket = check_open_sub_bracket(&mut state, ch);
        check_string_literal(&mut state, ch);
        let close_check_result = check_close_sub_bracket(&mut state, ch)?;

        state.prev_char = ch;
        match close_check_result {
            CheckCloseSubBracketResult::Supported => {
                // Do nothing for a matched substitution,
                // state will have been reset by the check_close_sub_bracket call.
            }
            CheckCloseSubBracketResult::Unsupported => {
                state.potential_non_sub_str.push(ch);
                // Replace the previous string value with the unsupported substitution string
                // that will already contain the previous string value.
                let last_parsed_value = state.parsed.last();
                if let Some(StringOrSubstitution::StringValue(_)) = last_parsed_value.cloned() {
                    state.parsed.pop();
                    state.parsed.push(StringOrSubstitution::StringValue(
                        state.potential_non_sub_str,
                    ));
                    state.potential_non_sub_str = "".to_string();
                }
            }
            CheckCloseSubBracketResult::Not => {
                state.potential_non_sub_str.push(ch);
            }
        }
        if state.in_possible_sub && !is_open_sub_bracket {
            state.potential_sub.push(ch);
        }
    }

    // If the input value is a string interpolated with substitutions,
    // make sure we capture the end of the string if it's not a substitution.
    if !state.potential_non_sub_str.is_empty() {
        state.parsed.push(StringOrSubstitution::StringValue(
            state.potential_non_sub_str,
        ));
    }

    Ok(StringOrSubstitutions {
        values: concat_adjacent_string_values(state.parsed),
    })
}

// When dealing with string interpolations for substitutions that are not supported
// by the runtime parser, we need to concatenate the adjacent string values as a part
// of the approach to gracefully handle unsupported substitutions.
fn concat_adjacent_string_values(values: Vec<StringOrSubstitution>) -> Vec<StringOrSubstitution> {
    let mut result = vec![];
    let mut last_string_value = "".to_string();
    for value in values {
        match value {
            StringOrSubstitution::StringValue(str_value) => {
                last_string_value += str_value.as_str();
            }
            StringOrSubstitution::SubstitutionValue(_) => {
                if !last_string_value.is_empty() {
                    result.push(StringOrSubstitution::StringValue(last_string_value));
                    last_string_value = "".to_string();
                }
                result.push(value);
            }
        }
    }

    if !last_string_value.is_empty() {
        result.push(StringOrSubstitution::StringValue(last_string_value));
    }

    result
}

fn check_open_sub_bracket(state: &mut InterpolationParseState, ch: char) -> bool {
    let is_open_sub_bracket = state.prev_char == '$' && ch == '{' && !state.in_string_literal;
    if is_open_sub_bracket {
        // Start of a substitution.
        state.in_possible_sub = true;
        let non_sub_str = &state.potential_non_sub_str[0..state.potential_non_sub_str.len() - 1];
        if !non_sub_str.is_empty() {
            state
                .parsed
                .push(StringOrSubstitution::StringValue(non_sub_str.to_string()))
        }
    }
    is_open_sub_bracket
}

fn check_string_literal(state: &mut InterpolationParseState, ch: char) {
    if ch == '"' && state.prev_char != '\\' && state.in_possible_sub {
        state.in_string_literal = !state.in_string_literal;
    }
}

enum CheckCloseSubBracketResult {
    Supported,
    Unsupported,
    Not,
}

fn check_close_sub_bracket<E>(
    state: &mut InterpolationParseState,
    ch: char,
) -> Result<CheckCloseSubBracketResult, E>
where
    E: de::Error,
{
    let is_close_sub_bracket = ch == '}' && state.in_possible_sub && !state.in_string_literal;
    if is_close_sub_bracket && state.potential_sub.starts_with("variables") {
        // We check for the "variables" prefix to avoid parsing unsupported substitutions
        // and differentiating between errors in parsing a variable reference and trying to
        // parse a substitution that is not supported (e.g. values.*, datasources.* etc.).
        // End of a supported substitution, let's parse it.
        let mut scanner = Scanner::new(&state.potential_sub);
        let parsed_sub = parse_substitution(&mut scanner).map_err(de::Error::custom)?;
        state
            .parsed
            .push(StringOrSubstitution::SubstitutionValue(parsed_sub));
        state.potential_sub = "".to_string();
        state.potential_non_sub_str = "".to_string();
        state.in_possible_sub = false;
    } else if is_close_sub_bracket {
        // End of a substitution, but it's not a supported substitution.
        return Ok(CheckCloseSubBracketResult::Unsupported);
    }

    Ok(if is_close_sub_bracket {
        CheckCloseSubBracketResult::Supported
    } else {
        CheckCloseSubBracketResult::Not
    })
}

#[derive(Debug)]
pub enum ParseError {
    Character(ParseErrorInfo),
    EndOfInput,
}

impl de::Error for ParseError {
    fn custom<T: fmt::Display>(msg: T) -> Self {
        ParseError::Character(ParseErrorInfo {
            pos: 0,
            expected: msg.to_string(),
        })
    }
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ParseError::Character(info) => {
                let expected_suffix = if info.expected.is_empty() {
                    "".to_string()
                } else {
                    format!(", expected {}", info.expected)
                };
                write!(
                    f,
                    "parse error at position {}{} in the ${{..}} substitution",
                    info.pos, expected_suffix
                )
            }
            ParseError::EndOfInput => write!(f, "parse error: unexpected end of input"),
        }
    }
}

impl std::error::Error for ParseError {}

#[derive(Debug)]
pub struct ParseErrorInfo {
    pub pos: usize,
    pub expected: String,
}

impl From<ScannerError> for ParseError {
    fn from(error: ScannerError) -> Self {
        match error {
            ScannerError::Character(pos) => ParseError::Character(ParseErrorInfo {
                pos,
                expected: "".to_string(),
            }),
            ScannerError::EndOfInput => ParseError::EndOfInput,
        }
    }
}

fn parse_substitution(scanner: &mut Scanner) -> Result<Substitution, ParseError> {
    substitution(scanner)
}

fn substitution(scanner: &mut Scanner) -> Result<Substitution, ParseError> {
    // Consume any whitespace before the keyword.
    consume_whitespace(scanner);

    variables_keyword(scanner)?;

    let var_name = match variable_name_accessor(scanner) {
        Ok(var_name) => var_name,
        Err(err) => {
            scanner.backtrack();
            return Err(err);
        }
    };

    Ok(Substitution::VariableReference(
        SubstitutionVariableReference {
            variable_name: var_name,
        },
    ))
}

fn variables_keyword(scanner: &mut Scanner) -> Result<(), ParseError> {
    scanner.save_pos();
    let scan_result = scanner.scan(|input| match input {
        "v" => Some(ScannerAction::Require),
        "va" => Some(ScannerAction::Require),
        "var" => Some(ScannerAction::Require),
        "vari" => Some(ScannerAction::Require),
        "varia" => Some(ScannerAction::Require),
        "variab" => Some(ScannerAction::Require),
        "variabl" => Some(ScannerAction::Require),
        "variable" => Some(ScannerAction::Require),
        "variables" => Some(ScannerAction::Return(())),
        _ => None,
    });
    match scan_result {
        Ok(Some(_)) => {
            scanner.pop_pos();
            Ok(())
        }
        Ok(None) => {
            let error_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::Character(ParseErrorInfo {
                pos: error_pos + 1,
                expected: "variables keyword".to_string(),
            }))
        }
        Err(err) => Err(err.into()),
    }
}

fn variable_name_accessor(scanner: &mut Scanner) -> Result<String, ParseError> {
    scanner.save_pos();

    if scanner.take(&'.') {
        return match identifier(scanner) {
            Ok(var_name) => Ok(var_name),
            Err(err) => {
                scanner.backtrack();
                return Err(err);
            }
        };
    }

    if scanner.peek() == Some(&'[') {
        return match string_literal_identifier(scanner) {
            Ok(index) => Ok(index),
            Err(err) => {
                scanner.backtrack();
                return Err(err);
            }
        };
    }

    Err(ParseError::Character(ParseErrorInfo {
        pos: scanner.pos(),
        expected: "variable name accessor of the form .<identifier> or [\"<identifier>\"]"
            .to_string(),
    }))
}

fn identifier(scanner: &mut Scanner) -> Result<String, ParseError> {
    let mut var_name = String::new();
    if let Some(next_char) = scanner.peek() {
        if is_identifier_start_char(next_char) {
            var_name.push(*next_char);
            scanner.pop();
            while let Some(ch) = scanner.peek() {
                if is_identifier_char(ch) {
                    var_name.push(*ch);
                    scanner.pop();
                } else {
                    break;
                }
            }

            return Ok(var_name);
        } else {
            return Err(ParseError::Character(ParseErrorInfo {
                pos: scanner.pos(),
                expected: "identifier start character".to_string(),
            }));
        }
    }

    Err(ParseError::Character(ParseErrorInfo {
        pos: scanner.pos(),
        expected: "identifier start character".to_string(),
    }))
}

fn is_identifier_start_char(ch: &char) -> bool {
    ch.is_alphabetic() || *ch == '_'
}

fn is_identifier_char(ch: &char) -> bool {
    ch.is_alphanumeric() || *ch == '_' || *ch == '-'
}

fn string_literal_identifier(scanner: &mut Scanner) -> Result<String, ParseError> {
    if !scanner.take(&'[') {
        return Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos(),
            expected: "opening square bracket".to_string(),
        }));
    }

    if !scanner.take(&'"') {
        return Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos(),
            expected: "opening double quote".to_string(),
        }));
    }

    let mut var_name = String::new();
    while let Some(ch) = scanner.peek() {
        if is_string_literal_ident_char(ch) {
            var_name.push(*ch);
            scanner.pop();
        } else {
            break;
        }
    }

    if !scanner.take(&'"') {
        return Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos(),
            expected: "closing double quote".to_string(),
        }));
    }

    if !scanner.take(&']') {
        return Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos(),
            expected: "closing square bracket".to_string(),
        }));
    }

    Ok(var_name)
}

fn is_string_literal_ident_char(ch: &char) -> bool {
    is_identifier_char(ch) || *ch == '.'
}

fn consume_whitespace(scanner: &mut Scanner) {
    while let Some(ch) = scanner.peek() {
        if ch.is_whitespace() {
            scanner.pop();
        } else {
            break;
        }
    }
}

#[cfg(test)]
mod tests {

    use super::*;

    use pretty_assertions::assert_eq;

    #[test]
    fn test_correctly_parses_string_with_multiple_var_substitutions() {
        let input = "\"This is a ${variables.string1} with \
            ${variables[\\\"multiple.v1\\\"]} substitutions\"";
        let result = serde_json::from_str::<StringOrSubstitutions>(input);

        assert!(result.is_ok());
        let string_with_subs = result.unwrap();
        assert_eq!(
            string_with_subs,
            StringOrSubstitutions {
                values: vec![
                    StringOrSubstitution::StringValue("This is a ".to_string()),
                    StringOrSubstitution::SubstitutionValue(Substitution::VariableReference(
                        SubstitutionVariableReference {
                            variable_name: "string1".to_string(),
                        },
                    )),
                    StringOrSubstitution::StringValue(" with ".to_string()),
                    StringOrSubstitution::SubstitutionValue(Substitution::VariableReference(
                        SubstitutionVariableReference {
                            variable_name: "multiple.v1".to_string(),
                        },
                    )),
                    StringOrSubstitution::StringValue(" substitutions".to_string()),
                ],
            }
        )
    }

    #[test]
    fn test_correctly_ignores_unsupported_substitutions() {
        // The current iteration of the parser only supports variable substitutions
        // but should not fail when encountering unsupported substitutions.
        let input = "\"This is a ${variables.string1} with \
            ${values[\\\"unsupported.v1\\\"]} ${datasources.unsupported} substitutions\"";
        let result = serde_json::from_str::<StringOrSubstitutions>(input);
        assert!(result.is_ok());
        let string_with_subs = result.unwrap();
        assert_eq!(
            string_with_subs,
            StringOrSubstitutions {
                values: vec![
                    StringOrSubstitution::StringValue("This is a ".to_string()),
                    StringOrSubstitution::SubstitutionValue(Substitution::VariableReference(
                        SubstitutionVariableReference {
                            variable_name: "string1".to_string(),
                        },
                    )),
                    StringOrSubstitution::StringValue(
                        " with ${values[\"unsupported.v1\"]} ${datasources.unsupported} substitutions".to_string()
                    ),
                ],
            }
        )
    }

    #[test]
    fn test_correctly_parses_a_single_variable_reference_substitution() {
        let input = "\"${variables.databaseName}\"";
        let result = serde_json::from_str::<StringOrSubstitutions>(input);
        assert!(result.is_ok());
        let string_with_subs = result.unwrap();
        assert_eq!(
            string_with_subs,
            StringOrSubstitutions {
                values: vec![StringOrSubstitution::SubstitutionValue(
                    Substitution::VariableReference(SubstitutionVariableReference {
                        variable_name: "databaseName".to_string(),
                    },)
                ),],
            }
        )
    }

    #[test]
    fn test_correctly_parses_string_without_substitutions() {
        let input = "\"This is a string without substitutions\"";
        let result = serde_json::from_str::<StringOrSubstitutions>(input);
        assert!(result.is_ok());
        let string_with_subs = result.unwrap();
        assert_eq!(
            string_with_subs,
            StringOrSubstitutions {
                values: vec![StringOrSubstitution::StringValue(
                    "This is a string without substitutions".to_string()
                ),],
            }
        )
    }

    #[test]
    fn test_fails_to_parse_invalid_variable_substitution() {
        let input = "\"This is invalid: ${variables.-120392}\"";
        let result = serde_json::from_str::<StringOrSubstitutions>(input);
        assert!(result.is_err());
        let error = result.err().unwrap();
        assert_eq!(
            error.to_string(),
            "parse error at position 10, expected identifier start character in the ${..} substitution at line 1 column 39",
        );
    }
}
