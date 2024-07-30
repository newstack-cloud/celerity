use std::fmt;
use std::marker::PhantomData;

use serde::{de, Deserialize, Deserializer};

use crate::blueprint::CELERITY_BLUEPRINT_V2023_04_20;

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
