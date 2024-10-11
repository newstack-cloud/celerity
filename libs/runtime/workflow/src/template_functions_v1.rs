use std::fmt;

use base64::{prelude::BASE64_STANDARD, Engine};
use rand::Rng;
use serde_json::Value;
use sha2::{Digest, Sha256, Sha384, Sha512};

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

/// V1 Workflow Template Function `jsonmerge` implementation.
///
/// This function merges two JSON objects into a single JSON object.
///
/// See [jsonmerge function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#jsonmerge).
pub fn jsonmerge(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "jsonmerge function requires two arguments".to_string(),
        ));
    }

    let (first, second) = (&args[0], &args[1]);
    match (first, second) {
        (Value::Object(first_obj), Value::Object(second_obj)) => {
            let mut merged = first_obj.clone();
            merged.extend(second_obj.clone());
            Ok(Value::Object(merged))
        }
        _ => Err(FunctionCallError::InvalidArgument(
            "jsonmerge function requires two JSON objects as arguments".to_string(),
        )),
    }
}

/// V1 Workflow Template Function `b64encode` implementation.
///
/// This function base64 encodes a string.
///
/// See [b64encode function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#b64encode).
pub fn b64encode(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "b64encode function requires a single argument".to_string(),
        ));
    }

    let input = match &args[0] {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "b64encode function requires the first argument to be a string".to_string(),
            ))
        }
    };

    Ok(Value::String(BASE64_STANDARD.encode(input.as_bytes())))
}

/// V1 Workflow Template Function `b64decode` implementation.
///
/// This function base64 decodes a string.
///
/// See [b64decode function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#b64decode).
pub fn b64decode(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "b64decode function requires a single argument".to_string(),
        ));
    }

    let input = match &args[0] {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "b64decode function requires the first argument to be a string".to_string(),
            ))
        }
    };

    match BASE64_STANDARD.decode(input.as_bytes()) {
        Ok(decoded) => Ok(Value::String(String::from_utf8_lossy(&decoded).to_string())),
        Err(err) => Err(FunctionCallError::InvalidInput(format!(
            "b64decode function failed to decode base64 string: {}",
            err
        ))),
    }
}

/// V1 Workflow Template Function `hash` implementation.
///
/// This function hashes some input data using a specified algorithm.
/// This returns the hash as a hex string.
///
/// The available algorithms are:
/// - `SHA256`
/// - `SHA384`
/// - `SHA512`
///
/// MD5 and SHA1 were considered in the original design but were not included
/// due to the insecurity of these algorithms.
///
/// See [hash function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#hash).
pub fn hash(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "hash function requires two arguments".to_string(),
        ));
    }

    let (algorithm, input) = (&args[0], &args[1]);
    let algorithm = match algorithm {
        Value::String(algo) => algo,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "hash function requires the first argument to be a string".to_string(),
            ))
        }
    };

    let input = match input {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "hash function requires the second argument to be a string".to_string(),
            ))
        }
    };

    let hash =
        match algorithm.as_str() {
            "SHA256" => {
                let mut hasher = Sha256::new();
                hasher.update(input.as_bytes());
                hex::encode(hasher.finalize())
            }
            "SHA384" => {
                let mut hasher = Sha384::new();
                hasher.update(input.as_bytes());
                hex::encode(hasher.finalize())
            }
            "SHA512" => {
                let mut hasher = Sha512::new();
                hasher.update(input.as_bytes());
                hex::encode(hasher.finalize())
            }
            _ => return Err(FunctionCallError::InvalidArgument(
                "hash function requires the first argument to be one of: SHA256, SHA384, SHA512"
                    .to_string(),
            )),
        };

    Ok(Value::String(hash))
}

/// V1 Workflow Template Function `list` implementation.
///
/// This function creates a list from a set of positional arguments.
///
/// See [list function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#list).
pub fn list(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    Ok(Value::Array(args))
}

/// V1 Workflow Template Function `chunk_list` implementation.
///
/// This function splits a list into chunks of a specified size.
///
/// See [chunk list function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#chunk_list).
pub fn chunk_list(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "chunk_list function requires two arguments".to_string(),
        ));
    }

    let (list, chunk_size) = (&args[0], &args[1]);
    let list = match list {
        Value::Array(list) => list,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "chunk_list function requires the first argument to be a list".to_string(),
            ))
        }
    };

    let chunk_size = match chunk_size {
        Value::Number(size) => size.as_u64().expect("chunk size must be a valid integer"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "chunk_list function requires the second argument to be a number".to_string(),
            ))
        }
    };

    let mut chunks = Vec::new();
    for chunk in list.chunks(chunk_size as usize) {
        chunks.push(Value::Array(chunk.to_vec()));
    }

    Ok(Value::Array(chunks))
}

/// V1 Workflow Template Function `list_elem` implementation.
///
/// This function returns an element from a list at a specific index.
///
/// See [list element function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#list_elem).
pub fn list_elem(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "list_elem function requires two arguments".to_string(),
        ));
    }

    let (list, index) = (&args[0], &args[1]);
    let list = match list {
        Value::Array(list) => list,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "list_elem function requires the first argument to be a list".to_string(),
            ))
        }
    };

    let index = match index {
        Value::Number(size) => size.as_u64().expect("index must be a valid integer"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "list_elem function requires the second argument to be a number".to_string(),
            ))
        }
    };

    match list.get(index as usize) {
        Some(elem) => Ok(elem.clone()),
        // As null (Value::Null) can be a valid value in a list, to avoid confusing an expected null value
        // with an index being out of bounds, an error is returned.
        None => Err(FunctionCallError::InvalidInput(
            "list_elem function failed to get element at index: index out of bounds".to_string(),
        )),
    }
}

/// V1 Workflow Template Function `remove_duplicates` implementation.
///
/// This function removes duplicates from an array of values.
/// The functoin will carry out deep equality checks for objects and arrays,
/// performance may be significantly implacted when working with large and complex structures.
///
/// See [remove duplicates function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#remove_duplicates).
pub fn remove_duplicates(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 1 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "remove_duplicates function requires a single argument".to_string(),
        ));
    }

    let list = match &args[0] {
        Value::Array(list) => list,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "remove_duplicates function requires the first argument to be a list".to_string(),
            ))
        }
    };

    let mut unique = Vec::new();
    for elem in list {
        if !unique.contains(elem) {
            unique.push(elem.clone());
        }
    }

    Ok(Value::Array(unique))
}

/// V1 Workflow Template Function `contains` implementation.
///
/// This function checks if a value is present in a list or a substring is present in a string.
/// This will carry out deep equality checks for objects and arrays.
///
/// See [contains function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#contains).
pub fn contains(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "contains function requires two arguments".to_string(),
        ));
    }

    let (list, value) = (&args[0], &args[1]);
    match list {
        Value::Array(list) => {
            for elem in list {
                if elem == value {
                    return Ok(Value::Bool(true));
                }
            }
            Ok(Value::Bool(false))
        }
        Value::String(haystack) => match value {
            Value::String(needle) => Ok(Value::Bool(haystack.contains(needle))),
            _ => Err(FunctionCallError::InvalidArgument(
                "contains function requires the second argument to be \
                a string when the first argument is a string"
                    .to_string(),
            )),
        },
        _ => Err(FunctionCallError::InvalidArgument(
            "contains function requires the first argument to be a list or a string".to_string(),
        )),
    }
}

/// V1 Workflow Template Function `split` implementation.
///
/// This function splits a string into an array of substrings based on a delimiter.
///
/// See [split function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#split).
pub fn split(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "split function requires two arguments".to_string(),
        ));
    }

    let (string, delimiter) = (&args[0], &args[1]);
    let string = match string {
        Value::String(string) => string,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "split function requires the first argument to be a string".to_string(),
            ))
        }
    };

    let delimiter = match delimiter {
        Value::String(delimiter) => delimiter,
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "split function requires the second argument to be a string".to_string(),
            ))
        }
    };

    Ok(Value::Array(
        string
            .split(delimiter)
            .map(|s| Value::String(s.to_string()))
            .collect(),
    ))
}

/// V1 Workflow Template Function `math_rand` implementation.
///
/// This function generates a random number between a minimum and maximum value.
/// The random number generated is an integer and the provided parameters must be integers.
///
/// See [math rand function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#math_rand).
pub fn math_rand(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "math_rand function requires two arguments".to_string(),
        ));
    }

    let (min, max) = (&args[0], &args[1]);
    let min = match min {
        Value::Number(min) => min.as_i64().expect("min value must be a valid integer"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_rand function requires the first argument to be a number".to_string(),
            ))
        }
    };

    let max = match max {
        Value::Number(max) => max.as_i64().expect("max value must be a valid integer"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_rand function requires the second argument to be a number".to_string(),
            ))
        }
    };

    if min >= max {
        return Err(FunctionCallError::InvalidArgument(
            "math_rand function requires the min to be less than the max".to_string(),
        ));
    }

    let random = rand::thread_rng().gen_range(min..max);
    Ok(Value::Number(random.into()))
}

/// V1 Workflow Template Function `math_add` implementation.
///
/// This function will add two numbers together.
///
/// See [math add function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#math_add).
pub fn math_add(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "math_add function requires two arguments".to_string(),
        ));
    }

    let (first, second) = (&args[0], &args[1]);
    let first = match first {
        Value::Number(first) => first.as_f64().expect("first value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_add function requires the first argument to be a number".to_string(),
            ))
        }
    };

    let second = match second {
        Value::Number(second) => second
            .as_f64()
            .expect("second value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_add function requires the second argument to be a number".to_string(),
            ))
        }
    };

    Ok(Value::Number(
        serde_json::Number::from_f64(first + second)
            .expect("result of math add function must be a valid number"),
    ))
}

/// V1 Workflow Template Function `math_sub` implementation.
///
/// This function will subtract the second number from the first.
///
/// See [math subtract function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#math_sub).
pub fn math_sub(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "math_sub function requires two arguments".to_string(),
        ));
    }

    let (first, second) = (&args[0], &args[1]);
    let first = match first {
        Value::Number(first) => first.as_f64().expect("first value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_sub function requires the first argument to be a number".to_string(),
            ))
        }
    };

    let second = match second {
        Value::Number(second) => second
            .as_f64()
            .expect("second value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_sub function requires the second argument to be a number".to_string(),
            ))
        }
    };

    Ok(Value::Number(
        serde_json::Number::from_f64(first - second)
            .expect("result of math sub function must be a valid number"),
    ))
}

/// V1 Workflow Template Function `math_mult` implementation.
///
/// This function will multiply two numbers together.
///
/// See [math mult function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#math_mult).
pub fn math_mult(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "math_mult function requires two arguments".to_string(),
        ));
    }

    let (first, second) = (&args[0], &args[1]);
    let first = match first {
        Value::Number(first) => first.as_f64().expect("first value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_mult function requires the first argument to be a number".to_string(),
            ))
        }
    };

    let second = match second {
        Value::Number(second) => second
            .as_f64()
            .expect("second value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_mult function requires the second argument to be a number".to_string(),
            ))
        }
    };

    Ok(Value::Number(
        serde_json::Number::from_f64(first * second)
            .expect("result of math mult function must be a valid number"),
    ))
}

/// V1 Workflow Template Function `math_div` implementation.
///
/// This function will divide a number by another.
///
/// See [math div function definition](https://celerityframework.com/docs/applications/resources/celerity-workflow#math_div).
pub fn math_div(args: Vec<Value>) -> Result<Value, FunctionCallError> {
    if args.len() != 2 {
        return Err(FunctionCallError::IncorrectNumberOfArguments(
            "math_div function requires two arguments".to_string(),
        ));
    }

    let (first, second) = (&args[0], &args[1]);
    let first = match first {
        Value::Number(first) => first.as_f64().expect("first value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_div function requires the first argument to be a number".to_string(),
            ))
        }
    };

    let second = match second {
        Value::Number(second) => second
            .as_f64()
            .expect("second value must be a valid number"),
        _ => {
            return Err(FunctionCallError::InvalidArgument(
                "math_div function requires the second argument to be a number".to_string(),
            ))
        }
    };

    if second == 0.0 {
        return Err(FunctionCallError::InvalidInput(
            "math_div function requires the second argument to be a non-zero number".to_string(),
        ));
    }

    Ok(Value::Number(
        serde_json::Number::from_f64(first / second)
            .expect("result of math div function must be a valid number"),
    ))
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

#[cfg(test)]
mod jsonmerge_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_merges_2_json_objects() {
        let args = vec![
            json!({"id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"}),
            json!({"name": "John Doe"}),
        ];
        let result = jsonmerge(args).unwrap();
        assert_eq!(
            result,
            json!({
                "id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32",
                "name": "John Doe"
            })
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!({"id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"})];
        let result = jsonmerge(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: jsonmerge function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!(42), json!(true)];
        let result = jsonmerge(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: jsonmerge function requires two JSON objects as arguments"
        );
    }
}

#[cfg(test)]
mod b64encode_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_encodes_string_to_base64() {
        let args = vec![json!("This is a test string")];
        let result = b64encode(args).unwrap();
        assert_eq!(result, json!("VGhpcyBpcyBhIHRlc3Qgc3RyaW5n"));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = b64encode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: b64encode function requires a single argument"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!(6094.20)];
        let result = b64encode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: b64encode function requires the first argument to be a string"
        );
    }
}

#[cfg(test)]
mod b64decode_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_decodes_base64_string() {
        let args = vec![json!("VGhpcyBpcyBhbm90aGVyIHRlc3Qgc3RyaW5n")];
        let result = b64decode(args).unwrap();
        assert_eq!(result, json!("This is another test string"));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = b64decode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: b64decode function requires a single argument"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!({"id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"})];
        let result = b64decode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: b64decode function requires the first argument to be a string"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_base64_input() {
        let args = vec![json!("VGhpcyBpcyBh$$@!bm$$90aGVyIHRlc3Qgc3RyaW5n")];
        let result = b64decode(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidInput(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid input: b64decode function failed to decode base64 string: \
            Invalid symbol 36, offset 12."
        );
    }
}

#[cfg(test)]
mod hash_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_hashes_input_with_sha256() {
        let args = vec![json!("SHA256"), json!("This is a test string")];
        let result = hash(args).unwrap();
        assert_eq!(
            result,
            json!("717ac506950da0ccb6404cdd5e7591f72018a20cbca27c8a423e9c9e5626ac61")
        );
    }

    #[test]
    fn test_hashes_input_with_sha384() {
        let args = vec![json!("SHA384"), json!("This is a test string")];
        let result = hash(args).unwrap();
        assert_eq!(
            result,
            json!(
                "9bd1f75eb75c8ffad8f4b4c67c8f14db32cc3d4177b942334abd4\
                7f9e02e35b371d599cb4796185d7410e808f046e119"
            )
        );
    }

    #[test]
    fn test_hashes_input_with_sha512() {
        let args = vec![json!("SHA512"), json!("This is a test string")];
        let result = hash(args).unwrap();
        assert_eq!(
            result,
            json!(
                "b8ee69b29956b0b56e26d0a25c6a80713c858cf2902a12962aad\
                08d682345646b2d5f193bbe03997543a9285e5932f34baf2c85c89459f25ba1cf43c4410793c"
            )
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!("SHA256")];
        let result = hash(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: hash function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!("SHA1"), json!("This is a test string")];
        let result = hash(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: hash function requires the first argument to be one of: SHA256, SHA384, SHA512"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_input() {
        let args = vec![json!("SHA256"), json!(42)];
        let result = hash(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: hash function requires the second argument to be a string"
        );
    }
}

#[cfg(test)]
mod list_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_creates_list_from_arguments() {
        let args = vec![json!("This is a test"), json!(42), json!(true)];
        let result = list(args).unwrap();
        assert_eq!(result, json!(["This is a test", 42, true]));
    }
}

#[cfg(test)]
mod chunk_list_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_chunks_list_into_specified_size() {
        let args = vec![json!([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]), json!(3)];
        let result = chunk_list(args).unwrap();
        assert_eq!(result, json!([[1, 2, 3], [4, 5, 6], [7, 8, 9], [10]]));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])];
        let result = chunk_list(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: chunk_list function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!(42), json!(3)];
        let result = chunk_list(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: chunk_list function requires the first argument to be a list"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]), json!("invalid")];
        let result = chunk_list(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: chunk_list function requires the second argument to be a number"
        );
    }
}

#[cfg(test)]
mod list_elem_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_gets_element_at_index() {
        let args = vec![json!([1, 2, 3, 4, 5]), json!(2)];
        let result = list_elem(args).unwrap();
        assert_eq!(result, json!(3));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!([1, 2, 3, 4, 5])];
        let result = list_elem(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: list_elem function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!(9483), json!(2)];
        let result = list_elem(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: list_elem function requires the first argument to be a list"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!([1, 2, 3, 4, 5]), json!("notvalid")];
        let result = list_elem(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: list_elem function requires the second argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_index_out_of_bounds() {
        let args = vec![json!([1, 2, 3, 4, 5]), json!(5)];
        let result = list_elem(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidInput(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid input: list_elem function failed to get element at index: index out of bounds"
        );
    }
}

#[cfg(test)]
mod remove_duplicates_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_removes_duplicates_from_list() {
        let args = vec![
            json!([1, 2, {"id": "1"}, 4, 2, {"id": "1"}, 1, 6, [57, 93], 8, 9, [57, 93], 3, 10]),
        ];
        let result = remove_duplicates(args).unwrap();
        assert_eq!(
            result,
            json!([1, 2, {"id": "1"}, 4, 6, [57, 93], 8, 9, 3, 10])
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![];
        let result = remove_duplicates(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: remove_duplicates function requires a single argument"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_argument() {
        let args = vec![json!({"id": "2aa3a8ae-64ff-4c94-8de9-6c952245da32"})];
        let result = remove_duplicates(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: remove_duplicates function requires the first argument to be a list"
        );
    }
}

#[cfg(test)]
mod contains_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_checks_if_value_is_in_list() {
        let args = vec![json!([1, 2, 3, 4, 5]), json!(3)];
        let result = contains(args).unwrap();
        assert_eq!(result, json!(true));
    }

    #[test]
    fn test_checks_if_needle_is_in_haystack() {
        let args = vec![json!("This is a test string"), json!("test")];
        let result = contains(args).unwrap();
        assert_eq!(result, json!(true));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!([1, 2, 3, 4, 5])];
        let result = contains(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: contains function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!(1204), json!(3)];
        let result = contains(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: contains function requires the first argument to be a list or a string"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument_for_substring_search() {
        let args = vec![json!("This is a test string"), json!(true)];
        let result = contains(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: contains function requires the second argument to be a string when the first argument is a string"
        );
    }
}

#[cfg(test)]
mod split_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_splits_string_into_array_of_substrings() {
        let args = vec![json!("This is a test string"), json!(" ")];
        let result = split(args).unwrap();
        assert_eq!(result, json!(["This", "is", "a", "test", "string"]));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!("This is a test string")];
        let result = split(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: split function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!(49), json!(" ")];
        let result = split(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: split function requires the first argument to be a string"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!("This is a test string"), json!(952)];
        let result = split(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: split function requires the second argument to be a string"
        );
    }
}

#[cfg(test)]
mod math_rand_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_generates_random_number_between_min_and_max() {
        let args = vec![json!(0), json!(100)];
        let result = math_rand(args).unwrap();
        assert!(result.is_number());
        let random = result.as_i64().unwrap();
        assert!(random >= 0 && random < 100);
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!(0)];
        let result = math_rand(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: math_rand function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!("0"), json!(100)];
        let result = math_rand(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_rand function requires the first argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!(0), json!("100")];
        let result = math_rand(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_rand function requires the second argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_when_min_is_greater_than_max() {
        let args = vec![json!(230), json!(50)];
        let result = math_rand(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_rand function requires the min to be less than the max"
        );
    }
}

#[cfg(test)]
mod math_add_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_adds_two_numbers() {
        let args = vec![json!(42), json!(58.931)];
        let result = math_add(args).unwrap();
        assert_eq!(result, json!(100.931));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!(42)];
        let result = math_add(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: math_add function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!("invalid value"), json!(58.931)];
        let result = math_add(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_add function requires the first argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!(42), json!("invalid second value")];
        let result = math_add(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_add function requires the second argument to be a number"
        );
    }
}

#[cfg(test)]
mod math_sub_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_subtracts_second_number_from_first() {
        let args = vec![json!(100), json!(58.931)];
        let result = math_sub(args).unwrap();
        assert_eq!(result, json!(41.069));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!(12)];
        let result = math_sub(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: math_sub function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!("invalid first value"), json!(58.931)];
        let result = math_sub(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_sub function requires the first argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!(100), json!("invalid second value")];
        let result = math_sub(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_sub function requires the second argument to be a number"
        );
    }
}

#[cfg(test)]
mod math_mult_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_multiplies_two_numbers() {
        let args = vec![json!(10), json!(58.931)];
        let result = math_mult(args).unwrap();
        assert_eq!(result, json!(589.31));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!(2012)];
        let result = math_mult(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: math_mult function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!("invalid value"), json!(58.931)];
        let result = math_mult(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_mult function requires the first argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!(11), json!("invalid second value")];
        let result = math_mult(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_mult function requires the second argument to be a number"
        );
    }
}

#[cfg(test)]
mod math_div_tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn test_divides_first_number_by_second() {
        let args = vec![json!(100), json!(10.5)];
        let result = math_div(args).unwrap();
        assert_eq!(result, json!(9.523809523809524));
    }

    #[test]
    fn test_fails_with_expected_error_for_incorrect_number_of_arguments() {
        let args = vec![json!(1312)];
        let result = math_div(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(
            err,
            FunctionCallError::IncorrectNumberOfArguments(_)
        ));
        assert_eq!(
            err.to_string(),
            "function call error: incorrect number of arguments: math_div function requires two arguments"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_first_argument() {
        let args = vec![json!("invalid value"), json!(10.5)];
        let result = math_div(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_div function requires the first argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_for_invalid_second_argument() {
        let args = vec![json!(100), json!("invalid second value")];
        let result = math_div(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidArgument(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid argument: math_div function requires the second argument to be a number"
        );
    }

    #[test]
    fn test_fails_with_expected_error_when_dividing_by_zero() {
        let args = vec![json!(100), json!(0)];
        let result = math_div(args);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(matches!(err, FunctionCallError::InvalidInput(_)));
        assert_eq!(
            err.to_string(),
            "function call error: invalid input: math_div function requires the second argument to be a non-zero number"
        );
    }
}
