use std::{collections::HashMap, fmt};

use axum::extract::path;
use celerity_blueprint_config_parser::blueprint::{BlueprintScalarValue, MappingNode};
use jsonpath_rust::JsonPath;
use serde_json::{json, Map, Number, Value};

use crate::{
    scanner::Scanner,
    template_func_parser::{parse_func, ParseError, TemplateFunctionCall, TemplateFunctionExpr},
    template_functions_v1::{self, FunctionCallError},
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
    ParseFunctionCallError(String),
    FunctionCallFailed(FunctionCallError),
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
                TemplateFunctionExpr::JsonPath(path) => computed_args.push(path.find(input)),
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
