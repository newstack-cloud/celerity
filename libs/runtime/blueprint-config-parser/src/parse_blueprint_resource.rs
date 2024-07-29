use core::fmt;

use serde::{
    de::{self, MapAccess, SeqAccess, Visitor},
    Deserialize, Deserializer,
};

use crate::blueprint::{
    BlueprintResourceMetadata, CelerityResourceSpec, CelerityResourceType, RuntimeBlueprintResource,
};

#[derive(Deserialize)]
#[serde(field_identifier, rename_all = "camelCase")]
enum Field {
    #[serde(rename = "type")]
    ResourceType,
    Metadata,
    Spec,
    Description,
    LinkSelector,
}

impl<'de> Deserialize<'de> for RuntimeBlueprintResource {
    fn deserialize<D>(deserializer: D) -> Result<RuntimeBlueprintResource, D::Error>
    where
        D: Deserializer<'de>,
    {
        const FIELDS: &[&str] = &["type", "metadata", "spec", "description", "linkSelector"];
        deserializer.deserialize_struct("RuntimeBlueprintResource", FIELDS, ResourceVisitor)
    }
}

struct ResourceVisitor;

impl<'de> ResourceVisitor {
    fn spec_from_resource_type_seq<V>(
        &self,
        resource_type: &CelerityResourceType,
        seq: &mut V,
    ) -> Result<CelerityResourceSpec, V::Error>
    where
        V: SeqAccess<'de>,
    {
        match resource_type {
            CelerityResourceType::CelerityApi => {
                let api_spec = seq
                    .next_element()?
                    .ok_or_else(|| de::Error::invalid_length(2, self))?;
                Ok(CelerityResourceSpec::Api(api_spec))
            }
            CelerityResourceType::CelerityConsumer => {
                let consumer_spec = seq
                    .next_element()?
                    .ok_or_else(|| de::Error::invalid_length(2, self))?;
                Ok(CelerityResourceSpec::Consumer(consumer_spec))
            }
            CelerityResourceType::CeleritySchedule => {
                let schedule_spec = seq
                    .next_element()?
                    .ok_or_else(|| de::Error::invalid_length(2, self))?;
                Ok(CelerityResourceSpec::Schedule(schedule_spec))
            }
            CelerityResourceType::CelerityHandler => {
                let handler_spec = seq
                    .next_element()?
                    .ok_or_else(|| de::Error::invalid_length(2, self))?;
                Ok(CelerityResourceSpec::Handler(handler_spec))
            }
        }
    }

    fn spec_from_resource_type_map<V>(
        &self,
        resource_type: &CelerityResourceType,
        map: &mut V,
    ) -> Result<CelerityResourceSpec, V::Error>
    where
        V: MapAccess<'de>,
    {
        match resource_type {
            CelerityResourceType::CelerityApi => {
                let api_spec = map.next_value()?;
                Ok(CelerityResourceSpec::Api(api_spec))
            }
            CelerityResourceType::CelerityConsumer => {
                let consumer_spec = map.next_value()?;
                Ok(CelerityResourceSpec::Consumer(consumer_spec))
            }
            CelerityResourceType::CeleritySchedule => {
                let schedule_spec = map.next_value()?;
                Ok(CelerityResourceSpec::Schedule(schedule_spec))
            }
            CelerityResourceType::CelerityHandler => {
                let handler_spec = map.next_value()?;
                Ok(CelerityResourceSpec::Handler(handler_spec))
            }
        }
    }
}

impl<'de> Visitor<'de> for ResourceVisitor {
    type Value = RuntimeBlueprintResource;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("struct RuntimeBlueprintResource")
    }

    fn visit_seq<V>(self, mut seq: V) -> Result<RuntimeBlueprintResource, V::Error>
    where
        V: SeqAccess<'de>,
    {
        let resource_type = seq
            .next_element()?
            .ok_or_else(|| de::Error::invalid_length(0, &self))?;
        let metadata = seq
            .next_element()?
            .ok_or_else(|| de::Error::invalid_length(1, &self))?;
        let spec = self.spec_from_resource_type_seq(&resource_type, &mut seq)?;
        let description = seq
            .next_element()?
            .ok_or_else(|| de::Error::invalid_length(3, &self))?;
        let link_selector = seq
            .next_element()?
            .ok_or_else(|| de::Error::invalid_length(4, &self))?;

        Ok(RuntimeBlueprintResource {
            resource_type,
            metadata,
            spec,
            description,
            link_selector,
        })
    }

    fn visit_map<V>(self, mut map: V) -> Result<RuntimeBlueprintResource, V::Error>
    where
        V: MapAccess<'de>,
    {
        let mut resource_type = None;
        let mut metadata = BlueprintResourceMetadata::default();
        let mut spec = CelerityResourceSpec::NoSpec;
        let mut description = None;
        let mut link_selector = None;
        while let Some(key) = map.next_key()? {
            match key {
                Field::ResourceType => {
                    if resource_type.is_some() {
                        return Err(de::Error::duplicate_field("type"));
                    }
                    resource_type = Some(map.next_value()?);
                }
                Field::Metadata => {
                    metadata = map.next_value()?;
                }
                Field::Spec => {
                    if spec != CelerityResourceSpec::NoSpec {
                        return Err(de::Error::duplicate_field("spec"));
                    }
                    if let Some(ref unwrapped_resource_type) = resource_type {
                        spec =
                            self.spec_from_resource_type_map(unwrapped_resource_type, &mut map)?;
                    } else {
                        return Err(de::Error::custom("spec must come after type in resource"));
                    }
                }
                Field::Description => {
                    if description.is_some() {
                        return Err(de::Error::duplicate_field("description"));
                    }
                    description = Some(map.next_value()?);
                }
                Field::LinkSelector => {
                    if link_selector.is_some() {
                        return Err(de::Error::duplicate_field("linkSelector"));
                    }
                    link_selector = Some(map.next_value()?);
                }
            }
        }
        Ok(RuntimeBlueprintResource {
            resource_type: resource_type.ok_or_else(|| de::Error::missing_field("type"))?,
            metadata,
            spec,
            description,
            link_selector,
        })
    }
}
