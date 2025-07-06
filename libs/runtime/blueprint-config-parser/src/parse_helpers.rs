use std::marker::PhantomData;
use std::{collections::HashMap, fmt};

use serde::{de, Deserialize, Deserializer};

use crate::blueprint::BLUELINK_BLUEPRINT_V2025_05_12;
use crate::blueprint_with_subs::RuntimeBlueprintResourceWithSubs;

/// Deserializes a blueprint version string and makes
/// sure it is a valid version.
/// This is a serde-compatible deserialize function.
pub fn deserialize_version<'de, D>(d: D) -> Result<String, D::Error>
where
    D: Deserializer<'de>,
{
    let version = String::deserialize(d)?;
    if version != BLUELINK_BLUEPRINT_V2025_05_12 {
        return Err(de::Error::invalid_value(
            de::Unexpected::Str(&version),
            &BLUELINK_BLUEPRINT_V2025_05_12,
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

/// Deserializes a blueprint resource map, making sure that resources
/// with unsupported types are skipped without causing blueprint deserialization
/// to completely fail.
pub fn deserialize_resource_map<'de, D>(
    d: D,
) -> Result<HashMap<String, RuntimeBlueprintResourceWithSubs>, D::Error>
where
    D: Deserializer<'de>,
{
    struct ResourceMapVisitor;

    impl<'de> de::Visitor<'de> for ResourceMapVisitor {
        type Value = HashMap<String, RuntimeBlueprintResourceWithSubs>;

        fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
            formatter.write_str("a map of blueprint resources")
        }

        fn visit_map<A>(self, mut map: A) -> Result<Self::Value, A::Error>
        where
            A: de::MapAccess<'de>,
        {
            let mut resources = HashMap::new();
            while let Some(key) = map.next_key::<String>()? {
                match map.next_value() {
                    Ok(Some(value)) => {
                        resources.insert(key, value);
                    }
                    Ok(None) => {}
                    Err(err) => {
                        // Skip unsupported resource types, as there is no reason
                        // for them to cause the entire blueprint deserialization to fail.
                        // Blueprints are expected to have a mix of Celerity-specific resources
                        // and other resources representing infrastructure that the Celerity
                        // runtime doesn't need to know about.
                        if !err.to_string().starts_with("unsupported resource type:") {
                            return Err(err);
                        }
                    }
                }
            }

            Ok(resources)
        }
    }

    d.deserialize_map(ResourceMapVisitor)
}
