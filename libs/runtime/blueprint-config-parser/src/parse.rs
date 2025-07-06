use std::fmt;
use std::fs::read_to_string;
use std::num::ParseFloatError;

use celerity_helpers::env::EnvVars;
use yaml_rust2::YamlLoader;

use crate::blueprint::BlueprintConfig;
use crate::blueprint_with_subs::BlueprintConfigWithSubs;
use crate::parse_substitutions;
use crate::parse_yaml::build_intermediate_blueprint_config_from_yaml;
use crate::resolve_substitutions::{resolve_blueprint_config_substitutions, ResolveError};
use crate::validate_parsed::validate_blueprint_config;

impl BlueprintConfig {
    /// Parses a Runtime-specific Blueprint
    /// configuration from a JSONC string
    /// and resolves variable substitutions with
    /// the provided environment variables.
    pub fn from_jsonc_str(
        jsonc: &str,
        env: Box<dyn EnvVars>,
    ) -> Result<BlueprintConfig, BlueprintParseError> {
        let mut json = String::from(jsonc);
        json_strip_comments::strip(&mut json)?;
        let intermediate_config = serde_json::from_str::<BlueprintConfigWithSubs>(&json)?;
        let final_config = resolve_blueprint_config_substitutions(intermediate_config, env)
            .map_err(BlueprintParseError::ResolveError)?;
        validate_blueprint_config(&final_config)?;
        Ok(final_config)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a JSONC file
    /// and resolves variable substitutions with
    /// the provided environment variables.
    pub fn from_jsonc_file(
        file_path: &str,
        env: Box<dyn EnvVars>,
    ) -> Result<BlueprintConfig, BlueprintParseError> {
        let mut doc_str: String = read_to_string(file_path)?;
        json_strip_comments::strip(&mut doc_str)?;
        let intermediate_config: BlueprintConfigWithSubs =
            serde_json::from_str::<BlueprintConfigWithSubs>(&doc_str)?;
        let final_config = resolve_blueprint_config_substitutions(intermediate_config, env)
            .map_err(BlueprintParseError::ResolveError)?;
        validate_blueprint_config(&final_config)?;
        Ok(final_config)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a YAML string
    /// and resolves variable substitutions with
    /// the provided environment variables.
    pub fn from_yaml_str(
        yaml: &str,
        env: Box<dyn EnvVars>,
    ) -> Result<BlueprintConfig, BlueprintParseError> {
        let docs = YamlLoader::load_from_str(yaml)?;
        let doc = &docs[0];
        let intermediate_config = build_intermediate_blueprint_config_from_yaml(doc)?;
        resolve_blueprint_config_substitutions(intermediate_config, env)
            .map_err(BlueprintParseError::ResolveError)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a YAML file
    /// and resolves variable substitutions with
    /// the provided environment variables.
    pub fn from_yaml_file(
        file_path: &str,
        env: Box<dyn EnvVars>,
    ) -> Result<BlueprintConfig, BlueprintParseError> {
        let doc_str: String = read_to_string(file_path)?;
        let docs = YamlLoader::load_from_str(&doc_str)?;
        let doc = &docs[0];
        let intermediate_config = build_intermediate_blueprint_config_from_yaml(doc)?;
        resolve_blueprint_config_substitutions(intermediate_config, env)
            .map_err(BlueprintParseError::ResolveError)
    }
}

/// Provides an error type for parsing
/// Blueprint configuration.
#[derive(Debug)]
pub enum BlueprintParseError {
    IoError(std::io::Error),
    JsonError(serde_json::Error),
    ValidationError(String),
    ResolveError(ResolveError),
    YamlScanError(yaml_rust2::ScanError),
    YamlFormatError(String),
    UnsupportedResourceType(String),
    UnsupportedWorkflowStateType(String),
    SubstitutionParseError(parse_substitutions::ParseError),
}

impl fmt::Display for BlueprintParseError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            BlueprintParseError::IoError(error) => write!(f, "io error: {}", error),
            BlueprintParseError::JsonError(error) => write!(f, "parsing json failed: {}", error),
            BlueprintParseError::YamlScanError(error) => {
                write!(f, "parsing yaml failed: {}", error)
            }
            BlueprintParseError::YamlFormatError(error) => {
                write!(f, "parsing yaml failed: {}", error)
            }
            BlueprintParseError::ValidationError(error) => write!(f, "validation error: {}", error),
            BlueprintParseError::ResolveError(error) => write!(f, "resolve error: {}", error),
            BlueprintParseError::UnsupportedResourceType(resource_type) => {
                write!(f, "resource type not supported: {}", resource_type)
            }
            BlueprintParseError::UnsupportedWorkflowStateType(state_type) => {
                write!(f, "workflow state type not supported: {}", state_type)
            }
            BlueprintParseError::SubstitutionParseError(error) => {
                write!(f, "substitution parse error: {}", error)
            }
        }
    }
}

impl From<serde_json::Error> for BlueprintParseError {
    fn from(error: serde_json::Error) -> Self {
        BlueprintParseError::JsonError(error)
    }
}

impl From<std::io::Error> for BlueprintParseError {
    fn from(error: std::io::Error) -> Self {
        BlueprintParseError::IoError(error)
    }
}

impl From<yaml_rust2::ScanError> for BlueprintParseError {
    fn from(error: yaml_rust2::ScanError) -> Self {
        BlueprintParseError::YamlScanError(error)
    }
}

impl From<ParseFloatError> for BlueprintParseError {
    fn from(error: ParseFloatError) -> Self {
        BlueprintParseError::YamlFormatError(error.to_string())
    }
}

impl From<ResolveError> for BlueprintParseError {
    fn from(error: ResolveError) -> Self {
        BlueprintParseError::ResolveError(error)
    }
}

impl From<parse_substitutions::ParseError> for BlueprintParseError {
    fn from(error: parse_substitutions::ParseError) -> Self {
        BlueprintParseError::SubstitutionParseError(error)
    }
}
