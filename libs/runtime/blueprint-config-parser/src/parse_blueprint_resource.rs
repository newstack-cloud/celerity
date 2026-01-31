use core::fmt;

use serde::{
    de::{self, MapAccess, Visitor},
    Deserialize, Deserializer,
};

use crate::{
    blueprint::CelerityResourceType,
    blueprint_with_subs::{
        BlueprintResourceMetadataWithSubs, CelerityResourceSpecWithSubs,
        RuntimeBlueprintResourceWithSubs,
    },
};

#[derive(Deserialize)]
#[serde(field_identifier, rename_all = "camelCase")]
enum ResourceField {
    #[serde(rename = "type")]
    ResourceType,
    Metadata,
    Spec,
    Description,
    LinkSelector,
}

impl<'de> Deserialize<'de> for RuntimeBlueprintResourceWithSubs {
    fn deserialize<D>(deserializer: D) -> Result<RuntimeBlueprintResourceWithSubs, D::Error>
    where
        D: Deserializer<'de>,
    {
        const FIELDS: &[&str] = &["type", "metadata", "spec", "description", "linkSelector"];
        deserializer.deserialize_struct(
            "RuntimeBlueprintResourceWithSubs",
            FIELDS,
            ResourceWithSubsVisitor,
        )
    }
}

struct ResourceWithSubsVisitor;

impl<'de> ResourceWithSubsVisitor {
    fn spec_from_resource_type_map<V>(
        &self,
        resource_type: &CelerityResourceType,
        map: &mut V,
    ) -> Result<CelerityResourceSpecWithSubs, V::Error>
    where
        V: MapAccess<'de>,
    {
        match resource_type {
            CelerityResourceType::CelerityApi => {
                let api_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Api(api_spec))
            }
            CelerityResourceType::CelerityConsumer => {
                let consumer_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Consumer(consumer_spec))
            }
            CelerityResourceType::CeleritySchedule => {
                let schedule_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Schedule(schedule_spec))
            }
            CelerityResourceType::CelerityHandler => {
                let handler_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Handler(handler_spec))
            }
            CelerityResourceType::CelerityHandlerConfig => {
                let handler_config_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::HandlerConfig(
                    handler_config_spec,
                ))
            }
            CelerityResourceType::CelerityWorkflow => {
                let workflow_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Workflow(workflow_spec))
            }
            CelerityResourceType::CelerityConfig => {
                let config_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Config(config_spec))
            }
            CelerityResourceType::CelerityBucket => {
                let bucket_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Bucket(bucket_spec))
            }
            CelerityResourceType::CelerityTopic => {
                let topic_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Topic(topic_spec))
            }
            CelerityResourceType::CelerityQueue => {
                let queue_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Queue(queue_spec))
            }
            CelerityResourceType::CelerityVpc => {
                let vpc_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Vpc(vpc_spec))
            }
            CelerityResourceType::CelerityDatastore => {
                let datastore_spec = map.next_value()?;
                Ok(CelerityResourceSpecWithSubs::Datastore(datastore_spec))
            }
        }
    }
}

impl<'de> Visitor<'de> for ResourceWithSubsVisitor {
    type Value = RuntimeBlueprintResourceWithSubs;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("struct RuntimeBlueprintResourceWithSubs")
    }

    fn visit_map<V>(self, mut map: V) -> Result<Self::Value, V::Error>
    where
        V: MapAccess<'de>,
    {
        let mut resource_type = None;
        let mut metadata = BlueprintResourceMetadataWithSubs::default();
        let mut spec = CelerityResourceSpecWithSubs::NoSpec;
        let mut description = None;
        let mut link_selector = None;
        let mut unsupported_resource_type_err: Option<String> = None;
        while let Some(key) = map.next_key()? {
            if unsupported_resource_type_err.is_some() {
                // Skip the rest of the fields for this resource if the resource type is unsupported.
                map.next_value::<serde::de::IgnoredAny>()?;
            } else {
                match key {
                    ResourceField::ResourceType => {
                        if resource_type.is_some() {
                            return Err(de::Error::duplicate_field("type"));
                        }
                        match map.next_value::<CelerityResourceType>() {
                            Ok(value) => {
                                resource_type = Some(value);
                            }
                            Err(err) => {
                                // Ideally we would match on a specific error field
                                // or enum type here instead of a string value
                                // that can change.
                                // The serde docs are not clear on the public members
                                // of an Error type and pushes teh responsibility of
                                // error discrimination to a Serializer/Deserializer
                                // implementation.
                                if err.to_string().starts_with("unknown variant") {
                                    // Capture an error prefixed with "unsupported resource type:"
                                    // in this case so the parent deserializer can differentiate
                                    // between an unknown resource type variant and other errors.
                                    // This ultimately allows skipping over resources with
                                    // resource types that are not recognised by the runtime.
                                    unsupported_resource_type_err =
                                        Some(format!("unsupported resource type: {err}"));
                                } else {
                                    // serde produces a generic "expected value" error,
                                    // so we need to provide a more specific error message
                                    // to provide a better user experience.
                                    return Err(de::Error::custom(
                                        "invalid data type provided for resource type",
                                    ));
                                }
                            }
                        }
                    }
                    ResourceField::Metadata => {
                        metadata = map.next_value()?;
                    }
                    ResourceField::Spec => {
                        if spec != CelerityResourceSpecWithSubs::NoSpec {
                            return Err(de::Error::duplicate_field("spec"));
                        }
                        if let Some(unwrapped_resource_type) = resource_type.clone() {
                            spec = self
                                .spec_from_resource_type_map(&unwrapped_resource_type, &mut map)?;
                        } else {
                            return Err(de::Error::custom(
                                "spec must come after type in resource, type is either defined after spec or is missing"
                            ));
                        }
                    }
                    ResourceField::Description => {
                        if description.is_some() {
                            return Err(de::Error::duplicate_field("description"));
                        }
                        description = Some(map.next_value()?);
                    }
                    ResourceField::LinkSelector => {
                        if link_selector.is_some() {
                            return Err(de::Error::duplicate_field("linkSelector"));
                        }
                        link_selector = Some(map.next_value()?);
                    }
                }
            }
        }

        if let Some(unsupported_resource_type_err) = unsupported_resource_type_err {
            return Err(de::Error::custom(unsupported_resource_type_err));
        }

        Ok(RuntimeBlueprintResourceWithSubs {
            resource_type: resource_type.ok_or_else(|| de::Error::missing_field("type"))?,
            metadata,
            spec,
            description,
            link_selector,
        })
    }
}
