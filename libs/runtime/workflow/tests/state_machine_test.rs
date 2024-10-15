use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
    time::Duration,
};

use celerity_blueprint_config_parser::blueprint::{
    BlueprintScalarValue, CelerityWorkflowCatchConfig, CelerityWorkflowCondition,
    CelerityWorkflowDecisionRule, CelerityWorkflowFailureConfig, CelerityWorkflowRetryConfig,
    CelerityWorkflowSpec, CelerityWorkflowState, CelerityWorkflowStateType,
    CelerityWorkflowWaitConfig, MappingNode,
};
use celerity_helpers::{runtime_types::RuntimePlatform, time::Clock};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tokio::{
    sync::{broadcast, RwLock},
    time::sleep,
};

use celerity_runtime_workflow::{
    consts::EVENT_BROADCASTER_CAPACITY,
    handlers::{BoxedWorkflowStateHandler, WorkflowStateHandlerError},
    payload_template::EngineV1,
    state_machine::StateMachine,
    types::WorkflowAppState,
    workflow_executions::{
        MemoryWorkflowExecutionService, WorkflowExecution, WorkflowExecutionService,
        WorkflowExecutionStatus,
    },
};

struct TestClock {
    fixtures_ms: Vec<u64>,
    index: Mutex<usize>,
}

impl TestClock {
    fn new(fixtures_ms: Vec<u64>) -> Self {
        TestClock {
            fixtures_ms,
            index: Mutex::new(0),
        }
    }
}

impl Clock for TestClock {
    fn now(&self) -> u64 {
        let mut index = self.index.lock().expect("lock should not be poisoned");
        let time = self.fixtures_ms[index.clone()] / 1000;
        *index += 1;
        time
    }

    fn now_millis(&self) -> u64 {
        let mut index = self.index.lock().expect("lock should not be poisoned");
        let time = self.fixtures_ms[index.clone()];
        *index += 1;
        time
    }
}

#[test_log::test(tokio::test)]
async fn test_state_machine_1_successful_with_retries() {
    let (tx, mut rx) = broadcast::channel(EVENT_BROADCASTER_CAPACITY);
    let collected_events_ref = Arc::new(Mutex::new(vec![]));

    tokio::spawn(async move {
        while let Ok(event) = rx.recv().await {
            let mut collected_events = collected_events_ref
                .lock()
                .expect("lock should not be poisoned");
            collected_events.push(event);
        }
    });

    let execution_service = Arc::new(MemoryWorkflowExecutionService::new());
    let state_machine = Arc::new(StateMachine::new(
        WorkflowAppState {
            platform: RuntimePlatform::Local,
            clock: Arc::new(TestClock::new(vec![1000, 1000, 2000])),
            workflow_spec: create_test_workflow_spec_1(),
            state_handlers: create_test_state_machine_1_handlers(true),
            execution_service: execution_service.clone(),
            event_broadcaster: tx,
            payload_template_engine: Arc::new(EngineV1::new()),
        },
        WorkflowExecution {
            id: "ef9ed4ef-7c2a-4eea-8f73-d7df0ebaf9d1".to_string(),
            input: json!({
                "url": "https://example.com/document.pdf"
            }),
            output: None,
            started: 0,
            completed: None,
            duration: None,
            status: WorkflowExecutionStatus::Preparing,
            status_detail: "The execution is currently being prepared".to_string(),
            current_state: None,
            states: vec![],
        },
    ));

    state_machine.start().await;

    // Allow some time for all events to have been collected.
    sleep(Duration::from_millis(10)).await;

    // Check the collected events.
    // 1. StateTransitionEvent: null -> fetchDocument
    // 2. FailureEvent: fetchDocument
    // 3. StateRetryEvent: fetchDocument
    // 4. StateTransitionEvent: fetchDocument -> scanDocument
    // 5. FailureEvent: scanDocument
    // 6. StateRetryEvent: scanDocument
    // 7. StateTransitionEvent: scanDocument -> scanResultDecision
    // 8. StateTransitionEvent: scanResultDecision -> documentProcessingDecision
    // 9. StateTransitionEvent: documentProcessingDecision -> processPDF
    // 10. StateTransitionEvent: processPDF -> waitForProcessing
    // 11. StateTransitionEvent: waitForProcessing -> uploadToSystem
    // 12. StateTransitionEvent: uploadToSystem -> success
    // 13. WorkflowExecutionCompleteEvent

    // Check execution in execution service.
    let result = execution_service
        .get_workflow_execution("ef9ed4ef-7c2a-4eea-8f73-d7df0ebaf9d1".to_string())
        .await;
    assert!(result.is_ok());
    // let execution = result.unwrap();
    // assert_eq!(
    //     execution,
    //     WorkflowExecution {
    //         id: "ef9ed4ef-7c2a-4eea-8f73-d7df0ebaf9d1".to_string(),
    //         input: json!({
    //             "url": "https://example.com/document.pdf"
    //         }),
    //         output: Some(json!({
    //             "uploaded": {
    //                 "path": "bucket://test/document.json"
    //             }
    //         })),
    //         started: 1000,
    //         completed: Some(2000),
    //         duration: Some(1000),
    //         status: WorkflowExecutionStatus::Succeeded,
    //         status_detail: "The execution completed successfully".to_string(),
    //         current_state: Some("success".to_string()),
    //         states: vec![],
    //     }
    // );
}

#[test_log::test(tokio::test)]
async fn test_state_machine_1_catches_and_handles_error() {}

fn create_test_workflow_spec_1() -> CelerityWorkflowSpec {
    CelerityWorkflowSpec {
        start_at: "fetchDocument".to_string(),
        states: HashMap::from([
            (
                "fetchDocument".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some(
                        "Fetch the document provided in the input data.".to_string(),
                    ),
                    input_path: Some("$.document".to_string()),
                    payload_template: Some(HashMap::from([(
                        "url".to_string(),
                        MappingNode::Scalar(BlueprintScalarValue::Str(
                            "$.url".to_string(),
                        )),
                    )])),
                    result_path: Some("$.downloaded".to_string()),
                    retry: Some(vec![create_test_retry_config()]),
                    catch: Some(vec![create_test_catch_config()]),
                    next: Some("scanDocument".to_string()),
                    ..Default::default()
                },
            ),
            (
                "scanDocument".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some(
                        "Scan the document for any malicious content.".to_string(),
                    ),
                    result_path: Some("$.scanResult".to_string()),
                    retry: Some(vec![create_test_retry_config()]),
                    catch: Some(vec![create_test_catch_config()]),
                    next: Some("scanResultDecision".to_string()),
                    ..Default::default()
                },
            ),
            (
                "scanResultDecision".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Decision,
                    description: Some(
                        "Decide the next state based on the scan result.".to_string(),
                    ),
                    decisions: Some(vec![
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "eq".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "$.scanResult".to_string(),
                                    )),
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "Clean".to_string(),
                                    )),
                                ],
                            }),
                            next: "documentProcessingDecision".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "eq".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "$.scanResult".to_string(),
                                    )),
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "Malicious".to_string(),
                                    )),
                                ],
                            }),
                            next: "maliciousContentFound".to_string(),
                            ..Default::default()
                        },
                    ]),
                    ..Default::default()
                },
            ),
            (
                "documentProcessingDecision".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Decision,
                    description: Some(
                        "Decide which document processing step to execute based on the document type."
                            .to_string(),
                    ),
                    input_path: Some("$.downloaded".to_string()),
                    decisions: Some(vec![
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "string_match".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "$.path".to_string(),
                                    )),
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "*.pdf".to_string(),
                                    )),
                                ],
                            }),
                            next: "processPDF".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "string_match".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "$.path".to_string(),
                                    )),
                                    MappingNode::Scalar(BlueprintScalarValue::Str(
                                        "*.docx".to_string(),
                                    )),
                                ]
                            }),
                            next: "processDOCX".to_string(),
                            ..Default::default()
                        }
                    ]),
                    ..Default::default()
                },
            ),
            (
                "processPDF".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some("Process the PDF document to extract text and metadata.".to_string()),
                    input_path: Some("$.downloaded".to_string()),
                    payload_template: Some(HashMap::from([
                        ("filePath".to_string(), MappingNode::Scalar(BlueprintScalarValue::Str("$.path".to_string())))
                    ])),
                    result_path: Some("$.extractedDataFile".to_string()),
                    retry: Some(vec![create_test_retry_config()]),
                    catch: Some(vec![create_test_catch_config()]),
                    next: Some("waitForProcessing".to_string()),
                    ..Default::default()
                },
            ),
            (
                "processDOCX".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some("Process the word document to extract text and metadata.".to_string()),
                    input_path: Some("$.downloaded".to_string()),
                    payload_template: Some(HashMap::from([
                        ("filePath".to_string(), MappingNode::Scalar(BlueprintScalarValue::Str("$.path".to_string())))
                    ])),
                    result_path: Some("$.extractedDataFile".to_string()),
                    retry: Some(vec![create_test_retry_config()]),
                    catch: Some(vec![create_test_catch_config()]),
                    next: Some("waitForProcessing".to_string()),
                    ..Default::default()
                },
            ),
            (
                "waitForProcessing".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Wait,
                    description: Some("Wait for the document processing to complete.".to_string()),
                    wait_config: Some(CelerityWorkflowWaitConfig {
                        seconds: Some("120".to_string()),
                        ..Default::default()
                    }),
                    next: Some("uploadToSystem".to_string()),
                    ..Default::default()
                },
            ),
            (
                "uploadToSystem".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some("Uploaded the extracted data to the system.".to_string()),
                    retry: Some(vec![create_test_retry_config()]),
                    catch: Some(vec![create_test_catch_config()]),
                    next: Some("success".to_string()),
                    ..Default::default()
                }
            ),
            (
                "handleError".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::ExecuteStep,
                    description: Some("Handle any error that occurred during the workflow, \
                    persisting status and error information to the domain-specific database.".to_string()),
                    input_path: Some("$.errorInfo".to_string()),
                    next: Some("failureDecision".to_string()),
                    ..Default::default()
                }
            ),
            (
                "failureDecision".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Decision,
                    description: Some("Choose the failure state to transition to based on the error type.".to_string()),
                    input_path: Some("$.errorInfo".to_string()),
                    decisions: Some(vec![
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "eq".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                    MappingNode::Scalar(BlueprintScalarValue::Str("DocumentFetchError".to_string())),
                                ],
                            }),
                            next: "fetchFailure".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "eq".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                    MappingNode::Scalar(BlueprintScalarValue::Str("DocumentScanError".to_string())),
                                ],
                            }),
                            next: "scanFailure".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            or: Some(vec![
                                CelerityWorkflowCondition {
                                    function: "eq".to_string(),
                                    inputs: vec![
                                        MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                        MappingNode::Scalar(BlueprintScalarValue::Str("ExtractPDFError".to_string())),
                                    ]
                                },
                                CelerityWorkflowCondition {
                                    function: "eq".to_string(),
                                    inputs: vec![
                                        MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                        MappingNode::Scalar(BlueprintScalarValue::Str("PDFLoadError".to_string())),
                                    ],
                                }
                            ]),
                            next: "processPDFFailure".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            or: Some(vec![
                                CelerityWorkflowCondition {
                                    function: "eq".to_string(),
                                    inputs: vec![
                                        MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                        MappingNode::Scalar(BlueprintScalarValue::Str("ExtractDOCXError".to_string())),
                                    ]
                                },
                                CelerityWorkflowCondition {
                                    function: "eq".to_string(),
                                    inputs: vec![
                                        MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                        MappingNode::Scalar(BlueprintScalarValue::Str("DOCXLoadError".to_string())),
                                    ],
                                }
                            ]),
                            next: "processDOCXFailure".to_string(),
                            ..Default::default()
                        },
                        CelerityWorkflowDecisionRule {
                            condition: Some(CelerityWorkflowCondition {
                                function: "eq".to_string(),
                                inputs: vec![
                                    MappingNode::Scalar(BlueprintScalarValue::Str("$.error".to_string())),
                                    MappingNode::Scalar(BlueprintScalarValue::Str("UploadToSystemError".to_string())),
                                ],
                            }),
                            next: "uploadToSystemFailure".to_string(),
                            ..Default::default()
                        }
                    ]),
                    ..Default::default()
                },
            ),
            (
                "success".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Success,
                    description: Some("The workflow execution completed successfully.".to_string()),
                    ..Default::default()
                }
            ),
            (
                "fetchFailure".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("The document could not be fetched.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("DocumentFetchError".to_string()),
                        cause: Some("The document could not be fetched from the provided URL.".to_string()),
                    }),
                    ..Default::default()
                }
            ),
            (
                "scanFailure".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("An error occurred while scanning the document.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("DocumentScanError".to_string()),
                        cause: Some("An error occurred while scanning the document.".to_string()),
                    }),
                    ..Default::default()
                }
            ),
            (
                "maliciousContentFound".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("Malicious content was found in the document.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("MaliciousContentFound".to_string()),
                        cause: Some("Malicious content was found in the document.".to_string()),
                    }),
                    ..Default::default()
                }
            ),
            (
                "processPDFFailure".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("An error occurred while processing the PDF document.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("PDFProcessingError".to_string()),
                        cause: Some("An error occurred while processing the PDF document.".to_string()),
                    }),
                    ..Default::default()
                }
            ),
            (
                "processDOCXFailure".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("An error occurred while processing the word document.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("DOCXProcessingError".to_string()),
                        cause: Some("An error occurred while processing the word document.".to_string()),
                    }),
                    ..Default::default()
                }
            ),
            (
                "uploadToSystemFailure".to_string(),
                CelerityWorkflowState {
                    state_type: CelerityWorkflowStateType::Failure,
                    description: Some("An error occurred while uploading the extracted data to the system.".to_string()),
                    failure_config: Some(CelerityWorkflowFailureConfig {
                        error: Some("UploadToSystemError".to_string()),
                        cause: Some("An error occurred while uploading the extracted data to the system.".to_string()),
                    }),
                    ..Default::default()
                }
            )
        ]),
    }
}

fn create_test_state_machine_1_handlers(
    scan_pass: bool,
) -> Arc<RwLock<HashMap<String, BoxedWorkflowStateHandler>>> {
    let mut hash_map = HashMap::new();
    hash_map.insert("fetchDocument".to_string(), create_fetch_doc_handler());
    hash_map.insert(
        "scanDocument".to_string(),
        create_scan_doc_handler(scan_pass),
    );
    hash_map.insert(
        "processPDF".to_string(),
        create_process_file_handler("PDF".to_string()),
    );
    hash_map.insert(
        "processDOCX".to_string(),
        create_process_file_handler("DOCX".to_string()),
    );
    hash_map.insert(
        "uploadToSystem".to_string(),
        create_upload_to_system_handler(),
    );
    Arc::new(RwLock::new(hash_map))
}

#[derive(Deserialize, Serialize)]
struct FetchDocumentInput {
    url: String,
}

fn create_fetch_doc_handler() -> BoxedWorkflowStateHandler {
    let fetch_doc_handler_call_count = Arc::new(Mutex::new(0));
    Box::new(move |value: Value| {
        let fetch_doc_handler_call_count = fetch_doc_handler_call_count.clone();
        async move {
            let mut count = fetch_doc_handler_call_count
                .lock()
                .expect("lock should not be poisoned");
            *count += 1;
            if *count == 1 {
                // Return a timeout error on the first call to trigger retry behaviour.
                return Err(WorkflowStateHandlerError {
                    name: "Timeout".to_string(),
                    message: "The document fetch timed out".to_string(),
                });
            }

            let fetch_doc_input = match serde_json::from_value::<FetchDocumentInput>(value) {
                Ok(input) => input,
                Err(e) => {
                    return Err(WorkflowStateHandlerError {
                        name: "InvalidInput".to_string(),
                        message: format!("Failed to parse input data: {}", e),
                    });
                }
            };
            let file_name = match fetch_doc_input.url.split('/').last() {
                Some(file_name) => file_name,
                None => {
                    return Err(WorkflowStateHandlerError {
                        name: "InvalidInput".to_string(),
                        message: "Failed to extract file name from URL".to_string(),
                    })
                }
            };

            Ok(json!({
                "downloaded": {
                    "path": format!("/tmp/{}", file_name)
                }
            }))
        }
    })
}

fn create_scan_doc_handler(scan_pass: bool) -> BoxedWorkflowStateHandler {
    let scan_doc_handler_call_count = Arc::new(Mutex::new(0));
    let scan_pass_ref = Arc::new(scan_pass);
    Box::new(move |_value: Value| {
        let scan_doc_handler_call_count = scan_doc_handler_call_count.clone();
        let scan_pass_ref = scan_pass_ref.clone();
        async move {
            let mut count = scan_doc_handler_call_count
                .lock()
                .expect("lock should not be poisoned");
            *count += 1;
            if *count == 1 {
                // Return a timeout error on the first call to trigger retry behaviour.
                return Err(WorkflowStateHandlerError {
                    name: "Timeout".to_string(),
                    message: "The document fetch timed out".to_string(),
                });
            }

            Ok(json!(if *scan_pass_ref { "Clean" } else { "Malicious" }))
        }
    })
}

#[derive(Deserialize, Serialize)]
struct ProcessFileInput {
    #[serde(rename = "filePath")]
    file_path: String,
}

fn create_process_file_handler(file_type: String) -> BoxedWorkflowStateHandler {
    let file_type_ref = Arc::new(file_type);
    Box::new(move |value: Value| {
        let file_type_ref = file_type_ref.clone();
        async move {
            let input = match serde_json::from_value::<ProcessFileInput>(value) {
                Ok(input) => input,
                Err(err) => {
                    return Err(WorkflowStateHandlerError {
                        name: "InvalidInput".to_string(),
                        message: format!(
                            "Failed to parse {} file input data: {}",
                            file_type_ref, err
                        ),
                    });
                }
            };

            let file_name_without_ext = match input.file_path.split("/").last() {
                Some(file_name) => file_name.split(".").next().unwrap_or_else(|| "unknown"),
                None => {
                    return Err(WorkflowStateHandlerError {
                        name: "InvalidInput".to_string(),
                        message: format!("Failed to extract {} file name from path", file_type_ref),
                    });
                }
            };

            Ok(json!({
                "extractedDataFile": {
                    "path": format!("{}.json", file_name_without_ext),
                }
            }))
        }
    })
}

#[derive(Deserialize, Serialize)]
struct ExtractedFileInput {
    extracted_data_file: ExtractedDataFile,
}

#[derive(Deserialize, Serialize)]
struct ExtractedDataFile {
    path: String,
}

fn create_upload_to_system_handler() -> BoxedWorkflowStateHandler {
    Box::new(|value: Value| async move {
        let input = match serde_json::from_value::<ExtractedFileInput>(value) {
            Ok(input) => input,
            Err(err) => {
                return Err(WorkflowStateHandlerError {
                    name: "InvalidInput".to_string(),
                    message: format!("Failed to parse input data: {}", err),
                });
            }
        };

        let file_name = match input.extracted_data_file.path.split("/").last() {
            Some(file_name) => file_name,
            None => {
                return Err(WorkflowStateHandlerError {
                    name: "InvalidInput".to_string(),
                    message: format!("Failed to extract file name from path"),
                });
            }
        };

        Ok(json!({
            "uploaded": {
                "path": format!("bucket://test/{}", file_name),
            }
        }))
    })
}

fn create_test_retry_config() -> CelerityWorkflowRetryConfig {
    CelerityWorkflowRetryConfig {
        match_errors: vec!["Timeout".to_string()],
        interval: Some(5),
        max_attempts: Some(3),
        jitter: Some(true),
        backoff_rate: Some(1.5),
        max_delay: None,
    }
}

fn create_test_catch_config() -> CelerityWorkflowCatchConfig {
    CelerityWorkflowCatchConfig {
        match_errors: vec!["*".to_string()],
        next: "handleError".to_string(),
        result_path: Some("$.errorInfo".to_string()),
    }
}
