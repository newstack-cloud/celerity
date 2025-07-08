use std::{fmt, str::FromStr};

use celerity_helpers::scanner::{Scanner, ScannerAction, ScannerError};
use jsonpath_rust::{parser::JsonPathParserError, JsonPath};

#[derive(Debug, PartialEq)]
pub struct TemplateFunctionCall {
    pub name: String,
    pub args: Vec<TemplateFunctionExpr>,
}

#[derive(Debug, PartialEq)]
pub enum TemplateFunctionExpr {
    Str(String),
    Int(i64),
    Float(f64),
    Bool(bool),
    Null,
    JsonPath(JsonPath),
    FuncCall(TemplateFunctionCall),
}

#[derive(Debug)]
pub enum ParseError {
    Character(ParseErrorInfo),
    JsonPath(JsonPathParseErrorInfo),
    EndOfInput,
}

#[derive(Debug)]
pub struct ParseErrorInfo {
    pub pos: usize,
    pub expected: String,
}

#[derive(Debug)]
pub struct JsonPathParseErrorInfo {
    pub pos: usize,
    pub expected: String,
    pub error: JsonPathParserError,
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

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ParseError::Character(info) => {
                let expected_suffix = if info.expected.is_empty() {
                    "".to_string()
                } else {
                    format!(", expected {}", info.expected)
                };
                write!(f, "parse error at position {}{}", info.pos, expected_suffix)
            }
            ParseError::JsonPath(info) => {
                write!(
                    f,
                    "parse error at position {}: expected {}, error: {}",
                    info.pos, info.expected, info.error
                )
            }
            ParseError::EndOfInput => write!(f, "parse error: unexpected end of input"),
        }
    }
}

/// Parse a template function call used in a payload template.
pub fn parse_func(scanner: &mut Scanner) -> Result<TemplateFunctionCall, ParseError> {
    func_call(scanner)
}

fn func_call(scanner: &mut Scanner) -> Result<TemplateFunctionCall, ParseError> {
    scanner.save_pos();
    let name = match func_name(scanner) {
        Ok(name) => name,
        Err(err) => {
            scanner.backtrack();
            return Err(err);
        }
    };
    let args = match func_args(scanner) {
        Ok(args) => {
            // Given that the entire function call has been successfully parsed,
            // we can discard the saved position.
            scanner.pop_pos();
            args
        }
        Err(err) => {
            scanner.backtrack();
            return Err(err);
        }
    };

    Ok(TemplateFunctionCall { name, args })
}

fn func_name(scanner: &mut Scanner) -> Result<String, ParseError> {
    let mut name = String::new();

    while let Some(ch) = scanner.peek() {
        if name.is_empty() {
            if ch.is_whitespace() {
                scanner.pop();
            } else if ch.is_alphabetic() || *ch == '_' {
                name.push(*ch);
                scanner.pop();
            } else {
                return Err(ParseError::Character(ParseErrorInfo {
                    pos: scanner.pos() + 1,
                    expected: "a valid function name".to_string(),
                }));
            }
        } else if ch.is_alphabetic() || ch.is_ascii_digit() || *ch == '_' {
            name.push(*ch);
            scanner.pop();
        } else {
            break;
        }
    }

    if name.is_empty() {
        Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos() + 1,
            expected: "a valid function name".to_string(),
        }))
    } else {
        Ok(name)
    }
}

fn func_args(scanner: &mut Scanner) -> Result<Vec<TemplateFunctionExpr>, ParseError> {
    let mut args = Vec::new();

    // Consume any whitespace before the opening parenthesis.
    consume_whitespace(scanner);

    // Consume the opening parenthesis.
    if !scanner.take(&'(') {
        return Err(ParseError::Character(ParseErrorInfo {
            pos: scanner.pos() + 1,
            expected: "\"(\" after function name".to_string(),
        }));
    }

    while let Some(ch) = scanner.peek() {
        if ch.is_whitespace() {
            scanner.pop();
        } else if *ch == ')' {
            scanner.pop();
            break;
        } else {
            let arg = func_arg(scanner)?;
            args.push(arg);

            // Consume any whitespace after the argument.
            consume_whitespace(scanner);

            // Consume the comma if there are more arguments.
            if !scanner.take(&',') {
                if let Some(next) = scanner.peek() {
                    if *next != ')' {
                        return Err(ParseError::Character(ParseErrorInfo {
                            pos: scanner.pos() + 1,
                            expected: "\")\" after the last function argument".to_string(),
                        }));
                    }
                }
            }
        }
    }

    Ok(args)
}

fn func_arg(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    let func_call_result = func_call(scanner);
    if let Ok(func_call) = func_call_result {
        return Ok(TemplateFunctionExpr::FuncCall(func_call));
    }

    let json_path_result = json_path_expr(scanner);
    if let Ok(json_path) = json_path_result {
        return Ok(json_path);
    } else if let Err(ParseError::JsonPath(_)) = json_path_result {
        // In the case the next character is "$" but the JSON Path expression is invalid,
        // exit early to give a more informative error message indicating that the JSON Path
        // expression is invalid.
        if let Some(ch) = scanner.peek() {
            if *ch == '$' {
                return json_path_result;
            }
        }
    }

    let bool_literal_result = bool_literal(scanner);
    if let Ok(bool_literal) = bool_literal_result {
        return Ok(bool_literal);
    }

    let null_literal_result = null_literal(scanner);
    if let Ok(null_literal) = null_literal_result {
        return Ok(null_literal);
    }

    let float_literal_result = float_literal(scanner);
    if let Ok(float_literal) = float_literal_result {
        return Ok(float_literal);
    }

    let int_literal_result = int_literal(scanner);
    if let Ok(int_literal) = int_literal_result {
        return Ok(int_literal);
    }

    let string_literal_result = string_literal(scanner);
    if string_literal_result.is_err() {
        if let Some(ch) = scanner.peek() {
            if *ch != '"' {
                // In the case where the next sequence failed to match any of the possible
                // function arguments and does not start with a double quote, avoid returning
                // a misleading error message suggesting that a string literal could not be parsed
                // for unsupported keywords, invalid numbers, etc.
                return Err(ParseError::Character(ParseErrorInfo {
                    pos: scanner.pos() + 1,
                    expected: "a valid function argument".to_string(),
                }));
            }
        }
    }
    string_literal_result
}

fn json_path_expr(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();

    let mut path = String::new();

    // Instead of trying to parse a valid JSON Path expression
    // and then repeat the process with the third-party library,
    // we can just consume the input until we reach a closing parenthesis
    // or comma that is not a part of the JSON Path expression.
    let mut in_string_literal = false;
    let mut in_brackets = false;
    while let Some(ch) = scanner.peek() {
        if *ch == '\'' {
            in_string_literal = !in_string_literal;
        } else if *ch == '[' {
            in_brackets = true;
        } else if *ch == ']' {
            in_brackets = false;
        } else if (*ch == ',' || *ch == ')') && !in_string_literal && !in_brackets {
            break;
        }
        path.push(*ch);
        scanner.pop();
    }

    match JsonPath::from_str(path.as_str()) {
        Ok(json_path) => {
            scanner.pop_pos();
            Ok(TemplateFunctionExpr::JsonPath(json_path))
        }
        Err(err) => {
            let err_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::JsonPath(JsonPathParseErrorInfo {
                pos: err_pos + 1,
                expected: "a valid JSON Path expression".to_string(),
                error: err,
            }))
        }
    }
}

fn bool_literal(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();
    let scan_result = scanner.scan(|input| match input {
        "t" => Some(ScannerAction::Require),
        "tr" => Some(ScannerAction::Require),
        "tru" => Some(ScannerAction::Require),
        "true" => Some(ScannerAction::Return(TemplateFunctionExpr::Bool(true))),
        "f" => Some(ScannerAction::Require),
        "fa" => Some(ScannerAction::Require),
        "fal" => Some(ScannerAction::Require),
        "fals" => Some(ScannerAction::Require),
        "false" => Some(ScannerAction::Return(TemplateFunctionExpr::Bool(false))),
        _ => None,
    });
    match scan_result {
        Ok(Some(bool_literal)) => {
            scanner.pop_pos();
            Ok(bool_literal)
        }
        Ok(None) => {
            let error_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::Character(ParseErrorInfo {
                pos: error_pos + 1,
                expected: "a boolean literal".to_string(),
            }))
        }
        Err(err) => Err(err.into()),
    }
}

fn null_literal(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();
    let scan_result = scanner.scan(|input| match input {
        "n" => Some(ScannerAction::Require),
        "nu" => Some(ScannerAction::Require),
        "nul" => Some(ScannerAction::Require),
        "null" => Some(ScannerAction::Return(TemplateFunctionExpr::Null)),
        _ => None,
    });
    match scan_result {
        Ok(Some(null_literal)) => {
            scanner.pop_pos();
            Ok(null_literal)
        }
        Ok(None) => {
            let error_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::Character(ParseErrorInfo {
                pos: error_pos + 1,
                expected: "a null literal".to_string(),
            }))
        }
        Err(err) => Err(err.into()),
    }
}

fn float_literal(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();

    let mut float_seq = String::new();
    let mut passed_decimal_point = false;

    while let Some(ch) = scanner.peek() {
        if *ch == '-' && float_seq.is_empty() {
            float_seq.push(*ch);
            scanner.pop();
        } else if *ch == '.' && !passed_decimal_point {
            float_seq.push(*ch);
            passed_decimal_point = true;
            scanner.pop();
        } else if ch.is_ascii_digit() {
            float_seq.push(*ch);
            scanner.pop();
        } else {
            break;
        }
    }

    if float_seq.is_empty() || !passed_decimal_point {
        let error_pos = scanner.pos();
        scanner.backtrack();
        return Err(ParseError::Character(ParseErrorInfo {
            pos: error_pos + 1,
            expected: "a floating point number".to_string(),
        }));
    }

    match float_seq.parse::<f64>() {
        Ok(float_val) => {
            scanner.pop_pos();
            Ok(TemplateFunctionExpr::Float(float_val))
        }
        Err(_) => {
            let error_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::Character(ParseErrorInfo {
                pos: error_pos + 1,
                expected: "a valid floating point number".to_string(),
            }))
        }
    }
}

fn int_literal(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();

    let mut int_seq = String::new();

    while let Some(ch) = scanner.peek() {
        if (*ch == '-' && int_seq.is_empty()) || ch.is_ascii_digit() {
            int_seq.push(*ch);
            scanner.pop();
        } else {
            break;
        }
    }

    if int_seq.is_empty() {
        let error_pos = scanner.pos();
        scanner.backtrack();
        return Err(ParseError::Character(ParseErrorInfo {
            pos: error_pos + 1,
            expected: "an integer".to_string(),
        }));
    }

    match int_seq.parse::<i64>() {
        Ok(int_val) => {
            scanner.pop_pos();
            Ok(TemplateFunctionExpr::Int(int_val))
        }
        Err(_) => {
            let error_pos = scanner.pos();
            scanner.backtrack();
            Err(ParseError::Character(ParseErrorInfo {
                pos: error_pos + 1,
                expected: "a valid integer".to_string(),
            }))
        }
    }
}

fn string_literal(scanner: &mut Scanner) -> Result<TemplateFunctionExpr, ParseError> {
    scanner.save_pos();

    let mut string = String::new();

    if !scanner.take(&'"') {
        let error_pos = scanner.pos();
        scanner.backtrack();
        return Err(ParseError::Character(ParseErrorInfo {
            pos: error_pos + 1,
            expected: "a string literal".to_string(),
        }));
    }

    let mut prev_char = ' ';
    let mut found_closing_quote = false;
    while let Some(ch) = scanner.peek() {
        if *ch == '"' && prev_char != '\\' {
            found_closing_quote = true;
            scanner.pop();
            break;
        } else {
            string.push(*ch);
            prev_char = *ch;
            scanner.pop();
        }
    }

    if !found_closing_quote {
        scanner.backtrack();
        return Err(ParseError::EndOfInput);
    }

    scanner.pop_pos();
    Ok(TemplateFunctionExpr::Str(string.replace("\\\"", "\"")))
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
    fn test_parse_func_simple() {
        let mut scanner = Scanner::new("format(\"Hello, {}!\", \"world\")");
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "format".to_string(),
                args: vec![
                    TemplateFunctionExpr::Str("Hello, {}!".to_string()),
                    TemplateFunctionExpr::Str("world".to_string())
                ]
            }
        )
    }

    #[test]
    fn test_parse_func_with_json_path() {
        let mut scanner = Scanner::new("map_prefix($.items[?(@.size > 100)], \"large:\")");
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "map_prefix".to_string(),
                args: vec![
                    TemplateFunctionExpr::JsonPath(
                        JsonPath::from_str("$.items[?(@.size > 100)]").unwrap()
                    ),
                    TemplateFunctionExpr::Str("large:".to_string()),
                ]
            }
        )
    }

    #[test]
    fn test_parse_func_with_json_path_2() {
        let mut scanner = Scanner::new("map_prefix($.items[0,1], \"important:\")");
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "map_prefix".to_string(),
                args: vec![
                    TemplateFunctionExpr::JsonPath(JsonPath::from_str("$.items[0,1]").unwrap()),
                    TemplateFunctionExpr::Str("important:".to_string()),
                ]
            }
        )
    }

    #[test]
    fn test_parse_func_mixed_args_1() {
        let mut scanner =
            Scanner::new("format(\"{}, {}, {}, {}, {}\", $.name, 42, true, null, 59.482)");
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "format".to_string(),
                args: vec![
                    TemplateFunctionExpr::Str("{}, {}, {}, {}, {}".to_string()),
                    TemplateFunctionExpr::JsonPath(JsonPath::from_str("$.name").unwrap()),
                    TemplateFunctionExpr::Int(42),
                    TemplateFunctionExpr::Bool(true),
                    TemplateFunctionExpr::Null,
                    TemplateFunctionExpr::Float(59.482),
                ]
            }
        )
    }

    #[test]
    fn test_parse_func_mixed_args_2() {
        let mut scanner = Scanner::new(
            // Additional white space is added to test the scanner can successfully skip it.
            "      list(  false, null, -973.593, \"nested \\\"inside\\\"\",        895048392, $['info'][0]    )",
        );
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "list".to_string(),
                args: vec![
                    TemplateFunctionExpr::Bool(false),
                    TemplateFunctionExpr::Null,
                    TemplateFunctionExpr::Float(-973.593),
                    TemplateFunctionExpr::Str("nested \"inside\"".to_string()),
                    TemplateFunctionExpr::Int(895048392),
                    TemplateFunctionExpr::JsonPath(JsonPath::from_str("$['info'][0]").unwrap()),
                ]
            }
        )
    }

    #[test]
    fn test_parse_nested_func_calls() {
        let mut scanner = Scanner::new(
            "list(format(   \"{}, {}\", \"hello\", \"world\"), format(\"{}, {}\", \"foo\", \"bar\"))",
        );
        let result = parse_func(&mut scanner);
        assert!(result.is_ok());
        let func_call = result.unwrap();
        assert_eq!(
            func_call,
            TemplateFunctionCall {
                name: "list".to_string(),
                args: vec![
                    TemplateFunctionExpr::FuncCall(TemplateFunctionCall {
                        name: "format".to_string(),
                        args: vec![
                            TemplateFunctionExpr::Str("{}, {}".to_string()),
                            TemplateFunctionExpr::Str("hello".to_string()),
                            TemplateFunctionExpr::Str("world".to_string()),
                        ]
                    }),
                    TemplateFunctionExpr::FuncCall(TemplateFunctionCall {
                        name: "format".to_string(),
                        args: vec![
                            TemplateFunctionExpr::Str("{}, {}".to_string()),
                            TemplateFunctionExpr::Str("foo".to_string()),
                            TemplateFunctionExpr::Str("bar".to_string()),
                        ]
                    }),
                ]
            }
        )
    }

    #[test]
    fn test_fails_to_parse_func_call_with_invalid_json_path() {
        let mut scanner = Scanner::new("map_prefix($.items[?(@.size > 100), \"large:\")");
        let result = parse_func(&mut scanner);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(matches!(error, ParseError::JsonPath(_)));
        assert!(error.to_string().starts_with(
            "parse error at position 46: expected a valid JSON Path expression, error: \
            Failed to parse rule: "
        ),)
    }

    #[test]
    fn test_fails_to_parse_func_call_with_invalid_string_literal() {
        let mut scanner = Scanner::new("format(\"Hello, \"world\")");
        let result = parse_func(&mut scanner);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(matches!(error, ParseError::Character(_)));
        // The first string literal is "Hello, " and it is expected to be followed by ","
        // for more arguments or ")" to close the function call.
        // In this case, the next character is "w" which is invalid.
        assert_eq!(
            error.to_string(),
            "parse error at position 17, expected \")\" after the last function argument"
        )
    }

    #[test]
    fn test_fails_to_parse_func_call_with_invalid_number() {
        let mut scanner = Scanner::new("format(\"result: {}\", 42.3.5)");
        let result = parse_func(&mut scanner);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(matches!(error, ParseError::Character(_)));
        // The second argument does not match a float or an integer literal.
        // The parser tries to match a float and fails at the second decimal point,
        // then tries to match an integer, it matches the integer 42 and then expects
        // a valid function argument separator or the closing parenthesis.
        // In this case, the next character is "." which is invalid.
        assert_eq!(
            error.to_string(),
            "parse error at position 26, expected \")\" after the last function argument"
        )
    }

    #[test]
    fn test_fails_to_parse_func_call_with_invalid_keyword() {
        let mut scanner = Scanner::new("format(\"result: {}\", nil)");
        let result = parse_func(&mut scanner);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(matches!(error, ParseError::Character(_)));
        assert_eq!(
            error.to_string(),
            "parse error at position 23, expected a valid function argument"
        )
    }

    #[test]
    fn test_fails_to_parse_func_call_with_invalid_function_name() {
        let mut scanner = Scanner::new("23-format(\"result: {}\", 42)");
        let result = parse_func(&mut scanner);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert!(matches!(error, ParseError::Character(_)));
        assert_eq!(
            error.to_string(),
            "parse error at position 1, expected a valid function name"
        )
    }
}
