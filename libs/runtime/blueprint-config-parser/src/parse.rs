use std::fmt;
use std::fs::read_to_string;
use std::marker::PhantomData;
use std::num::ParseFloatError;
use std::{fs::File, io::BufReader};

use serde::{de, Deserialize, Deserializer};
use yaml_rust2::YamlLoader;

use crate::blueprint::{BlueprintConfig, CELERITY_BLUEPRINT_V2023_04_20};
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

/// Deserializes a blueprint version string and makes
/// sure it is a valid version.
/// This is a serde-compatible deserialize function.
pub fn deserialize_version<'de, D>(d: D) -> Result<String, D::Error>
where
    D: Deserializer<'de>,
{
    let version = String::deserialize(d)?;
    if version != CELERITY_BLUEPRINT_V2023_04_20 {
        return Err(de::Error::invalid_value(
            de::Unexpected::Str(&version),
            &CELERITY_BLUEPRINT_V2023_04_20,
        ));
    }
    Ok(version.to_string())
}

/// Deserializes a string or an array of strings.
/// This is required for blueprint config fields such as transform
/// which can be a string or an array of strings in its serialized form.
pub fn deserialize_optional_string_or_vec<'de, D>(d: D) -> Result<Option<Vec<String>>, D::Error>
where
    D: Deserializer<'de>,
{
    struct StringOrVec(PhantomData<Vec<String>>);

    impl<'de> de::Visitor<'de> for StringOrVec {
        type Value = Vec<String>;

        fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
            formatter.write_str("a string or an array of strings")
        }

        fn visit_str<E>(self, value: &str) -> Result<Self::Value, E>
        where
            E: de::Error,
        {
            Ok(vec![value.to_string()])
        }

        fn visit_seq<S>(self, visitor: S) -> Result<Self::Value, S::Error>
        where
            S: de::SeqAccess<'de>,
        {
            Deserialize::deserialize(de::value::SeqAccessDeserializer::new(visitor))
        }
    }

    d.deserialize_any(StringOrVec(PhantomData)).map(Some)
}

/// Provides an error type for parsing
/// Blueprint configuration.
#[derive(Debug)]
pub enum BlueprintParseError {
    IoError(std::io::Error),
    JsonError(serde_json::Error),
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
