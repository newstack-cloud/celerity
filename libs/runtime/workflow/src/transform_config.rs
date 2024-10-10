use std::collections::HashMap;

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, CelerityResourceType, CelerityWorkflowSpec,
};
use celerity_helpers::blueprint::select_resources;

use crate::{config::WorkflowAppConfig, errors::ConfigError};

pub fn collect_workflow_app_config(
    blueprint_config: BlueprintConfig,
) -> Result<WorkflowAppConfig, ConfigError> {
    // Find the first Workflow resource in the blueprint.
    let (_, workflow) = blueprint_config
        .resources
        .iter()
        .find(|(_, resource)| resource.resource_type == CelerityResourceType::CelerityWorkflow)
        .ok_or_else(|| ConfigError::WorkflowMissing)?;

    let target_handlers = select_resources(
        &workflow.link_selector,
        &blueprint_config,
        CelerityResourceType::CelerityHandler,
    );

    Ok(WorkflowAppConfig {
        state_handlers: None,
        workflow: CelerityWorkflowSpec {
            start_at: "".to_string(),
            states: HashMap::new(),
        },
    })
}
