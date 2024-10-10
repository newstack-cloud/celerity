use std::{collections::HashMap, fmt};

use celerity_blueprint_config_parser::blueprint::{BlueprintScalarValue, MappingNode};
use serde_json::{json, Map, Number, Value};

use crate::{
    scanner::Scanner,
    template_func_parser::{parse_func, ParseError},
};

/// A trait for a payload template rendering engine
/// that can be used to render the "payload" input object for
/// a workflow state that will in most cases be passed into a handler.
pub trait Engine {
    /// Render the payload template using the provided template
    /// and input data.
    fn render(
        &self,
        template: &HashMap<String, MappingNode>,
        input: &Value,
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
    JSONPathError(String),
    FunctionNotFound(String),
    IncorrectFunctionArgs(String),
    ParseFunctionCallError(String),
}

impl fmt::Display for PayloadTemplateEngineError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            PayloadTemplateEngineError::JSONPathError(path) => {
                write!(
                    f,
                    "payload template engine error: JSON path error: {}",
                    path
                )
            }
            PayloadTemplateEngineError::FunctionNotFound(func) => {
                write!(
                    f,
                    "payload template engine error: function not found: {}",
                    func
                )
            }
            PayloadTemplateEngineError::IncorrectFunctionArgs(err) => {
                write!(
                    f,
                    "payload template engine error: incorrect function arguments: {}",
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

/// A payload template rendering engine that implements
/// payload templates as defined in the v2024-07-22 `celerity/workflow`
/// resource type spec.
pub struct EngineV1 {}

impl EngineV1 {
    /// Create a new instance of a payload template engine
    /// that implements the v2024-07-22 `celerity/workflow` resource type spec.
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

    fn render_func_call(
        &self,
        key: &str,
        func_call: &str,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let mut scanner = Scanner::new(func_call);
        let parsed = parse_func(&mut scanner)?;
        // match func_name {
        //     "format" => self.format(),
        //     _ => Err(PayloadTemplateEngineError::FunctionNotFound(
        //         func_name.to_string(),
        //     )),
        // }
        Ok(json!({}))
    }

    fn render_json_path_query(
        &self,
        key: &str,
        path: &str,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        Ok(json!({}))
    }
}

impl Engine for EngineV1 {
    fn render(
        &self,
        template: &HashMap<String, MappingNode>,
        input: &Value,
    ) -> Result<Value, PayloadTemplateEngineError> {
        let mut rendered = Map::new();
        for (key, node) in template.iter() {
            let rendered_value = match node {
                MappingNode::Scalar(value) => self.render_scalar(key, value, input)?,
                _ => {
                    return Err(PayloadTemplateEngineError::FunctionNotFound(
                        "array functions are not supported".to_string(),
                    ))
                }
            };
            rendered.insert(key.clone(), rendered_value);
        }
        Ok(Value::Object(rendered))
    }
}
