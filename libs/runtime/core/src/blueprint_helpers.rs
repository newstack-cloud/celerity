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
                            .unwrap_or(false)
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

/// Finds resources of the specified types whose link selector
/// matches the labels of the target resource.
/// This is the reverse of `select_resources()` — instead of finding
/// resources that a given resource links TO, this finds resources
/// that link TO the given resource.
pub fn find_resources_linking_to<'a>(
    target_resource: &'a RuntimeBlueprintResource,
    blueprint_config: &'a BlueprintConfig,
    source_types: &[CelerityResourceType],
) -> Vec<ResourceWithName<'a>> {
    let target_labels = match &target_resource.metadata.labels {
        Some(labels) => labels,
        None => return Vec::new(),
    };

    blueprint_config
        .resources
        .iter()
        .filter(|(_, resource)| {
            if !source_types.contains(&resource.resource_type) {
                return false;
            }
            let link_selector = match &resource.link_selector {
                Some(ls) => ls,
                None => return false,
            };
            // All label pairs in the link selector must match the target's labels.
            link_selector.by_label.iter().all(|(key, value)| {
                target_labels
                    .get(key)
                    .map(|target_val| target_val == value)
                    .unwrap_or(false)
            })
        })
        .map(|(name, resource)| ResourceWithName {
            name: name.clone(),
            resource,
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use celerity_blueprint_config_parser::blueprint::{
        BlueprintConfig, BlueprintLinkSelector, BlueprintResourceMetadata, CelerityDatastoreSpec,
        CelerityQueueSpec, CelerityResourceSpec, CelerityResourceType, RuntimeBlueprintResource,
    };

    use super::*;

    fn make_resource(
        resource_type: CelerityResourceType,
        labels: Option<HashMap<String, String>>,
        link_selector: Option<BlueprintLinkSelector>,
        spec: CelerityResourceSpec,
    ) -> RuntimeBlueprintResource {
        RuntimeBlueprintResource {
            resource_type,
            metadata: BlueprintResourceMetadata {
                display_name: "".to_string(),
                annotations: None,
                labels,
            },
            link_selector,
            description: None,
            spec,
        }
    }

    fn make_blueprint(resources: Vec<(String, RuntimeBlueprintResource)>) -> BlueprintConfig {
        BlueprintConfig {
            version: "2023-04-20".to_string(),
            transform: None,
            variables: None,
            metadata: None,
            resources: resources.into_iter().collect(),
        }
    }

    #[test]
    fn test_find_resources_linking_to_single_match() {
        let blueprint = make_blueprint(vec![
            (
                "OrdersQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_resource(
                    CelerityResourceType::CelerityConsumer,
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    CelerityResourceSpec::NoSpec,
                ),
            ),
        ]);
        let consumer = blueprint.resources.get("OrdersConsumer").unwrap();
        let results =
            find_resources_linking_to(consumer, &blueprint, &[CelerityResourceType::CelerityQueue]);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].name, "OrdersQueue");
    }

    #[test]
    fn test_find_resources_linking_to_no_match() {
        let blueprint = make_blueprint(vec![
            (
                "PaymentsQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "payments".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_resource(
                    CelerityResourceType::CelerityConsumer,
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    CelerityResourceSpec::NoSpec,
                ),
            ),
        ]);
        let consumer = blueprint.resources.get("OrdersConsumer").unwrap();
        let results =
            find_resources_linking_to(consumer, &blueprint, &[CelerityResourceType::CelerityQueue]);
        assert!(results.is_empty());
    }

    #[test]
    fn test_find_resources_linking_to_filters_by_type() {
        let blueprint = make_blueprint(vec![
            (
                "OrdersQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersDatastore".to_string(),
                make_resource(
                    CelerityResourceType::CelerityDatastore,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Datastore(CelerityDatastoreSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_resource(
                    CelerityResourceType::CelerityConsumer,
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    CelerityResourceSpec::NoSpec,
                ),
            ),
        ]);
        let consumer = blueprint.resources.get("OrdersConsumer").unwrap();
        // Only request queues — datastore should be excluded.
        let results =
            find_resources_linking_to(consumer, &blueprint, &[CelerityResourceType::CelerityQueue]);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].name, "OrdersQueue");
    }

    #[test]
    fn test_find_resources_linking_to_multiple_labels() {
        let blueprint = make_blueprint(vec![
            (
                "StagingQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([
                            ("app".to_string(), "orders".to_string()),
                            ("env".to_string(), "staging".to_string()),
                        ]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "ProdQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([
                            ("app".to_string(), "orders".to_string()),
                            ("env".to_string(), "prod".to_string()),
                        ]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_resource(
                    CelerityResourceType::CelerityConsumer,
                    Some(HashMap::from([
                        ("app".to_string(), "orders".to_string()),
                        ("env".to_string(), "prod".to_string()),
                    ])),
                    None,
                    CelerityResourceSpec::NoSpec,
                ),
            ),
        ]);
        let consumer = blueprint.resources.get("OrdersConsumer").unwrap();
        let results =
            find_resources_linking_to(consumer, &blueprint, &[CelerityResourceType::CelerityQueue]);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].name, "ProdQueue");
    }

    #[test]
    fn test_find_resources_linking_to_no_labels_on_target() {
        let blueprint = make_blueprint(vec![
            (
                "OrdersQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_resource(
                    CelerityResourceType::CelerityConsumer,
                    None, // No labels
                    None,
                    CelerityResourceSpec::NoSpec,
                ),
            ),
        ]);
        let consumer = blueprint.resources.get("OrdersConsumer").unwrap();
        let results =
            find_resources_linking_to(consumer, &blueprint, &[CelerityResourceType::CelerityQueue]);
        assert!(results.is_empty());
    }
}
