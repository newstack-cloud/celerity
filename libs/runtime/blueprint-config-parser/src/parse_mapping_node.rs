use std::{collections::HashMap, fmt};

use serde::{
    de::{MapAccess, SeqAccess, Visitor},
    Deserialize, Deserializer,
};

use crate::blueprint::{BlueprintScalarValue, MappingNode};

impl<'de> Deserialize<'de> for MappingNode {
    fn deserialize<D>(deserializer: D) -> Result<MappingNode, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(MappingNodeVisitor)
    }
}

struct MappingNodeVisitor;

impl<'de> Visitor<'de> for MappingNodeVisitor {
    type Value = MappingNode;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("a valid mapping node that can be a mapping, sequence or scalar")
    }

    fn visit_map<V>(self, mut map: V) -> Result<Self::Value, V::Error>
    where
        V: MapAccess<'de>,
    {
        let mut map_value = HashMap::<String, MappingNode>::new();
        while let Some(key) = map.next_key::<String>()? {
            let value = map.next_value()?;
            map_value.insert(key, value);
        }
        Ok(MappingNode::Mapping(map_value))
    }

    fn visit_seq<V>(self, mut seq: V) -> Result<Self::Value, V::Error>
    where
        V: SeqAccess<'de>,
    {
        let mut seq_value = Vec::<MappingNode>::new();
        while let Some(value) = seq.next_element()? {
            seq_value.push(value);
        }
        Ok(MappingNode::Sequence(seq_value))
    }

    fn visit_string<E>(self, value: String) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Str(value)))
    }

    fn visit_bool<E>(self, value: bool) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Bool(value)))
    }

    fn visit_i8<E>(self, value: i8) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Int(value.into())))
    }

    fn visit_i16<E>(self, value: i16) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Int(value.into())))
    }

    fn visit_i32<E>(self, value: i32) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Int(value.into())))
    }

    fn visit_i64<E>(self, value: i64) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Int(value)))
    }

    fn visit_f32<E>(self, value: f32) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Float(
            value.into(),
        )))
    }

    fn visit_f64<E>(self, value: f64) -> Result<Self::Value, E> {
        Ok(MappingNode::Scalar(BlueprintScalarValue::Float(value)))
    }

    fn visit_none<E>(self) -> Result<Self::Value, E> {
        Ok(MappingNode::Null)
    }
}
