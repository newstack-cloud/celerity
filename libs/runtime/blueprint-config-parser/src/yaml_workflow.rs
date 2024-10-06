use std::collections::HashMap;

use crate::{
    blueprint::{
        BlueprintScalarValue, CelerityWorkflowCatchConfig, CelerityWorkflowCondition,
        CelerityWorkflowDecisionRule, CelerityWorkflowFailureConfig,
        CelerityWorkflowParallelBranch, CelerityWorkflowRetryConfig, CelerityWorkflowSpec,
        CelerityWorkflowState, CelerityWorkflowStateType, CelerityWorkflowWaitConfig, MappingNode,
    },
    parse::BlueprintParseError,
    workflow_consts::{
        CELERITY_WORKFLOW_STATE_TYPE_DECISION, CELERITY_WORKFLOW_STATE_TYPE_EXECUTE_STEP,
        CELERITY_WORKFLOW_STATE_TYPE_FAILURE, CELERITY_WORKFLOW_STATE_TYPE_PARALLEL,
        CELERITY_WORKFLOW_STATE_TYPE_PASS, CELERITY_WORKFLOW_STATE_TYPE_SUCCESS,
        CELERITY_WORKFLOW_STATE_TYPE_WAIT,
    },
};

// Validates the Celerity workflow spec from the parsed YAML value map.
// This will only validate the structure and not the semantics of the spec
// for a workflow resource.
// The workflow crate will validate the semantics of a parsed workflow spec
// during workflow application startup.
pub fn validate_celerity_workflow_spec(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityWorkflowSpec, BlueprintParseError> {
    let mut workflow_spec = CelerityWorkflowSpec::default();
    for (key, value) in value_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "startAt" => {
                    if let yaml_rust2::Yaml::String(start_at_str) = value {
                        workflow_spec.start_at = start_at_str.to_string();
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Start at state must be a string".to_string(),
                        ));
                    }
                }
                "states" => {
                    if let yaml_rust2::Yaml::Hash(states_map) = value {
                        workflow_spec.states = validate_celerity_workflow_states(states_map)?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "States must be a map".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for workflow spec: {}",
                        key_str
                    )))
                }
            }
        }
    }
    Ok(workflow_spec)
}

fn validate_celerity_workflow_states(
    states_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, CelerityWorkflowState>, BlueprintParseError> {
    let mut states = HashMap::<String, CelerityWorkflowState>::new();
    for (state_name, state_value) in states_map.iter() {
        if let yaml_rust2::Yaml::String(state_name_str) = state_name {
            let state_name = state_name_str.to_string();
            let state = validate_celerity_workflow_state(state_name.as_str(), state_value)?;
            states.insert(state_name, state);
        }
    }
    Ok(states)
}

fn validate_celerity_workflow_state(
    state_name: &str,
    state_value: &yaml_rust2::Yaml,
) -> Result<CelerityWorkflowState, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(state_map) = state_value {
        let mut state = CelerityWorkflowState::default();
        for (key, value) in state_map.iter() {
            if let yaml_rust2::Yaml::String(key_str) = key {
                match key_str.as_str() {
                    "type" => {
                        state.state_type = validate_state_type_field(value, state_name)?;
                    }
                    "description" => {
                        state.description = validate_state_description_field(value, state_name)?;
                    }
                    "inputPath" => {
                        state.input_path = validate_state_input_path_field(value, state_name)?;
                    }
                    "resultPath" => {
                        state.result_path = validate_state_result_path_field(value, state_name)?;
                    }
                    "outputPath" => {
                        state.output_path = validate_state_output_path_field(value, state_name)?;
                    }
                    "payloadTemplate" => {
                        state.payload_template =
                            validate_state_payload_template_field(value, state_name)?;
                    }
                    "next" => {
                        state.next = validate_state_next_field(value, state_name)?;
                    }
                    "end" => {
                        state.end = validate_state_end_field(value, state_name)?;
                    }
                    "decisions" => {
                        state.decisions = validate_state_decisions_field(value, state_name)?;
                    }
                    "result" => {
                        state.result = Some(validate_mapping_node(value, state_name)?);
                    }
                    "timeout" => {
                        state.timeout = validate_state_timeout_field(value, state_name)?;
                    }
                    "waitConfig" => {
                        state.wait_config = validate_state_wait_config_field(value, state_name)?;
                    }
                    "failureConfig" => {
                        state.failure_config =
                            validate_state_failure_config_field(value, state_name)?;
                    }
                    "parallelBranches" => {
                        state.parallel_branches =
                            validate_state_parallel_branches_field(value, state_name)?;
                    }
                    "retry" => {
                        state.retry = validate_state_retry_field(value, state_name)?;
                    }
                    "catch" => {
                        state.catch = validate_state_catch_field(value, state_name)?;
                    }
                    _ => {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Unsupported key provided in workflow state \"{}\": {}",
                            state_name, key_str
                        )))
                    }
                }
            }
        }

        if state.state_type == CelerityWorkflowStateType::Unknown {
            return Err(BlueprintParseError::YamlFormatError(format!(
                "State type not provided for state \"{}\"",
                state_name
            )));
        }

        Ok(state)
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "State must be a map".to_string(),
        ))
    }
}

fn validate_state_type_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<CelerityWorkflowStateType, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(type_str) = value {
        validate_state_type(type_str.as_str())
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "State type provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_description_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<String>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(description_str) = value {
        Ok(Some(description_str.to_string()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Description provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_input_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<String>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(input_path_str) = value {
        Ok(Some(input_path_str.to_string()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Input path provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_result_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<String>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(result_path_str) = value {
        Ok(Some(result_path_str.to_string()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Result path provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_output_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<String>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(output_path_str) = value {
        Ok(Some(output_path_str.to_string()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Output path provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_payload_template_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<HashMap<String, MappingNode>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(payload_template_map) = value {
        Ok(Some(validate_payload_template(
            payload_template_map,
            state_name,
        )?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Payload template provided for state \"{}\" must be a map",
            state_name
        )))
    }
}

fn validate_state_next_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<String>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(next_str) = value {
        Ok(Some(next_str.to_string()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Next state provided for state \"{}\" must be a string",
            state_name
        )))
    }
}

fn validate_state_end_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<bool>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Boolean(end_bool) = value {
        Ok(Some(end_bool.clone()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "End value provided for state \"{}\" must be a boolean",
            state_name
        )))
    }
}

fn validate_state_decisions_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowDecisionRule>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(decision_rules_array) = value {
        Ok(Some(validate_celerity_workflow_decision_rules(
            decision_rules_array,
        )?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Decisions provided for state \"{}\" must be an array",
            state_name
        )))
    }
}

fn validate_state_timeout_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<i64>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Integer(timeout_int) = value {
        Ok(Some(timeout_int.clone()))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Timeout value provided for state \"{}\" must be an integer",
            state_name
        )))
    }
}

fn validate_state_wait_config_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<CelerityWorkflowWaitConfig>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(wait_config_map) = value {
        let mut wait_config = CelerityWorkflowWaitConfig::default();
        for (key, value) in wait_config_map.iter() {
            if let yaml_rust2::Yaml::String(key_str) = key {
                match key_str.as_str() {
                    "seconds" => {
                        if let yaml_rust2::Yaml::String(seconds) = value {
                            wait_config.seconds = Some(seconds.clone());
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "Seconds value provided for wait config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    "timestamp" => {
                        if let yaml_rust2::Yaml::String(timestamp) = value {
                            wait_config.timestamp = Some(timestamp.clone());
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "Timestamp value provided for wait config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    _ => (),
                }
            }
        }
        Ok(Some(wait_config))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Wait config provided for state \"{}\" must be a map",
            state_name
        )))
    }
}

fn validate_state_failure_config_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<CelerityWorkflowFailureConfig>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(failure_config_map) = value {
        let mut failure_config = CelerityWorkflowFailureConfig::default();
        for (key, value) in failure_config_map.iter() {
            if let yaml_rust2::Yaml::String(key_str) = key {
                match key_str.as_str() {
                    "error" => {
                        if let yaml_rust2::Yaml::String(error) = value {
                            failure_config.error = Some(error.clone());
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "error value provided for failure config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    "cause" => {
                        if let yaml_rust2::Yaml::String(cause) = value {
                            failure_config.cause = Some(cause.clone());
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "cause value provided for failure config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    _ => (),
                }
            }
        }
        Ok(Some(failure_config))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Failure config provided for state \"{}\" must be a map",
            state_name
        )))
    }
}

fn validate_state_parallel_branches_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowParallelBranch>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(parallel_branches_array) = value {
        let mut parallel_branches = Vec::<CelerityWorkflowParallelBranch>::new();
        for branch_value in parallel_branches_array.iter() {
            if let yaml_rust2::Yaml::Hash(branch_map) = branch_value {
                let branch = validate_workflow_state_parallel_branch(branch_map, state_name)?;
                parallel_branches.push(branch);
            }
        }
        Ok(Some(parallel_branches))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Parallel branches provided for state \"{}\" must be an array",
            state_name
        )))
    }
}

fn validate_workflow_state_parallel_branch(
    branch_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowParallelBranch, BlueprintParseError> {
    let mut parallel_branch = CelerityWorkflowParallelBranch::default();
    for (key, value) in branch_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "startAt" => {
                    if let yaml_rust2::Yaml::String(start_at_str) = value {
                        parallel_branch.start_at = start_at_str.to_string();
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Start at state value provided for parallel branch must be a string in state \"{}\"",
                            state_name,
                        )));
                    }
                }
                "states" => {
                    if let yaml_rust2::Yaml::Hash(states_map) = value {
                        parallel_branch.states = validate_celerity_workflow_states(states_map)?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "States provided for parallel branch in state \"{}\" must be a map",
                            state_name,
                        )));
                    }
                }
                _ => (),
            }
        }
    }
    Ok(parallel_branch)
}

fn validate_state_retry_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowRetryConfig>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(retry_config_array) = value {
        let mut retry_config_list = Vec::<CelerityWorkflowRetryConfig>::new();
        for retry_config_value in retry_config_array.iter() {
            if let yaml_rust2::Yaml::Hash(retry_config_map) = retry_config_value {
                let retry_config = validate_retry_config(retry_config_map, state_name)?;
                retry_config_list.push(retry_config);
            }
        }
        Ok(Some(retry_config_list))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Retry field for state \"{}\" must be an array",
            state_name
        )))
    }
}

fn validate_retry_config(
    retry_config_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowRetryConfig, BlueprintParseError> {
    let mut retry_config = CelerityWorkflowRetryConfig::default();
    for (key, value) in retry_config_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "matchErrors" => {
                    if let yaml_rust2::Yaml::Array(match_errors_yaml_array) = value {
                        let mut match_errors = Vec::<String>::new();
                        for match_error in match_errors_yaml_array.iter() {
                            if let yaml_rust2::Yaml::String(match_error_str) = match_error {
                                match_errors.push(match_error_str.to_string());
                            }
                        }
                        retry_config.match_errors = match_errors;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Match errors field for retry config in state \"{}\" must be an array",
                            state_name,
                        )));
                    }
                }
                "interval" => {
                    if let yaml_rust2::Yaml::Integer(interval_seconds_int) = value {
                        retry_config.interval = Some(interval_seconds_int.clone());
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Interval field for retry config in state \"{}\" must be an integer",
                            state_name,
                        )));
                    }
                }
                "maxAttempts" => {
                    if let yaml_rust2::Yaml::Integer(max_attempts_int) = value {
                        retry_config.max_attempts = Some(max_attempts_int.clone());
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Max attempts field for retry config in state \"{}\" must be an integer",
                            state_name,
                        )));
                    }
                }
                "maxDelay" => {
                    if let yaml_rust2::Yaml::Integer(max_delay_seconds_int) = value {
                        retry_config.max_delay = Some(max_delay_seconds_int.clone());
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Max delay field for retry config in state \"{}\" must be an integer",
                            state_name,
                        )));
                    }
                }
                "jitter" => {
                    if let yaml_rust2::Yaml::Boolean(jitter_bool) = value {
                        retry_config.jitter = Some(jitter_bool.clone());
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Jitter field for retry config in state \"{}\" must be a boolean",
                            state_name,
                        )));
                    }
                }
                "backoffRate" => {
                    if let yaml_rust2::Yaml::Real(backoff_rate_float) = value {
                        retry_config.backoff_rate = Some(backoff_rate_float.parse::<f64>()?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Backoff rate field for retry config in state \"{}\" must be a float",
                            state_name,
                        )));
                    }
                }
                _ => (),
            }
        }
    }
    Ok(retry_config)
}

fn validate_state_catch_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowCatchConfig>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(catch_config_array) = value {
        let mut catch_config_list = Vec::<CelerityWorkflowCatchConfig>::new();
        for catch_config_value in catch_config_array.iter() {
            if let yaml_rust2::Yaml::Hash(catch_config_map) = catch_config_value {
                let catch_config = validate_catch_config(catch_config_map, state_name)?;
                catch_config_list.push(catch_config);
            }
        }
        Ok(Some(catch_config_list))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Catch field for state \"{}\" must be an array",
            state_name
        )))
    }
}

fn validate_catch_config(
    catch_config_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowCatchConfig, BlueprintParseError> {
    let mut catch_config = CelerityWorkflowCatchConfig::default();
    for (key, value) in catch_config_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "matchErrors" => {
                    if let yaml_rust2::Yaml::Array(match_errors_yaml_array) = value {
                        let mut match_errors = Vec::<String>::new();
                        for match_error in match_errors_yaml_array.iter() {
                            if let yaml_rust2::Yaml::String(match_error_str) = match_error {
                                match_errors.push(match_error_str.to_string());
                            }
                        }
                        catch_config.match_errors = match_errors;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Match errors field for catch config in state \"{}\" must be an array",
                            state_name,
                        )));
                    }
                }
                "next" => {
                    if let yaml_rust2::Yaml::String(next_str) = value {
                        catch_config.next = next_str.clone();
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Next field for catch config in state \"{}\" must be a string",
                            state_name,
                        )));
                    }
                }
                "resultPath" => {
                    if let yaml_rust2::Yaml::String(result_path_str) = value {
                        catch_config.result_path = Some(result_path_str.clone());
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Result path field for catch config in state \"{}\" must be a string",
                            state_name,
                        )));
                    }
                }
                _ => (),
            }
        }
    }
    Ok(catch_config)
}

fn validate_state_type(state_type: &str) -> Result<CelerityWorkflowStateType, BlueprintParseError> {
    match state_type {
        CELERITY_WORKFLOW_STATE_TYPE_EXECUTE_STEP => Ok(CelerityWorkflowStateType::ExecuteStep),
        CELERITY_WORKFLOW_STATE_TYPE_PASS => Ok(CelerityWorkflowStateType::Pass),
        CELERITY_WORKFLOW_STATE_TYPE_PARALLEL => Ok(CelerityWorkflowStateType::Parallel),
        CELERITY_WORKFLOW_STATE_TYPE_WAIT => Ok(CelerityWorkflowStateType::Decision),
        CELERITY_WORKFLOW_STATE_TYPE_DECISION => Ok(CelerityWorkflowStateType::Decision),
        CELERITY_WORKFLOW_STATE_TYPE_FAILURE => Ok(CelerityWorkflowStateType::Failure),
        CELERITY_WORKFLOW_STATE_TYPE_SUCCESS => Ok(CelerityWorkflowStateType::Success),
        _ => Err(BlueprintParseError::UnsupportedWorkflowStateType(
            state_type.to_string(),
        )),
    }
}

fn validate_payload_template(
    payload_template_map: &yaml_rust2::yaml::Hash,
    state: &str,
) -> Result<HashMap<String, MappingNode>, BlueprintParseError> {
    let mut payload_template = HashMap::<String, MappingNode>::new();
    let context = format!("payload template for state \"{}\"", state);
    let context_ref = context.as_str();
    for (key, value) in payload_template_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            let key = key_str.to_string();
            let value = validate_mapping_node(value, context_ref)?;
            payload_template.insert(key, value);
        }
    }
    Ok(payload_template)
}

fn validate_celerity_workflow_decision_rules(
    decision_rules_array: &yaml_rust2::yaml::Array,
) -> Result<Vec<CelerityWorkflowDecisionRule>, BlueprintParseError> {
    let mut decision_rules = Vec::<CelerityWorkflowDecisionRule>::new();
    for decision_rule_value in decision_rules_array.iter() {
        if let yaml_rust2::Yaml::Hash(decision_rule_map) = decision_rule_value {
            let decision_rule = validate_decision_rule(decision_rule_map)?;
            decision_rules.push(decision_rule);
        }
    }
    Ok(decision_rules)
}

fn validate_decision_rule(
    decision_rule_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityWorkflowDecisionRule, BlueprintParseError> {
    let mut decision_rule = CelerityWorkflowDecisionRule::default();
    for (key, value) in decision_rule_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "and" => {
                    if let yaml_rust2::Yaml::Array(and_conditions_array) = value {
                        let and_conditions = validate_conditions(and_conditions_array)?;
                        decision_rule.and = Some(and_conditions);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "decision rule \"and\" conditions must be an array".to_string(),
                        ));
                    }
                }
                "or" => {
                    if let yaml_rust2::Yaml::Array(or_conditions_array) = value {
                        let or_conditions = validate_conditions(or_conditions_array)?;
                        decision_rule.or = Some(or_conditions);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "decision rule \"or\" conditions must be an array".to_string(),
                        ));
                    }
                }
                "not" => {
                    if let yaml_rust2::Yaml::Hash(not_condition_map) = value {
                        let not_condition = validate_condition(not_condition_map)?;
                        decision_rule.not = Some(not_condition);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "decision rule \"not\" condition must be a map".to_string(),
                        ));
                    }
                }
                "condition" => {
                    if let yaml_rust2::Yaml::Hash(condition_map) = value {
                        let condition = validate_condition(condition_map)?;
                        decision_rule.condition = Some(condition);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "decision rule \"condition\" must be a map".to_string(),
                        ));
                    }
                }
                "next" => {
                    if let yaml_rust2::Yaml::String(next_str) = value {
                        decision_rule.next = next_str.to_string();
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "decision rule \"next\" must be a string".to_string(),
                        ));
                    }
                }
                _ => (),
            }
        }
    }

    if decision_rule.next.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "Decision rule must have a next state".to_string(),
        ));
    }

    Ok(decision_rule)
}

fn validate_conditions(
    conditions_array: &yaml_rust2::yaml::Array,
) -> Result<Vec<CelerityWorkflowCondition>, BlueprintParseError> {
    let mut conditions = Vec::<CelerityWorkflowCondition>::new();
    for condition_value in conditions_array.iter() {
        if let yaml_rust2::Yaml::Hash(condition_map) = condition_value {
            let condition = validate_condition(condition_map)?;
            conditions.push(condition);
        }
    }
    Ok(conditions)
}

fn validate_condition(
    condition_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityWorkflowCondition, BlueprintParseError> {
    let mut condition = CelerityWorkflowCondition::default();

    if let Some(inputs_yaml) = condition_map.get(&yaml_rust2::Yaml::String("inputs".to_string())) {
        if let yaml_rust2::Yaml::Array(inputs_array) = inputs_yaml {
            let mut inputs = Vec::<MappingNode>::new();
            for input_value in inputs_array.iter() {
                let input = validate_mapping_node(input_value, "condition")?;
                inputs.push(input);
            }
            condition.inputs = inputs;
        } else {
            return Err(BlueprintParseError::YamlFormatError(
                "Condition inputs must be an array".to_string(),
            ));
        }
    } else {
        return Err(BlueprintParseError::YamlFormatError(
            "Condition must have a list of inputs".to_string(),
        ));
    }

    if let Some(function_yaml) =
        condition_map.get(&yaml_rust2::Yaml::String("function".to_string()))
    {
        if let yaml_rust2::Yaml::String(function_str) = function_yaml {
            condition.function = function_str.to_string();
        } else {
            return Err(BlueprintParseError::YamlFormatError(
                "Condition function must be a string".to_string(),
            ));
        }
    } else {
        return Err(BlueprintParseError::YamlFormatError(
            "Condition must have a function".to_string(),
        ));
    }

    Ok(condition)
}

fn validate_mapping_node(
    value: &yaml_rust2::Yaml,
    context: &str,
) -> Result<MappingNode, BlueprintParseError> {
    match value {
        yaml_rust2::Yaml::Hash(map) => {
            let mut map_value = HashMap::<String, MappingNode>::new();
            for (key, value) in map.iter() {
                if let yaml_rust2::Yaml::String(key_str) = key {
                    let value = validate_mapping_node(value, context)?;
                    map_value.insert(key_str.to_string(), value);
                }
            }
            Ok(MappingNode::Mapping(map_value))
        }
        yaml_rust2::Yaml::Array(seq) => {
            let mut seq_value = Vec::<MappingNode>::new();
            for value in seq.iter() {
                let value = validate_mapping_node(value, context)?;
                seq_value.push(value);
            }
            Ok(MappingNode::Sequence(seq_value))
        }
        yaml_rust2::Yaml::String(value_str) => Ok(MappingNode::Scalar(BlueprintScalarValue::Str(
            value_str.to_string(),
        ))),
        yaml_rust2::Yaml::Boolean(value_bool) => Ok(MappingNode::Scalar(
            BlueprintScalarValue::Bool(value_bool.clone()),
        )),
        yaml_rust2::Yaml::Integer(value_int) => Ok(MappingNode::Scalar(BlueprintScalarValue::Int(
            value_int.clone(),
        ))),
        yaml_rust2::Yaml::Real(value_float) => Ok(MappingNode::Scalar(
            BlueprintScalarValue::Float(value_float.parse::<f64>()?),
        )),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "Unsupported value type provided for mapping node in {}",
            context,
        ))),
    }
}
