use std::fmt;

use serde_json::Value;

/// The error type used for template function
/// call errors.
#[derive(Debug)]
pub enum FunctionCallError {
    InvalidArgument(String),
    IncorrectNumberOfArguments(String),
    InvalidInput(String),
}

impl fmt::Display for FunctionCallError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            FunctionCallError::InvalidArgument(arg) => {
                write!(f, "function call error: invalid argument: {}", arg)
            }
            FunctionCallError::IncorrectNumberOfArguments(func) => {
                write!(
                    f,
                    "function call error: incorrect number of arguments: {}",
                    func
                )
            }
            FunctionCallError::InvalidInput(err) => {
                write!(f, "function call error: invalid input: {}", err)
            }
        }
    }
}

/// V1 Workflow Template Function `format` implementation.
///
/// This function formats a string using the provided arguments.
/// The use of `{}` in the format string will be replaced by the arguments
/// in the order they are provided.
///
/// See [format function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#format).
pub fn format(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() < 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "format function requires at least one argument".to_string(),
        ));
    }

    let format_string = match &args[0] {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "format function requires the first argument to be a string".to_string(),
            ))
        }
    };

    let placeholder_count = format_string.matches("{}").count();
    if args.len() - 1 != placeholder_count as usize {
        return Err(FunctionCallError::IncorrectNumberOfArguments(format!(
            "format function requires {} arguments after the format string, \
            one for each \"{{}}\" placeholder",
            placeholder_count
        )));
    }

    let mut formatted = format_string.to_string();
    for arg in args.iter().skip(1) {
        match arg {
            Value::String(string) => {
                formatted = formatted.replacen("{}", string, 1);
            }
            Value::Number(number) => {
                formatted = formatted.replacen("{}", &number.to_string(), 1);
            }
            Value::Bool(boolean) => {
                formatted = formatted.replacen("{}", &boolean.to_string(), 1);
            }
            Value::Null => {
                formatted = formatted.replacen("{}", "null", 1);
            }
            Value::Array(_) | Value::Object(_) => {
                return Err(FunctionCallError::InvalidArgument(
                    "format function does not support arrays or objects as arguments".to_string(),
                ));
            }
        }
    }
    Ok(Value::String(formatted))
}

/// V1 Workflow Template Function `jsondecode` implementation.
///
/// This function decodes a JSON string into an object, array or scalar value.
///
/// See [jsondecode function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#jsondecode).
pub fn jsondecode(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "jsondecode function requires a single argument".to_string(),
        ));
    }

    let encoded_string = match &args[0] {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "jsondecode function requires the first argument to be a string".to_string(),
            ))
        }
    };

    match serde_json::from_str(encoded_string) {
        Ok(decoded) => Ok(decoded),
        Err(err) => Err(FunctionCallError::InvalidInput(format!(
            "jsondecode function failed to decode JSON string: {}",
            err
        ))),
    }
}

/// V1 Workflow Template Function `jsonencode` implementation.
///
/// This function encodes a value into a JSON string.
///
/// See [jsonencode function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#jsonencode).
pub fn jsonencode(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "jsonencode function requires a single argument".to_string(),
        ));
    }

    match serde_json::to_string(&args[0]) {
        Ok(encoded) => Ok(Value::String(encoded)),
        Err(err) => Err(FunctionCallError::InvalidInput(format!(
            "jsonencode function failed to encode JSON value: {}",
            err
        ))),
    }
}

#[cfg(test)]
mod format_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_format_simple() {
        let args = vec![json!("This is a simple {}!"), json!("test")];
        let result = format(args).unwrap();
        assert_eq!(result, json!("This is a simple test!"));
    }

    #[test]
    fn test_format_multiple_placeholders() {
        let args = vec![
            json!("{} {} {}"),
            json!("This is a test"),
            json!("with"),
            json!("multiple placeholders!"),
        ];
        let result = format(args).unwrap();
        assert_eq!(result, json!("This is a test with multiple placeholders!"));
    }

    #[test]
    fn test_format_number() {
        let args = vec![json!("This is a number: {}"), json!(42)];
        let result = format(args).unwrap();
        assert_eq!(result, json!("This is a number: 42"));
    }

    #[test]
    fn test_format_boolean() {
        let args = vec![json!("This is a boolean: {}"), json!(true)];
        let result = format(args).unwrap();
        assert_eq!(result, json!("This is a boolean: true"));
    }

    #[test]
    fn test_format_null() {
        let args = vec![json!("This is {}"), json!(Value::Null)];
        let result = format(args).unwrap();
        assert_eq!(result, json!("This is null"));
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!(42)];
        let result = format(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: format function requires the first argument to be a string"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = format(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: format function requires at least one argument"
        );
    }

    #[test]
    fn test_fails_when_format_argument_is_an_array() {
        let args = vec![json!("Format {}"), json!(["This is an array"])];
        let result = format(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: format function does not support arrays or objects as arguments"
        );
    }

    #[test]
    fn test_fails_when_format_argument_is_an_object() {
        let args = vec![json!("Format {}"), json!({"key": "value"})];
        let result = format(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: format function does not support arrays or objects as arguments"
        );
    }

    #[test]
    fn test_fails_when_incorrect_number_of_arguments_follow_format_string() {
        let args = vec![json!("Format {} {}")];
        let result = format(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: format function requires \
            2 arguments after the format string, one for each \"{}\" placeholder"
        );
    }
}

#[cfg(test)]
mod jsondecode_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_decodes_valid_json() {
        let args = vec![json!("{\"id\":\"2aa3a8ae-64ff-4c94-8de9-6c952245da32\"}")];
        let result = jsondecode(args).unwrap();
        assert_eq!(
            result,
            json!({
                "id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"
            })
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!(905)];
        let result = jsondecode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: jsondecode function requires the first argument to be a string"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = jsondecode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: jsondecode function requires a single argument"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_json_input() {
        let args = vec![json!("{\"invalid\": \"json\"")];
        let result = jsondecode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidInput(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid input: jsondecode function failed to decode JSON string: \
            EOF while parsing an object at line 1 column 18"
        );
    }
}

#[cfg(test)]
mod jsonencode_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_encodes_value_as_json_string() {
        let args = vec![json!({"id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"})];
        let result = jsonencode(args).unwrap();
        assert_eq!(
            result,
            json!("{\"id\":\"2aa3a8ae-64ff-4c94-8de9-6c952245da32\"}")
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = jsonencode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: jsonencode function requires a single argument"
        );
    }
}
