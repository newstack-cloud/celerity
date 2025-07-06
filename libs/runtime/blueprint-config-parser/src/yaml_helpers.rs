use std::collections::HashMap;

use crate::{
    blueprint::BlueprintScalarValue,
    blueprint_with_subs::{MappingNode, StringOrSubstitution, StringOrSubstitutions},
    parse::BlueprintParseError,
    parse_substitutions::{parse_substitutions, ParseError},
};

pub fn validate_mapping_node(
    value: &yaml_rust2::Yaml,
    context: &str,
) -> Result<MappingNode, BlueprintParseError> {
    match value {
        yaml_rust2::Yaml::Hash(map) => {
            let mut map_value = HashMap::<String, MappingNode>::new();
            for (key, value) in map.iter() {
                if let yaml_rust2::Yaml::String(key_str) = key {
                    let value = validate_mapping_node(value, context)?;
                    map_value.insert(key_str.to_string(), value);
                }
            }
            Ok(MappingNode::Mapping(map_value))
        }
        yaml_rust2::Yaml::Array(seq) => {
            let mut seq_value = Vec::<MappingNode>::new();
            for value in seq.iter() {
                let value = validate_mapping_node(value, context)?;
                seq_value.push(value);
            }
            Ok(MappingNode::Sequence(seq_value))
        }
        yaml_rust2::Yaml::String(value_str) => {
            Ok(MappingNode::SubstitutionStr(parse_substitutions::<
                ParseError,
            >(value_str)?))
        }
        yaml_rust2::Yaml::Boolean(value_bool) => Ok(MappingNode::Scalar(
            BlueprintScalarValue::Bool(value_bool.clone()),
        )),
        yaml_rust2::Yaml::Integer(value_int) => Ok(MappingNode::Scalar(BlueprintScalarValue::Int(
            value_int.clone(),
        ))),
        yaml_rust2::Yaml::Real(value_float) => Ok(MappingNode::Scalar(
            BlueprintScalarValue::Float(value_float.parse::<f64>()?),
        )),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "Unsupported value type provided for mapping node in {}",
            context,
        ))),
    }
}

pub fn extract_scalar_value(
    value: &yaml_rust2::Yaml,
    field: &str,
) -> Result<Option<BlueprintScalarValue>, BlueprintParseError> {
    match value {
        yaml_rust2::Yaml::Integer(value_int) => Ok(Some(BlueprintScalarValue::Int(*value_int))),
        yaml_rust2::Yaml::Real(value_int) => {
            Ok(Some(BlueprintScalarValue::Float(value_int.parse()?)))
        }
        yaml_rust2::Yaml::Boolean(value_bool) => Ok(Some(BlueprintScalarValue::Bool(*value_bool))),
        yaml_rust2::Yaml::String(value_str) => {
            Ok(Some(BlueprintScalarValue::Str(value_str.clone())))
        }
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a scalar value for {}, found {:?}",
            field, value
        ))),
    }
}

pub fn validate_single_substitution(
    value: &str,
    target_type: &str,
) -> Result<StringOrSubstitutions, BlueprintParseError> {
    let string_with_subs = parse_substitutions::<ParseError>(value)?;
    if string_with_subs.values.len() == 1
        && matches!(
            string_with_subs.values[0],
            StringOrSubstitution::SubstitutionValue(_)
        )
    {
        Ok(string_with_subs)
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "expected a string consisting of a single ${{..}} substitution \
            that can resolve to a value of type {}, found {:?}",
            target_type, value
        )))
    }
}

pub fn validate_array_of_strings(
    values: &Vec<yaml_rust2::Yaml>,
    field: &str,
) -> Result<Vec<StringOrSubstitutions>, BlueprintParseError> {
    let mut strings = Vec::new();
    for value in values {
        if let yaml_rust2::Yaml::String(value_str) = value {
            strings.push(parse_substitutions::<ParseError>(value_str)?);
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for {}, found {:?}",
                field, value
            )))?
        }
    }
    Ok(strings)
}
