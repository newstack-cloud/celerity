use std::{collections::HashMap, fmt, str::FromStr};

use celerity_blueprint_config_parser::blueprint::{BlueprintScalarValue, ResolvedMappingNode};
use celerity_helpers::{jsonpath::jsonpath_inject_root, scanner::Scanner};
use jsonpath_rust::JsonPath;
use serde_json::{Map, Number, Value};

use crate::{
    template_func_parser::{parse_func, ParseError, TemplateFunctionCall, TemplateFunctionExpr},
    template_functions_v1::{self, FunctionCallError},
};

/// A trait for a payload template rendering engine
/// that can be used to render the "payload" input object for
/// a workflow state that will in most cases be passed into a handler.
/// This is also used to inject values into a provided input value
/// and extract values from a provided input value.
pub trait Engine {
    /// Render the payload template using the provided template
    /// and input data.
    fn render(
        &self,
        template: &HashMap<String, ResolvedMappingNode>,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError>;

    /// Inject a value into the provided input with the given path.
    fn inject(
        &self,
        input: &Value,
        inject_path: &str,
        inject_value: Value,
    ) -> Result<Value, PayloadTemplateEngineError>;

    /// Extracts a value from the provided input using the given path.
    fn extract(
        &self,
        input: &Value,
        extract_path: &str,
    ) -> Result<Value, PayloadTemplateEngineError>;
}

impl fmt::Debug for dyn Engine + Send + Sync {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "Engine")
    }
}

/// The error type used for payload template
/// engine implementations.
#[derive(Debug)]
pub enum PayloadTemplateEngineError {
    JsonPathError(String),
    FunctionNotFound(String),
    ParseFunctionCallError(String),
    FunctionCallFailed(FunctionCallError),
}

impl fmt::Display for PayloadTemplateEngineError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            PayloadTemplateEngineError::JsonPathError(path) => {
                write!(
                    f,
                    "payload template engine error: JSON path error: {}",
                    path
                )
            }
            PayloadTemplateEngineError::FunctionNotFound(func) => {
                write!(
                    f,
                    "payload template engine error: function \"{}\" not found",
                    func
                )
            }
            PayloadTemplateEngineError::FunctionCallFailed(err) => {
                write!(
                    f,
                    "payload template engine error: function call failed: {}",
                    err
                )
            }
            PayloadTemplateEngineError::ParseFunctionCallError(err) => {
                write!(
                    f,
                    "payload template engine error: failed to parse function call: {}",
                    err
                )
            }
        }
    }
}

impl From<ParseError> for PayloadTemplateEngineError {
    fn from(error: ParseError) -> Self {
        PayloadTemplateEngineError::ParseFunctionCallError(error.to_string())
    }
}

impl From<FunctionCallError> for PayloadTemplateEngineError {
    fn from(error: FunctionCallError) -> Self {
        PayloadTemplateEngineError::FunctionCallFailed(error)
    }
}

/// A payload template rendering engine that implements
/// payload templates as defined in the v2026-02-28 `celerity/workflow`
/// resource type spec.
pub struct EngineV1 {}

impl EngineV1 {
    /// Create a new instance of a payload template engine
    /// that implements the v2026-02-28 `celerity/workflow` resource type spec.
    pub fn new() -> Self {
        EngineV1 {}
    }

    fn render_scalar(
        &self,
        key: &str,
        value: &BlueprintScalarValue,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        match value {
            BlueprintScalarValue::Str(string) => {
                if let Some(func_call) = string.strip_prefix("func:") {
                    self.render_func_call(key, func_call, input)
                } else if string.starts_with("$") {
                    self.render_json_path_query(key, string, input)
                } else {
                    Ok(Value::String(string.clone()))
                }
            }
            BlueprintScalarValue::Int(int) => Ok(Value::Number(Number::from(int.clone()))),
            BlueprintScalarValue::Float(float) => Ok(Value::Number(
                Number::from_f64(float.clone())
                    .expect("floating point number in payload template must be valid"),
            )),
            BlueprintScalarValue::Bool(boolean) => Ok(Value::Bool(boolean.clone())),
        }
    }

    fn render_sequence(
        &self,
        key: &str,
        items: &Vec<ResolvedMappingNode>,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let mut rendered = Vec::new();
        for item in items {
            let rendered_item = match item {
                ResolvedMappingNode::Scalar(value) => self.render_scalar(key, value, input)?,
                ResolvedMappingNode::Mapping(mapping) => self.render(mapping, input)?,
                ResolvedMappingNode::Sequence(child_items) => {
                    self.render_sequence(key, child_items, input)?
                }
                ResolvedMappingNode::Null => Value::Null,
            };
            rendered.push(rendered_item);
        }
        Ok(Value::Array(rendered))
    }

    fn render_func_call(
        &self,
        context: &str,
        func_call: &str,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let mut scanner = Scanner::new(func_call);
        let parsed = parse_func(&mut scanner)?;
        self.compute_func_call(context, &parsed, input)
    }

    fn compute_func_call(
        &self,
        context: &str,
        func_call: &TemplateFunctionCall,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let computed_args = self.compute_args(context, &func_call.args, input)?;
        match func_call.name.as_str() {
            "format" => template_functions_v1::format(computed_args).map_err(Into::into),
            "jsondecode" => template_functions_v1::jsondecode(computed_args).map_err(Into::into),
            "jsonencode" => template_functions_v1::jsonencode(computed_args).map_err(Into::into),
            "jsonmerge" => template_functions_v1::jsonmerge(computed_args).map_err(Into::into),
            "b64encode" => template_functions_v1::b64encode(computed_args).map_err(Into::into),
            "b64decode" => template_functions_v1::b64decode(computed_args).map_err(Into::into),
            "hash" => template_functions_v1::hash(computed_args).map_err(Into::into),
            "list" => template_functions_v1::list(computed_args).map_err(Into::into),
            "chunk_list" => template_functions_v1::chunk_list(computed_args).map_err(Into::into),
            "list_elem" => template_functions_v1::list_elem(computed_args).map_err(Into::into),
            "remove_duplicates" => {
                template_functions_v1::remove_duplicates(computed_args).map_err(Into::into)
            }
            "contains" => template_functions_v1::contains(computed_args).map_err(Into::into),
            "split" => template_functions_v1::split(computed_args).map_err(Into::into),
            "math_rand" => template_functions_v1::math_rand(computed_args).map_err(Into::into),
            "math_add" => template_functions_v1::math_add(computed_args).map_err(Into::into),
            "math_sub" => template_functions_v1::math_sub(computed_args).map_err(Into::into),
            "math_mult" => template_functions_v1::math_mult(computed_args).map_err(Into::into),
            "math_div" => template_functions_v1::math_div(computed_args).map_err(Into::into),
            "len" => template_functions_v1::len(computed_args).map_err(Into::into),
            "uuid" => template_functions_v1::uuid(computed_args).map_err(Into::into),
            "nanoid" => template_functions_v1::nanoid(computed_args).map_err(Into::into),
            _ => Err(PayloadTemplateEngineError::FunctionNotFound(
                func_call.name.clone(),
            )),
        }
    }

    fn compute_args(
        &self,
        context: &str,
        args: &Vec<TemplateFunctionExpr>,
        input: &Value,
    ) -> Result<Vec<Value>, PayloadTemplateEngineError> {
        let mut computed_args = Vec::new();
        for arg in args {
            match arg {
                TemplateFunctionExpr::Str(value) => {
                    computed_args.push(Value::String(value.clone()))
                }
                TemplateFunctionExpr::Int(value) => {
                    computed_args.push(Value::Number(Number::from(value.clone())))
                }
                TemplateFunctionExpr::Float(value) => computed_args
                    .push(Value::Number(Number::from_f64(value.clone()).expect(
                        "float parsed by template function parser must be valid",
                    ))),
                TemplateFunctionExpr::Bool(value) => computed_args.push(Value::Bool(value.clone())),
                TemplateFunctionExpr::Null => computed_args.push(Value::Null),
                TemplateFunctionExpr::JsonPath(path) => {
                    computed_args.push(self.extract_json_path_value(path, input))
                }
                TemplateFunctionExpr::FuncCall(func_call) => {
                    let computed = self.compute_func_call(context, &func_call, input)?;
                    computed_args.push(computed);
                }
            }
        }
        Ok(computed_args)
    }

    fn render_json_path_query(
        &self,
        key: &str,
        path: &str,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let path = match JsonPath::from_str(path) {
            Ok(path) => path,
            Err(err) => {
                return Err(PayloadTemplateEngineError::JsonPathError(format!(
                    "invalid json path found for key \"{}\": {}",
                    key, err,
                )))
            }
        };

        Ok(self.extract_json_path_value(&path, input))
    }

    fn extract_json_path_value(&self, path: &JsonPath, input: &Value) -> Value {
        match path.find(input) {
            Value::Null => Value::Null,
            // The jsonpath crate always returns a Value::Array for the query result,
            // even for a single result. We unwrap the single result here.
            wrapped => match wrapped {
                Value::Array(mut array) => {
                    if array.len() == 1 {
                        array.remove(0)
                    } else {
                        // Keep an array where the query result is an array
                        // with multiple elements, suggesting that the query
                        // matched multiple elements in the input.
                        Value::Array(array)
                    }
                }
                value => value,
            },
        }
    }
}

impl Engine for EngineV1 {
    fn render(
        &self,
        template: &HashMap<String, ResolvedMappingNode>,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let mut rendered = Map::new();
        for (key, node) in template.iter() {
            let rendered_value = match node {
                ResolvedMappingNode::Scalar(value) => self.render_scalar(key, value, input)?,
                ResolvedMappingNode::Mapping(mapping) => self.render(mapping, input)?,
                ResolvedMappingNode::Sequence(items) => self.render_sequence(key, items, input)?,
                ResolvedMappingNode::Null => Value::Null,
            };
            rendered.insert(key.to_string(), rendered_value);
        }
        Ok(Value::Object(rendered))
    }

    fn inject(
        &self,
        input: &Value,
        inject_path: &str,
        inject_value: Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let path = match JsonPath::from_str(inject_path) {
            Ok(path) => path,
            Err(err) => {
                return Err(PayloadTemplateEngineError::JsonPathError(format!(
                    "invalid json path found for inject path: {}",
                    err,
                )))
            }
        };

        let mut cloned_input = input.clone();
        let injected = jsonpath_inject_root(&path, &mut cloned_input, inject_value);
        if !injected {
            return Err(PayloadTemplateEngineError::JsonPathError(format!(
                "failed to inject value at path: {}",
                inject_path,
            )));
        }
        Ok(cloned_input)
    }

    fn extract(
        &self,
        input: &Value,
        extract_path: &str,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let path = match JsonPath::from_str(extract_path) {
            Ok(path) => path,
            Err(err) => {
                return Err(PayloadTemplateEngineError::JsonPathError(format!(
                    "invalid json path found for extract path: {}",
                    err,
                )))
            }
        };

        Ok(self.extract_json_path_value(&path, input))
    }
}

#[cfg(test)]
mod engine_v1_render_tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn test_engine_renders_template() {
        let engine = EngineV1::new();
        let template = HashMap::from([
            (
                "value1".to_string(),
                ResolvedMappingNode::Scalar(BlueprintScalarValue::Str("$.values[0]".to_string())),
            ),
            (
                "restOfValues".to_string(),
                ResolvedMappingNode::Scalar(BlueprintScalarValue::Str(
                    "func:remove_duplicates($.values[-5:])".to_string(),
                )),
            ),
            (
                "nestedStructure".to_string(),
                ResolvedMappingNode::Mapping(HashMap::from([
                    (
                        "key1".to_string(),
                        ResolvedMappingNode::Scalar(BlueprintScalarValue::Str("some value".to_string())),
                    ),
                    (
                        "key2".to_string(),
                        ResolvedMappingNode::Scalar(BlueprintScalarValue::Int(20)),
                    ),
                    (
                        "key3".to_string(),
                        ResolvedMappingNode::Scalar(BlueprintScalarValue::Float(4039.402)),
                    ),
                    (
                        "sequence".to_string(),
                        ResolvedMappingNode::Sequence(vec![
                            ResolvedMappingNode::Scalar(BlueprintScalarValue::Str(
                                "$.values[0]".to_string(),
                            )),
                            ResolvedMappingNode::Scalar(BlueprintScalarValue::Str(
                                "$.values[1]".to_string(),
                            )),
                            ResolvedMappingNode::Scalar(BlueprintScalarValue::Str(
                                "func:list(3054, 43.2, remove_duplicates($.values[?(@ > 300)]), true, $['inputStructure'], null, \"string value\")"
                                    .to_string(),
                            )),
                            ResolvedMappingNode::Null,
                        ]),
                    ),
                    (
                        "flag".to_string(),
                        ResolvedMappingNode::Scalar(BlueprintScalarValue::Str("$.flag1".to_string())),
                    ),
                    (
                        "flag2".to_string(),
                        ResolvedMappingNode::Scalar(BlueprintScalarValue::Bool(false)),
                    ),
                ])),
            ),
        ]);
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
            "inputStructure": {
                "id": "1fb11a12-21a5-4404-a12b-b86e06329605"
            },
            "flag1": true,
        });
        let rendered = engine.render(&template, &input).unwrap();
        assert_eq!(
            rendered,
            json!({
                "value1": 10,
                "restOfValues": [405, 304, 20],
                "nestedStructure": {
                    "key1": "some value",
                    "key2": 20,
                    "key3": 4039.402,
                    "sequence": [
                        10,
                        405,
                        [
                            3054,
                            43.2,
                            [405, 304],
                            true,
                            { "id": "1fb11a12-21a5-4404-a12b-b86e06329605" },
                            null,
                            "string value"
                        ],
                        null
                    ],
                    "flag": true,
                    "flag2": false,
                },
            })
        );
    }

    #[test]
    fn test_fails_with_expected_error_due_to_invalid_json_path() {
        let engine = EngineV1::new();
        let template = HashMap::from([(
            "value1".to_string(),
            ResolvedMappingNode::Scalar(BlueprintScalarValue::Str("$.values[0".to_string())),
        )]);
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
        });
        let rendered = engine.render(&template, &input);
        assert!(matches!(
            rendered,
            Err(PayloadTemplateEngineError::JsonPathError(_))
        ));
    }

    #[test]
    fn test_fails_with_expected_error_due_to_missing_function() {
        let engine = EngineV1::new();
        let template = HashMap::from([(
            "value1".to_string(),
            ResolvedMappingNode::Scalar(BlueprintScalarValue::Str(
                "func:unknown_function()".to_string(),
            )),
        )]);
        let input = json!({});
        let rendered = engine.render(&template, &input);
        assert!(matches!(
            rendered,
            Err(PayloadTemplateEngineError::FunctionNotFound(_))
        ));
    }
}

#[cfg(test)]
mod engine_v1_inject_tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn test_engine_injects_value() {
        let engine = EngineV1::new();
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
            "inputStructure": {
                "id": "1fb11a12-21a5-4404-a12b-b86e06329605"
            },
            "flag1": true,
        });
        let inject_path = "$.flag2";
        let inject_value = json!(false);
        let injected = engine.inject(&input, inject_path, inject_value).unwrap();
        assert_eq!(
            injected,
            json!({
                "values": [10, 405, 304, 20, 304, 20],
                "inputStructure": {
                    "id": "1fb11a12-21a5-4404-a12b-b86e06329605"
                },
                "flag1": true,
                "flag2": false,
            })
        );
    }

    #[test]
    fn test_fails_with_expected_error_due_to_invalid_json_path() {
        let engine = EngineV1::new();
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
        });
        let inject_path = "$.values[0";
        let inject_value = json!(100);
        let injected = engine.inject(&input, inject_path, inject_value);
        assert!(matches!(
            injected,
            Err(PayloadTemplateEngineError::JsonPathError(_))
        ));
    }

    #[test]
    fn test_fails_with_expected_error_due_to_unsupported_injection() {
        let engine = EngineV1::new();
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
        });
        // Only fields of the root object can be injected into.
        let inject_path = "$.values[10]";
        let inject_value = json!(100);
        let injected = engine.inject(&input, inject_path, inject_value);
        assert!(matches!(
            injected,
            Err(PayloadTemplateEngineError::JsonPathError(_))
        ));
    }
}

#[cfg(test)]
mod engine_v1_extract_tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn test_engine_extracts_value() {
        let engine = EngineV1::new();
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
            "inputStructure": {
                "id": "1fb11a12-21a5-4404-a12b-b86e06329605"
            },
            "flag1": true,
        });
        let extract_path = "$.inputStructure.id";
        let extracted = engine.extract(&input, extract_path).unwrap();
        assert_eq!(extracted, json!("1fb11a12-21a5-4404-a12b-b86e06329605"));
    }

    #[test]
    fn test_fails_with_expected_error_due_to_invalid_json_path() {
        let engine = EngineV1::new();
        let input = json!({
            "values": [10, 405, 304, 20, 304, 20],
        });
        let extract_path = "$.values[0";
        let extracted = engine.extract(&input, extract_path);
        assert!(matches!(
            extracted,
            Err(PayloadTemplateEngineError::JsonPathError(_))
        ));
    }
}
