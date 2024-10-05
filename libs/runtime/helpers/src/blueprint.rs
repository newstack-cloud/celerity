use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintLinkSelector, CelerityResourceType, RuntimeBlueprintResource,
};

/// Contains a resource from a blueprint and its name.
#[derive(Debug)]
pub struct ResourceWithName<'a> {
    pub name: String,
    pub resource: &'a RuntimeBlueprintResource,
}

/// Selects resources in a blueprint that are of the specified resource type
/// and have labels that match the provided link selector.
pub fn select_resources<'a>(
    link_selector: &'a Option<BlueprintLinkSelector>,
    blueprint_config: &'a BlueprintConfig,
    target_type: CelerityResourceType,
) -> Vec<ResourceWithName<'a>> {
    let mut target_resources = Vec::new();
    if let Some(link_selector) = link_selector {
        for (key, value) in &link_selector.by_label {
            let matching_resources = blueprint_config
                .resources
                .iter()
                .filter(|(_, resource)| {
                    if let Some(labels) = &resource.metadata.labels {
                        labels
                            .get(key)
                            .map(|search_label_val| search_label_val == value)
                            .is_some()
                            && resource.resource_type == target_type
                    } else {
                        false
                    }
                })
                .map(|(name, resource)| ResourceWithName {
                    name: name.clone(),
                    resource,
                })
                .collect::<Vec<_>>();
            target_resources.extend(matching_resources);
        }
    }
    target_resources
}
