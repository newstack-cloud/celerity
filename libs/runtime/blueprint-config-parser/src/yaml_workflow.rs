use std::collections::HashMap;

use crate::{
    blueprint::BlueprintScalarValue,
    blueprint_with_subs::{
        is_string_with_substitutions_empty, CelerityWorkflowCatchConfigWithSubs,
        CelerityWorkflowConditionWithSubs, CelerityWorkflowDecisionRuleWithSubs,
        CelerityWorkflowFailureConfigWithSubs, CelerityWorkflowParallelBranchWithSubs,
        CelerityWorkflowRetryConfigWithSubs, CelerityWorkflowSpecWithSubs,
        CelerityWorkflowStateWithSubs, CelerityWorkflowWaitConfigWithSubs, MappingNode,
        StringOrSubstitutions,
    },
    parse::BlueprintParseError,
    parse_substitutions::{parse_substitutions, ParseError},
    yaml_helpers::{
        validate_array_of_strings, validate_mapping_node, validate_single_substitution,
    },
};

// Validates the Celerity workflow spec from the parsed YAML value map.
// This will only validate the structure and not the semantics of the spec
// for a workflow resource.
// The workflow crate will validate the semantics of a parsed workflow spec
// during workflow application startup.
pub fn validate_celerity_workflow_spec(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityWorkflowSpecWithSubs, BlueprintParseError> {
    let mut workflow_spec = CelerityWorkflowSpecWithSubs::default();
    for (key, value) in value_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "startAt" => {
                    if let yaml_rust2::Yaml::String(start_at_str) = value {
                        workflow_spec.start_at = parse_substitutions::<ParseError>(start_at_str)?;
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
                        "Unsupported key for workflow spec: {key_str}"
                    )))
                }
            }
        }
    }
    Ok(workflow_spec)
}

fn validate_celerity_workflow_states(
    states_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, CelerityWorkflowStateWithSubs>, BlueprintParseError> {
    let mut states = HashMap::<String, CelerityWorkflowStateWithSubs>::new();
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
) -> Result<CelerityWorkflowStateWithSubs, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(state_map) = state_value {
        let mut state = CelerityWorkflowStateWithSubs::default();
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
                            "Unsupported key provided in workflow state \"{state_name}\": {key_str}"
                        )))
                    }
                }
            }
        }

        if is_string_with_substitutions_empty(&state.state_type) {
            return Err(BlueprintParseError::YamlFormatError(format!(
                "State type not provided for state \"{state_name}\""
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
) -> Result<StringOrSubstitutions, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(type_str) = value {
        parse_substitutions::<ParseError>(type_str).map_err(BlueprintParseError::from)
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "State type provided for state \"{state_name}\" must be a string"
        )))
    }
}

fn validate_state_description_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<StringOrSubstitutions>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(description_str) = value {
        Ok(Some(parse_substitutions::<ParseError>(description_str)?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Description provided for state \"{state_name}\" must be a string"
        )))
    }
}

fn validate_state_input_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<StringOrSubstitutions>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(input_path_str) = value {
        Ok(Some(parse_substitutions::<ParseError>(input_path_str)?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Input path provided for state \"{state_name}\" must be a string"
        )))
    }
}

fn validate_state_result_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<StringOrSubstitutions>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(result_path_str) = value {
        Ok(Some(parse_substitutions::<ParseError>(result_path_str)?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Result path provided for state \"{state_name}\" must be a string"
        )))
    }
}

fn validate_state_output_path_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<StringOrSubstitutions>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(output_path_str) = value {
        Ok(Some(parse_substitutions::<ParseError>(output_path_str)?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Output path provided for state \"{state_name}\" must be a string"
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
            "Payload template provided for state \"{state_name}\" must be a map"
        )))
    }
}

fn validate_state_next_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<StringOrSubstitutions>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(next_str) = value {
        Ok(Some(parse_substitutions::<ParseError>(next_str)?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Next state provided for state \"{state_name}\" must be a string"
        )))
    }
}

fn validate_state_end_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<MappingNode>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Boolean(end_bool) = value {
        Ok(Some(MappingNode::Scalar(BlueprintScalarValue::Bool(
            *end_bool,
        ))))
    } else if let yaml_rust2::Yaml::String(end_str) = value {
        Ok(Some(MappingNode::SubstitutionStr(
            validate_single_substitution(end_str, "boolean")?,
        )))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "End value provided for state \"{state_name}\" must be a boolean or ${{..}} substitution"
        )))
    }
}

fn validate_state_decisions_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowDecisionRuleWithSubs>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(decision_rules_array) = value {
        Ok(Some(validate_celerity_workflow_decision_rules(
            decision_rules_array,
        )?))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Decisions provided for state \"{state_name}\" must be an array"
        )))
    }
}

fn validate_state_timeout_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<MappingNode>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Integer(timeout_int) = value {
        Ok(Some(MappingNode::Scalar(BlueprintScalarValue::Int(
            *timeout_int,
        ))))
    } else if let yaml_rust2::Yaml::String(timeout_str) = value {
        Ok(Some(MappingNode::SubstitutionStr(
            validate_single_substitution(timeout_str, "integer")?,
        )))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Timeout value provided for state \"{state_name}\" must be an integer or ${{..}} substitution"
        )))
    }
}

fn validate_state_wait_config_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<CelerityWorkflowWaitConfigWithSubs>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(wait_config_map) = value {
        let mut wait_config = CelerityWorkflowWaitConfigWithSubs::default();
        for (key, value) in wait_config_map.iter() {
            if let yaml_rust2::Yaml::String(key_str) = key {
                match key_str.as_str() {
                    "seconds" => {
                        if let yaml_rust2::Yaml::String(seconds) = value {
                            wait_config.seconds = Some(parse_substitutions::<ParseError>(seconds)?);
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "Seconds value provided for wait config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    "timestamp" => {
                        if let yaml_rust2::Yaml::String(timestamp) = value {
                            wait_config.timestamp =
                                Some(parse_substitutions::<ParseError>(timestamp)?);
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
            "Wait config provided for state \"{state_name}\" must be a map"
        )))
    }
}

fn validate_state_failure_config_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<CelerityWorkflowFailureConfigWithSubs>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Hash(failure_config_map) = value {
        let mut failure_config = CelerityWorkflowFailureConfigWithSubs::default();
        for (key, value) in failure_config_map.iter() {
            if let yaml_rust2::Yaml::String(key_str) = key {
                match key_str.as_str() {
                    "error" => {
                        if let yaml_rust2::Yaml::String(error) = value {
                            failure_config.error = Some(parse_substitutions::<ParseError>(error)?);
                        } else {
                            return Err(BlueprintParseError::YamlFormatError(
                                "error value provided for failure config must be a string"
                                    .to_string(),
                            ));
                        }
                    }
                    "cause" => {
                        if let yaml_rust2::Yaml::String(cause) = value {
                            failure_config.cause = Some(parse_substitutions::<ParseError>(cause)?);
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
            "Failure config provided for state \"{state_name}\" must be a map"
        )))
    }
}

fn validate_state_parallel_branches_field(
    value: &yaml_rust2::Yaml,
    state_name: &str,
) -> Result<Option<Vec<CelerityWorkflowParallelBranchWithSubs>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(parallel_branches_array) = value {
        let mut parallel_branches = Vec::<CelerityWorkflowParallelBranchWithSubs>::new();
        for branch_value in parallel_branches_array.iter() {
            if let yaml_rust2::Yaml::Hash(branch_map) = branch_value {
                let branch = validate_workflow_state_parallel_branch(branch_map, state_name)?;
                parallel_branches.push(branch);
            }
        }
        Ok(Some(parallel_branches))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Parallel branches provided for state \"{state_name}\" must be an array"
        )))
    }
}

fn validate_workflow_state_parallel_branch(
    branch_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowParallelBranchWithSubs, BlueprintParseError> {
    let mut parallel_branch = CelerityWorkflowParallelBranchWithSubs::default();
    for (key, value) in branch_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "startAt" => {
                    if let yaml_rust2::Yaml::String(start_at_str) = value {
                        parallel_branch.start_at = parse_substitutions::<ParseError>(start_at_str)?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Start at state value provided for parallel branch must be a string in state \"{state_name}\""
                        )));
                    }
                }
                "states" => {
                    if let yaml_rust2::Yaml::Hash(states_map) = value {
                        parallel_branch.states = validate_celerity_workflow_states(states_map)?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "States provided for parallel branch in state \"{state_name}\" must be a map"
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
) -> Result<Option<Vec<CelerityWorkflowRetryConfigWithSubs>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(retry_config_array) = value {
        let mut retry_config_list = Vec::<CelerityWorkflowRetryConfigWithSubs>::new();
        for retry_config_value in retry_config_array.iter() {
            if let yaml_rust2::Yaml::Hash(retry_config_map) = retry_config_value {
                let retry_config = validate_retry_config(retry_config_map, state_name)?;
                retry_config_list.push(retry_config);
            }
        }
        Ok(Some(retry_config_list))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Retry field for state \"{state_name}\" must be an array"
        )))
    }
}

fn validate_retry_config(
    retry_config_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowRetryConfigWithSubs, BlueprintParseError> {
    let mut retry_config = CelerityWorkflowRetryConfigWithSubs::default();
    for (key, value) in retry_config_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "matchErrors" => {
                    if let yaml_rust2::Yaml::Array(match_errors_yaml_array) = value {
                        retry_config.match_errors =
                            validate_array_of_strings(match_errors_yaml_array, "matchErrors")?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Match errors field for retry config in state \"{state_name}\" must be an array"
                        )));
                    }
                }
                "interval" => {
                    if let yaml_rust2::Yaml::Integer(interval_seconds_int) = value {
                        retry_config.interval = Some(MappingNode::Scalar(
                            BlueprintScalarValue::Int(*interval_seconds_int),
                        ));
                    } else if let yaml_rust2::Yaml::String(interval_str) = value {
                        retry_config.interval = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(interval_str, "integer")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Interval field for retry config in state \"{state_name}\" must be an integer or ${{..}} substitution"
                        )));
                    }
                }
                "maxAttempts" => {
                    if let yaml_rust2::Yaml::Integer(max_attempts_int) = value {
                        retry_config.max_attempts = Some(MappingNode::Scalar(
                            BlueprintScalarValue::Int(*max_attempts_int),
                        ));
                    } else if let yaml_rust2::Yaml::String(max_attempts_str) = value {
                        retry_config.max_attempts = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(max_attempts_str, "integer")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Max attempts field for retry config in state \"{state_name}\" must be an integer or ${{..}} substitution"
                        )));
                    }
                }
                "maxDelay" => {
                    if let yaml_rust2::Yaml::Integer(max_delay_seconds_int) = value {
                        retry_config.max_delay = Some(MappingNode::Scalar(
                            BlueprintScalarValue::Int(*max_delay_seconds_int),
                        ));
                    } else if let yaml_rust2::Yaml::String(max_delay_str) = value {
                        retry_config.max_delay = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(max_delay_str, "integer")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Max delay field for retry config in state \"{state_name}\" must be an integer or ${{..}} substitution"
                        )));
                    }
                }
                "jitter" => {
                    if let yaml_rust2::Yaml::Boolean(jitter_bool) = value {
                        retry_config.jitter = Some(MappingNode::Scalar(
                            BlueprintScalarValue::Bool(*jitter_bool),
                        ));
                    } else if let yaml_rust2::Yaml::String(jitter_str) = value {
                        retry_config.jitter = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(jitter_str, "boolean")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Jitter field for retry config in state \"{state_name}\" must be a boolean or ${{..}} substitution"
                        )));
                    }
                }
                "backoffRate" => {
                    if let yaml_rust2::Yaml::Real(backoff_rate_str) = value {
                        retry_config.backoff_rate = Some(MappingNode::Scalar(
                            BlueprintScalarValue::Float(backoff_rate_str.parse()?),
                        ));
                    } else if let yaml_rust2::Yaml::String(backoff_rate_str) = value {
                        retry_config.backoff_rate = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(backoff_rate_str, "float")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Backoff rate field for retry config in state \"{state_name}\" must be a float or ${{..}} substitution"
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
) -> Result<Option<Vec<CelerityWorkflowCatchConfigWithSubs>>, BlueprintParseError> {
    if let yaml_rust2::Yaml::Array(catch_config_array) = value {
        let mut catch_config_list = Vec::<CelerityWorkflowCatchConfigWithSubs>::new();
        for catch_config_value in catch_config_array.iter() {
            if let yaml_rust2::Yaml::Hash(catch_config_map) = catch_config_value {
                let catch_config = validate_catch_config(catch_config_map, state_name)?;
                catch_config_list.push(catch_config);
            }
        }
        Ok(Some(catch_config_list))
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "Catch field for state \"{state_name}\" must be an array"
        )))
    }
}

fn validate_catch_config(
    catch_config_map: &yaml_rust2::yaml::Hash,
    state_name: &str,
) -> Result<CelerityWorkflowCatchConfigWithSubs, BlueprintParseError> {
    let mut catch_config = CelerityWorkflowCatchConfigWithSubs::default();
    for (key, value) in catch_config_map.iter() {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "matchErrors" => {
                    if let yaml_rust2::Yaml::Array(match_errors_yaml_array) = value {
                        catch_config.match_errors =
                            validate_array_of_strings(match_errors_yaml_array, "matchErrors")?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Match errors field for catch config in state \"{state_name}\" must be an array"
                        )));
                    }
                }
                "next" => {
                    if let yaml_rust2::Yaml::String(next_str) = value {
                        catch_config.next = parse_substitutions::<ParseError>(next_str)?;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Next field for catch config in state \"{state_name}\" must be a string"
                        )));
                    }
                }
                "resultPath" => {
                    if let yaml_rust2::Yaml::String(result_path_str) = value {
                        catch_config.result_path =
                            Some(parse_substitutions::<ParseError>(result_path_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(format!(
                            "Result path field for catch config in state \"{state_name}\" must be a string"
                        )));
                    }
                }
                _ => (),
            }
        }
    }
    Ok(catch_config)
}

fn validate_payload_template(
    payload_template_map: &yaml_rust2::yaml::Hash,
    state: &str,
) -> Result<HashMap<String, MappingNode>, BlueprintParseError> {
    let mut payload_template = HashMap::<String, MappingNode>::new();
    let context = format!("payload template for state \"{state}\"");
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
) -> Result<Vec<CelerityWorkflowDecisionRuleWithSubs>, BlueprintParseError> {
    let mut decision_rules = Vec::<CelerityWorkflowDecisionRuleWithSubs>::new();
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
) -> Result<CelerityWorkflowDecisionRuleWithSubs, BlueprintParseError> {
    let mut decision_rule = CelerityWorkflowDecisionRuleWithSubs::default();
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
                        decision_rule.next = parse_substitutions::<ParseError>(next_str)?;
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

    if is_string_with_substitutions_empty(&decision_rule.next) {
        return Err(BlueprintParseError::YamlFormatError(
            "Decision rule must have a next state".to_string(),
        ));
    }

    Ok(decision_rule)
}

fn validate_conditions(
    conditions_array: &yaml_rust2::yaml::Array,
) -> Result<Vec<CelerityWorkflowConditionWithSubs>, BlueprintParseError> {
    let mut conditions = Vec::<CelerityWorkflowConditionWithSubs>::new();
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
) -> Result<CelerityWorkflowConditionWithSubs, BlueprintParseError> {
    let mut condition = CelerityWorkflowConditionWithSubs::default();

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
            condition.function = parse_substitutions::<ParseError>(function_str)?;
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
