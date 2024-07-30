use std::fmt;
use std::fs::read_to_string;
use std::num::ParseFloatError;
use std::{fs::File, io::BufReader};

use yaml_rust2::YamlLoader;

use crate::blueprint::BlueprintConfig;
use crate::parse_yaml::build_blueprint_config_from_yaml;

impl BlueprintConfig {
    /// Parses a Runtime-specific Blueprint
    /// configuration from a JSON string.
    pub fn from_json_str(json: &str) -> Result<BlueprintConfig, BlueprintParseError> {
        serde_json::from_str(json).map_err(BlueprintParseError::JsonError)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a JSON file.
    pub fn from_json_file(file_path: &str) -> Result<BlueprintConfig, BlueprintParseError> {
        let file = File::open(file_path)?;
        let reader = BufReader::new(file);
        let blueprint: BlueprintConfig = serde_json::from_reader(reader)?;
        validate_blueprint_config(&blueprint)?;
        Ok(blueprint)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a YAML string.
    pub fn from_yaml_str(yaml: &str) -> Result<BlueprintConfig, BlueprintParseError> {
        let docs = YamlLoader::load_from_str(yaml)?;
        let doc = &docs[0];
        build_blueprint_config_from_yaml(doc)
    }

    /// Parses a Runtime-specific Blueprint
    /// configuration from a YAML file.
    pub fn from_yaml_file(file_path: &str) -> Result<BlueprintConfig, BlueprintParseError> {
        let doc_str: String = read_to_string(file_path)?;
        let docs = YamlLoader::load_from_str(&doc_str)?;
        let doc = &docs[0];
        build_blueprint_config_from_yaml(doc)
    }
}

fn validate_blueprint_config(blueprint: &BlueprintConfig) -> Result<(), BlueprintParseError> {
    if blueprint.resources.is_empty() {
        return Err(BlueprintParseError::ValidationError(
            "at least one resource must be provided for a blueprint".to_string(),
        ));
    }

    Ok(())
}

/// Provides an error type for parsing
/// Blueprint configuration.
#[derive(Debug)]
pub enum BlueprintParseError {
    IoError(std::io::Error),
    JsonError(serde_json::Error),
    ValidationError(String),
    YamlScanError(yaml_rust2::ScanError),
    YamlFormatError(String),
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
