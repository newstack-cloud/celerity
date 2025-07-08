use crate::{
    blueprint::{BlueprintConfig, BlueprintVariable},
    parse::BlueprintParseError,
};

/// Validates a blueprint configuration that has been parsed,
/// primarily used for serde deserialization for JSON,
/// YAML parsing is validated during parsing due to the usage
/// of yaml_rust2 in light of serde_yaml being an archived project.
pub fn validate_blueprint_config(blueprint: &BlueprintConfig) -> Result<(), BlueprintParseError> {
    if blueprint.resources.is_empty() {
        return Err(BlueprintParseError::ValidationError(
            "at least one resource must be provided for a blueprint".to_string(),
        ));
    }

    if let Some(variables_map) = &blueprint.variables {
        for (var_name, var_definition) in variables_map {
            validate_blueprint_var(var_name.as_str(), var_definition)?;
        }
    }

    Ok(())
}

fn validate_blueprint_var(
    var_name: &str,
    var_definition: &BlueprintVariable,
) -> Result<(), BlueprintParseError> {
    if var_definition.var_type.is_empty() {
        return Err(BlueprintParseError::ValidationError(format!(
            "type must be provided in \\\"{var_name}\\\" variable definition",
        )));
    }
    Ok(())
}
