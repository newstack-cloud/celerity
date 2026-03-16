use std::{collections::HashMap, sync::Arc, time::Duration};

use async_trait::async_trait;
use celerity_runtime_core::{
  consumer_handler::{ConsumerEventHandler, ConsumerEventHandlerError},
  types::{
    ConsumerEventData, EventResult, EventResultData, MessageProcessingFailure,
    MessageProcessingResponseData, ScheduleEventData, ScheduledEventResponseData,
  },
};
use napi::bindgen_prelude::*;
use napi::threadsafe_function::ThreadsafeFunction;
use napi_derive::napi;
use tokio::time;

// ---------------------------------------------------------------------------
// ThreadsafeFunction type aliases
// ---------------------------------------------------------------------------

/// Weak tsfn for consumer event handlers (JS callback receives JsConsumerEventInput).
pub(crate) type ConsumerWeakTsfn = ThreadsafeFunction<
  JsConsumerEventInput,
  Promise<JsEventResult>,
  JsConsumerEventInput,
  Status,
  true,
  true,
>;

/// Weak tsfn for schedule event handlers (JS callback receives JsScheduleEventInput).
pub(crate) type ScheduleWeakTsfn = ThreadsafeFunction<
  JsScheduleEventInput,
  Promise<JsEventResult>,
  JsScheduleEventInput,
  Status,
  true,
  true,
>;

// ---------------------------------------------------------------------------
// JS-facing input/output types
// ---------------------------------------------------------------------------

/// The handler input for when consumer event messages are received from an event source.
#[napi(object)]
pub struct JsConsumerEventInput {
  /// A tag identifying the handler, in the format "source::\<source_id\>::\<handler_name\>".
  pub handler_tag: String,
  /// List of consumer messages in the batch.
  pub messages: Vec<JsConsumerMessage>,
  /// Vendor-specific metadata for the event source (e.g. AWS SQS metadata).
  pub vendor: serde_json::Value,
  /// A dictionary of trace context including a W3C Trace Context string
  /// (in the traceparent format) and platform specific trace IDs.
  pub trace_context: Option<HashMap<String, String>>,
}

/// A single message from an event source consumer.
#[napi(object)]
pub struct JsConsumerMessage {
  /// The unique identifier of the message.
  pub message_id: String,
  /// The message body as a string.
  pub body: String,
  /// The source of the message (e.g. queue URL or stream name).
  pub source: String,
  /// The type of source parsed from the `celerity:{type}:{name}` format
  /// (e.g. "bucket", "datastore", "queue", "topic").
  pub source_type: Option<String>,
  /// The name of the source parsed from the `celerity:{type}:{name}` format.
  pub source_name: Option<String>,
  /// The Celerity-standard event type (e.g. "created", "deleted", "inserted", "modified").
  /// Present for bucket and datastore consumers only.
  pub event_type: Option<String>,
  /// Vendor-specific message attributes.
  pub message_attributes: serde_json::Value,
  /// Vendor-specific metadata for the message.
  pub vendor: serde_json::Value,
}

/// The handler input for when a scheduled event is triggered.
#[napi(object)]
pub struct JsScheduleEventInput {
  /// A tag identifying the handler, in the format "source::\<schedule_id\>::\<handler_name\>".
  pub handler_tag: String,
  /// The identifier of the schedule that triggered the event.
  pub schedule_id: String,
  /// The unique identifier of the schedule event message.
  pub message_id: String,
  /// The schedule expression (e.g. cron expression or rate).
  pub schedule: String,
  /// Optional input data configured for the schedule.
  pub input: Option<serde_json::Value>,
  /// Vendor-specific metadata for the schedule event.
  pub vendor: serde_json::Value,
  /// A dictionary of trace context including a W3C Trace Context string
  /// (in the traceparent format) and platform specific trace IDs.
  pub trace_context: Option<HashMap<String, String>>,
}

/// The result returned from a consumer or schedule event handler.
#[napi(object)]
pub struct JsEventResult {
  /// Whether the event was processed successfully.
  pub success: bool,
  /// Optional list of individual message processing failures
  /// (for partial failure reporting in consumer handlers).
  pub failures: Option<Vec<JsMessageProcessingFailure>>,
  /// Optional error message (used for schedule handler failures).
  pub error_message: Option<String>,
}

/// Represents a failure to process an individual message in a consumer batch.
#[napi(object)]
pub struct JsMessageProcessingFailure {
  /// The ID of the message that failed to process.
  pub message_id: String,
  /// Optional description of the error.
  pub error_message: Option<String>,
}

// ---------------------------------------------------------------------------
// Conversions: core types → JS types
// ---------------------------------------------------------------------------

impl JsConsumerEventInput {
  pub fn from_core(handler_tag: &str, event_data: ConsumerEventData) -> Self {
    Self {
      handler_tag: handler_tag.to_string(),
      messages: event_data
        .messages
        .into_iter()
        .map(|m| JsConsumerMessage {
          message_id: m.message_id,
          body: m.body,
          source: m.source,
          source_type: m.source_type,
          source_name: m.source_name,
          event_type: m.event_type,
          message_attributes: m.message_attributes,
          vendor: m.vendor,
        })
        .collect(),
      vendor: event_data.vendor,
      trace_context: celerity_runtime_core::telemetry_utils::extract_trace_context(),
    }
  }
}

impl JsScheduleEventInput {
  pub fn from_core(handler_tag: &str, event_data: ScheduleEventData) -> Self {
    Self {
      handler_tag: handler_tag.to_string(),
      schedule_id: event_data.schedule_id,
      message_id: event_data.message_id,
      schedule: event_data.schedule,
      input: event_data.input,
      vendor: event_data.vendor,
      trace_context: celerity_runtime_core::telemetry_utils::extract_trace_context(),
    }
  }
}

// ---------------------------------------------------------------------------
// Conversions: JS result → core EventResult
// ---------------------------------------------------------------------------

fn js_result_to_consumer_event_result(result: JsEventResult, event_id: &str) -> EventResult {
  EventResult {
    event_id: event_id.to_string(),
    data: EventResultData::MessageProcessingResponse(MessageProcessingResponseData {
      success: result.success,
      failures: result.failures.map(|fs| {
        fs.into_iter()
          .map(|f| MessageProcessingFailure {
            message_id: f.message_id,
            error_message: f.error_message,
          })
          .collect()
      }),
    }),
    context: None,
  }
}

fn js_result_to_schedule_event_result(result: JsEventResult, event_id: &str) -> EventResult {
  EventResult {
    event_id: event_id.to_string(),
    data: EventResultData::ScheduledEventResponse(ScheduledEventResponseData {
      success: result.success,
      error_message: result.error_message,
    }),
    context: None,
  }
}

// ---------------------------------------------------------------------------
// NapiConsumerEventHandlerBuilder — accumulates per-handler_tag tsfns
// ---------------------------------------------------------------------------

pub(crate) struct NapiConsumerEventHandlerBuilder {
  consumer_handlers: HashMap<String, (Arc<ConsumerWeakTsfn>, u64)>,
  schedule_handlers: HashMap<String, (Arc<ScheduleWeakTsfn>, u64)>,
}

impl Default for NapiConsumerEventHandlerBuilder {
  fn default() -> Self {
    Self::new()
  }
}

impl NapiConsumerEventHandlerBuilder {
  pub fn new() -> Self {
    Self {
      consumer_handlers: HashMap::new(),
      schedule_handlers: HashMap::new(),
    }
  }

  pub fn add_consumer_handler(
    &mut self,
    handler_tag: String,
    timeout_secs: u64,
    tsfn: Arc<ConsumerWeakTsfn>,
  ) {
    self
      .consumer_handlers
      .insert(handler_tag, (tsfn, timeout_secs));
  }

  pub fn add_schedule_handler(
    &mut self,
    handler_tag: String,
    timeout_secs: u64,
    tsfn: Arc<ScheduleWeakTsfn>,
  ) {
    self
      .schedule_handlers
      .insert(handler_tag, (tsfn, timeout_secs));
  }

  pub fn is_empty(&self) -> bool {
    self.consumer_handlers.is_empty() && self.schedule_handlers.is_empty()
  }

  pub fn build(self) -> NapiConsumerEventHandler {
    NapiConsumerEventHandler {
      consumer_handlers: self.consumer_handlers,
      schedule_handlers: self.schedule_handlers,
    }
  }
}

// ---------------------------------------------------------------------------
// NapiConsumerEventHandler — dispatches by handler_tag
// ---------------------------------------------------------------------------

pub struct NapiConsumerEventHandler {
  consumer_handlers: HashMap<String, (Arc<ConsumerWeakTsfn>, u64)>,
  schedule_handlers: HashMap<String, (Arc<ScheduleWeakTsfn>, u64)>,
}

#[async_trait]
impl ConsumerEventHandler for NapiConsumerEventHandler {
  async fn handle_consumer_event(
    &self,
    handler_tag: &str,
    event_data: ConsumerEventData,
  ) -> std::result::Result<EventResult, ConsumerEventHandlerError> {
    // handler_tag format: "source::<source_id>::<handler_name>"
    // JS registers handlers by resource name (the last segment).
    let handler_name = handler_tag.rsplit("::").next().unwrap_or(handler_tag);
    let (tsfn, timeout_secs) = self
      .consumer_handlers
      .get(handler_name)
      .ok_or(ConsumerEventHandlerError::MissingHandler)?;

    let js_input = JsConsumerEventInput::from_core(handler_tag, event_data);
    let promise = tsfn
      .call_async(Ok(js_input))
      .await
      .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;

    let sleep = time::sleep(Duration::from_secs(*timeout_secs));
    tokio::select! {
        _ = sleep => {
            Err(ConsumerEventHandlerError::Timeout)
        }
        value = promise => {
            let result = value.map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;
            Ok(js_result_to_consumer_event_result(result, handler_tag))
        }
    }
  }

  async fn handle_schedule_event(
    &self,
    handler_tag: &str,
    event_data: ScheduleEventData,
  ) -> std::result::Result<EventResult, ConsumerEventHandlerError> {
    // handler_tag format: "source::<schedule_id>::<handler_name>"
    // JS registers handlers by resource name (the last segment).
    let handler_name = handler_tag.rsplit("::").next().unwrap_or(handler_tag);
    let (tsfn, timeout_secs) = self
      .schedule_handlers
      .get(handler_name)
      .ok_or(ConsumerEventHandlerError::MissingHandler)?;

    let js_input = JsScheduleEventInput::from_core(handler_tag, event_data);
    let promise = tsfn
      .call_async(Ok(js_input))
      .await
      .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;

    let sleep = time::sleep(Duration::from_secs(*timeout_secs));
    tokio::select! {
        _ = sleep => {
            Err(ConsumerEventHandlerError::Timeout)
        }
        value = promise => {
            let result = value.map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;
            Ok(js_result_to_schedule_event_result(result, handler_tag))
        }
    }
  }
}
