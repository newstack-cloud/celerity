//! Converts between the runtime's internal events and the protocol frames that
//! carry them to a handlers executable.
//!
//! The internal event types are serde types that predate this protocol: bodies
//! are strings, and a body that is not text is base64 encoded because the
//! events were carried as JSON, which has no binary representation. The
//! protocol has no such limit, so converting a body here decodes it back to the
//! bytes it always was. The encoding hop exists only on the internal side and
//! disappears entirely once the event types carry bytes themselves.

use std::collections::HashMap;

use bytes::Bytes;
use serde_json::Value;
use tracing::{error, warn};

use crate::{
    consts::UNEXPECTED_ERROR_BODY,
    ipc_proto as proto,
    types::{
        ConsumerEventData, CustomInvokeEventData, CustomInvokeResponseData, EventData,
        EventDataPayload, EventMessageEventData, EventResult, EventResultData, EventType,
        HttpRequestEventData, HttpResponseData, MessageProcessingFailure,
        MessageProcessingResponseData, ScheduleEventData, ScheduledEventResponseData,
        SimpleResponseData, WebSocketEventData,
    },
};

/// Turns an event into the frame that carries it to a handler.
pub fn dispatch_from_event(event: EventData, deadline_unix_ms: i64) -> proto::Dispatch {
    proto::Dispatch {
        id: event.id,
        handler_tag: event.handler_tag,
        timestamp_ms: event.timestamp.saturating_mul(1_000),
        deadline_unix_ms,
        trace_context: event.trace_context.unwrap_or_default(),
        source: Some(source_from_payload(event.data)),
    }
}

/// Carries a custom invocation's input to the handler.
///
/// The input is whatever the caller supplied, serialised as JSON, since the
/// runtime does not interpret it and the handler is the only thing that knows
/// what shape it should be. An absent input travels as no bytes at all rather
/// than as the four bytes spelling `null`, so a handler can tell "nothing was
/// sent" from "null was sent".
fn custom_invoke(invoke: CustomInvokeEventData) -> proto::CustomInvoke {
    proto::CustomInvoke {
        handler_name: invoke.handler_name,
        input: invoke
            .input
            .map(|input| input.to_string().into_bytes())
            .unwrap_or_default(),
    }
}

/// Reads back what a custom handler returned.
///
/// The output stays as text. It is on its way to a caller that will render it
/// as JSON, so parsing it here only to serialise it again would be work that
/// could also fail on a payload the handler considers valid.
fn custom_invoke_response(custom: proto::CustomInvokeResult) -> CustomInvokeResponseData {
    CustomInvokeResponseData {
        output: Bytes::from(custom.output),
        error_message: none_if_empty(custom.error_message),
    }
}

fn source_from_payload(payload: EventDataPayload) -> proto::dispatch::Source {
    match payload {
        EventDataPayload::HttpRequestEventData(request) => {
            proto::dispatch::Source::Http(http_request(*request))
        }
        EventDataPayload::WsMessageEventData(message) => {
            proto::dispatch::Source::Websocket(websocket_message(message))
        }
        EventDataPayload::ConsumerMessageEventData(batch) => {
            proto::dispatch::Source::Consumer(consumer_batch(batch))
        }
        EventDataPayload::ScheduleMessageEventData(trigger) => {
            proto::dispatch::Source::Schedule(schedule_trigger(trigger))
        }
        // An event trigger delivers one message from a bucket or stream. To a
        // handler that is the same shape as a consumer message, and both are
        // addressed by the same `source::{id}::{handler}` tag, so it travels as
        // a batch of one rather than as a source of its own.
        EventDataPayload::EventMessageEventData(message) => {
            proto::dispatch::Source::Consumer(event_message_batch(message))
        }
        EventDataPayload::CustomInvokeEventData(invoke) => {
            proto::dispatch::Source::Custom(custom_invoke(invoke))
        }
    }
}

fn http_request(request: HttpRequestEventData) -> proto::HttpRequest {
    proto::HttpRequest {
        method: request.method.to_uppercase(),
        path: request.path,
        route: request.route,
        path_params: values_map(request.path_params),
        query_params: values_map(request.query_params),
        headers: values_map(request.headers),
        source_ip: request.source_ip,
        request_id: request.request_id,
        body: request.body.to_vec(),
    }
}

fn websocket_message(message: WebSocketEventData) -> proto::WebSocketMessage {
    proto::WebSocketMessage {
        route: message.route,
        connection_id: message.connection_id,
        source_ip: message.source_ip,
        request_id: message.request_id.unwrap_or_default(),
        message: message.message.to_vec(),
        is_binary: message.is_binary,
        message_id: message.message_id.unwrap_or_default(),
    }
}

fn consumer_batch(batch: ConsumerEventData) -> proto::ConsumerBatch {
    let source_id = batch
        .messages
        .first()
        .map(|message| message.source.clone())
        .unwrap_or_default();
    let source_type = batch
        .messages
        .first()
        .and_then(|message| message.source_type.clone())
        .unwrap_or_default();

    proto::ConsumerBatch {
        records: batch
            .messages
            .into_iter()
            .map(|message| proto::ConsumerRecord {
                message_id: message.message_id,
                body: message.body.into_bytes(),
                source: message.source,
                event_type: message.event_type.unwrap_or_default(),
                attributes: json_bytes(Some(message.message_attributes)),
                vendor: json_bytes(Some(message.vendor)),
            })
            .collect(),
        source_id,
        source_type,
        vendor: json_bytes(Some(batch.vendor)),
    }
}

fn event_message_batch(message: EventMessageEventData) -> proto::ConsumerBatch {
    proto::ConsumerBatch {
        source_id: message.source.clone(),
        source_type: String::new(),
        vendor: json_bytes(Some(message.vendor.clone())),
        records: vec![proto::ConsumerRecord {
            message_id: message.message_id.unwrap_or_default(),
            body: message.body.into_bytes(),
            source: message.source,
            event_type: String::new(),
            attributes: json_bytes(message.message_attributes),
            vendor: json_bytes(Some(message.vendor)),
        }],
    }
}

fn schedule_trigger(trigger: ScheduleEventData) -> proto::ScheduleTrigger {
    proto::ScheduleTrigger {
        schedule_id: trigger.schedule_id,
        message_id: trigger.message_id,
        schedule: trigger.schedule,
        input: json_bytes(trigger.input),
        vendor: json_bytes(Some(trigger.vendor)),
    }
}

/// Turns a handler's result frame back into the result the waiting caller
/// expects.
///
/// A frame with no outcome is a protocol error rather than a success, so it is
/// reported as a failure of whatever kind the caller is waiting for.
pub fn event_result_from_frame(result: proto::Result, waiting_for: &EventType) -> EventResult {
    let data = match result.outcome {
        Some(proto::result::Outcome::Http(response)) => {
            EventResultData::HttpResponse(http_response(response))
        }
        Some(proto::result::Outcome::Websocket(ack)) => {
            EventResultData::WebSocketResponse(simple_response(ack))
        }
        Some(proto::result::Outcome::Schedule(ack)) => {
            EventResultData::ScheduledEventResponse(ScheduledEventResponseData {
                success: ack.success,
                error_message: none_if_empty(ack.error_message),
            })
        }
        Some(proto::result::Outcome::Consumer(batch)) => {
            EventResultData::MessageProcessingResponse(batch_result(batch))
        }
        Some(proto::result::Outcome::Custom(custom)) => {
            EventResultData::CustomInvokeResponse(custom_invoke_response(custom))
        }
        Some(proto::result::Outcome::Error(error)) => {
            record_handler_error(&result.id, &error);
            failure_for(waiting_for, handler_error_message(&error))
        }
        None => {
            warn!(
                event_id = %result.id,
                "handler returned a result with no outcome"
            );
            failure_for(waiting_for, "handler returned an empty result".to_string())
        }
    };

    EventResult {
        event_id: result.id,
        data,
        context: None,
    }
}

fn http_response(response: proto::HttpResponse) -> HttpResponseData {
    HttpResponseData {
        status: status_code(response.status),
        headers: response
            .headers
            .into_iter()
            .map(|(name, values)| (name, values.values))
            .collect(),
        body: Bytes::from(response.body),
    }
}

/// Narrows a status from the wire, where the smallest type protobuf offers is
/// wider than a status can be.
///
/// Truncating instead would let a value wrap into a perfectly ordinary code:
/// 65736 becomes 200, so a handler that sent nonsense would report success.
/// Anything that cannot be a status is a fault in the handler, and reported as
/// one. A value that fits but is still not a status, such as 4464, is caught
/// again when the response is built.
fn status_code(status: u32) -> u16 {
    u16::try_from(status).unwrap_or_else(|_| {
        warn!(
            status,
            "handler returned a status that cannot be one, answering 500"
        );
        500
    })
}

/// Reports a failure in the shape the caller is waiting for.
///
/// A handler that fails with no outcome of its own, or with an unhandled error,
/// still has to answer the caller in the shape that caller expects. Answering
/// every one of them as an HTTP response means a WebSocket or a custom
/// invocation sees a result it cannot read, and reports that instead of what
/// actually went wrong, which is the one thing worth reporting.
fn failure_for(waiting_for: &EventType, message: String) -> EventResultData {
    match waiting_for {
        EventType::HttpRequest => EventResultData::HttpResponse(HttpResponseData {
            status: 500,
            headers: HashMap::new(),
            body: Bytes::from_static(UNEXPECTED_ERROR_BODY.as_bytes()),
        }),
        EventType::WsMessage => EventResultData::WebSocketResponse(SimpleResponseData {
            success: false,
            error_message: Some(message),
        }),
        EventType::ScheduleMessage => {
            EventResultData::ScheduledEventResponse(ScheduledEventResponseData {
                success: false,
                error_message: Some(message),
            })
        }
        // The batch result has nowhere to put a message that belongs to no
        // particular record, so it is logged rather than quietly dropped.
        EventType::ConsumerMessage | EventType::EventMessage => {
            warn!(%message, "handler failed to process a batch");
            EventResultData::MessageProcessingResponse(MessageProcessingResponseData {
                success: false,
                failures: None,
            })
        }
        EventType::CustomInvoke => {
            EventResultData::CustomInvokeResponse(CustomInvokeResponseData {
                output: Bytes::new(),
                error_message: Some(message),
            })
        }
    }
}

/// Records what a handler reported about an unhandled error in user code.
///
/// The stack is recorded here and nowhere else. The message this function's
/// caller goes on to build becomes what the source was waiting for, which for
/// an HTTP request is the body its client receives, and a stack trace names
/// the files, line numbers and internal frames of the process serving it.
fn record_handler_error(event_id: &str, error: &proto::HandlerError) {
    if error.stack.is_empty() {
        error!(
            event_id = %event_id,
            error_type = %error.r#type,
            "handler reported an unhandled error: {}",
            error.message
        );
        return;
    }

    error!(
        event_id = %event_id,
        error_type = %error.r#type,
        stack = %error.stack,
        "handler reported an unhandled error: {}",
        error.message
    );
}

/// The message an unhandled error carries, with its type when it has one.
///
/// This is what the source waiting on the event is told, so it reaches whoever
/// asked. Deliberately without the stack, see [`record_handler_error`].
fn handler_error_message(error: &proto::HandlerError) -> String {
    if error.r#type.is_empty() {
        error.message.clone()
    } else {
        format!("{}: {}", error.r#type, error.message)
    }
}

fn simple_response(ack: proto::Ack) -> SimpleResponseData {
    SimpleResponseData {
        success: ack.success,
        error_message: none_if_empty(ack.error_message),
    }
}

fn batch_result(batch: proto::BatchResult) -> MessageProcessingResponseData {
    let failures: Vec<MessageProcessingFailure> = batch
        .failures
        .into_iter()
        .map(|failure| MessageProcessingFailure {
            message_id: failure.message_id,
            error_message: none_if_empty(failure.error_message),
        })
        .collect();

    MessageProcessingResponseData {
        success: batch.success,
        failures: (!failures.is_empty()).then_some(failures),
    }
}

fn json_bytes(value: Option<Value>) -> Vec<u8> {
    match value {
        Some(value) => serde_json::to_vec(&value).unwrap_or_else(|err| {
            warn!("failed to serialise a payload field: {err}");
            Vec::new()
        }),
        None => Vec::new(),
    }
}

/// Carries an ordered set of values per name through to the protocol.
///
/// The internal types and the protocol agree on this shape, so nothing has to
/// be chosen between or dropped here.
fn values_map(values: HashMap<String, Vec<String>>) -> HashMap<String, proto::Values> {
    values
        .into_iter()
        .map(|(name, values)| (name, proto::Values { values }))
        .collect()
}

fn none_if_empty(value: String) -> Option<String> {
    (!value.is_empty()).then_some(value)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;
    use crate::types::{ConsumerMessage, EventType};

    fn http_event(body: &'static [u8]) -> EventData {
        EventData {
            id: "event-1".to_string(),
            event_type: EventType::HttpRequest,
            handler_tag: "GET::/orders/{id}".to_string(),
            timestamp: 1_700_000_000,
            data: EventDataPayload::HttpRequestEventData(Box::new(HttpRequestEventData {
                method: "get".to_string(),
                path: "/orders/1".to_string(),
                route: "/orders/{id}".to_string(),
                path_params: HashMap::from([("id".to_string(), vec!["1".to_string()])]),
                query_params: HashMap::from([(
                    "expand".to_string(),
                    vec!["items".to_string(), "totals".to_string()],
                )]),
                headers: HashMap::from([(
                    "accept".to_string(),
                    vec!["application/json".to_string()],
                )]),
                body: Bytes::from_static(body),
                source_ip: "10.0.0.1".to_string(),
                request_id: "request-1".to_string(),
            })),
            trace_context: None,
        }
    }

    #[test]
    fn carries_an_http_request_onto_the_wire() {
        let dispatch = dispatch_from_event(http_event(b"{}"), 42);

        assert_eq!(dispatch.id, "event-1");
        assert_eq!(dispatch.deadline_unix_ms, 42);
        // Seconds internally, milliseconds on the wire.
        assert_eq!(dispatch.timestamp_ms, 1_700_000_000_000);

        let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
            panic!("expected an HTTP source");
        };
        assert_eq!(request.method, "GET");
        assert_eq!(request.body, b"{}");
        assert_eq!(
            request.path_params.get("id").map(|v| v.values.clone()),
            Some(vec!["1".to_string()])
        );
    }

    /// A handler's spans are children of the runtime's only if the context
    /// reaches it, and the field carrying it was previously always empty.
    #[test]
    fn carries_the_producer_trace_context_onto_the_wire() {
        let mut event = http_event(b"{}");
        event.trace_context = Some(HashMap::from([
            (
                "traceparent".to_string(),
                "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01".to_string(),
            ),
            ("tracestate".to_string(), "vendor=value".to_string()),
        ]));

        let dispatch = dispatch_from_event(event, 42);

        assert_eq!(
            dispatch
                .trace_context
                .get("traceparent")
                .map(String::as_str),
            Some("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
        );
        assert_eq!(
            dispatch.trace_context.get("tracestate").map(String::as_str),
            Some("vendor=value")
        );
    }

    /// An event produced with tracing off carries nothing, which has to reach
    /// the handler as an empty map rather than failing to build the frame.
    #[test]
    fn carries_an_empty_trace_context_for_an_event_that_has_none() {
        let dispatch = dispatch_from_event(http_event(b"{}"), 42);

        assert!(dispatch.trace_context.is_empty());
    }

    #[test]
    fn carries_every_value_a_name_was_sent_with() {
        let dispatch = dispatch_from_event(http_event(b""), 0);
        let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
            panic!("expected an HTTP source");
        };

        assert_eq!(
            request.query_params.get("expand").map(|v| v.values.clone()),
            Some(vec!["items".to_string(), "totals".to_string()])
        );
        assert_eq!(
            request.headers.get("accept").map(|v| v.values.clone()),
            Some(vec!["application/json".to_string()])
        );
    }

    #[test]
    fn carries_a_body_that_is_not_text_byte_for_byte() {
        let raw: &[u8] = &[0xff, 0xfe, 0x00, 0x80];
        let dispatch = dispatch_from_event(http_event(raw), 0);

        let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
            panic!("expected an HTTP source");
        };
        assert_eq!(request.body, raw);
    }

    #[test]
    fn keeps_every_value_of_a_repeated_response_header() {
        let result = event_result_from_frame(
            proto::Result {
                id: "event-1".to_string(),
                credit_grant: 1,
                outcome: Some(proto::result::Outcome::Http(proto::HttpResponse {
                    status: 200,
                    headers: HashMap::from([(
                        "set-cookie".to_string(),
                        proto::Values {
                            values: vec!["a=1".to_string(), "b=2".to_string()],
                        },
                    )]),
                    body: b"{}".to_vec(),
                })),
            },
            &EventType::HttpRequest,
        );

        let EventResultData::HttpResponse(response) = result.data else {
            panic!("expected an HTTP response");
        };
        assert_eq!(
            response.headers.get("set-cookie"),
            Some(&vec!["a=1".to_string(), "b=2".to_string()])
        );
    }

    #[test]
    fn carries_an_event_trigger_as_a_batch_of_one() {
        let event = EventData {
            id: "event-1".to_string(),
            event_type: EventType::EventMessage,
            handler_tag: "source::uploads::Process".to_string(),
            timestamp: 0,
            data: EventDataPayload::EventMessageEventData(EventMessageEventData {
                body: "{\"key\":\"a.png\"}".to_string(),
                source: "uploads-queue".to_string(),
                message_id: Some("message-1".to_string()),
                message_attributes: None,
                vendor: json!({"provider": "aws"}),
            }),
            trace_context: None,
        };

        let Some(proto::dispatch::Source::Consumer(batch)) = dispatch_from_event(event, 0).source
        else {
            panic!("expected a consumer source");
        };
        assert_eq!(batch.source_id, "uploads-queue");
        assert_eq!(batch.records.len(), 1);
        assert_eq!(batch.records[0].message_id, "message-1");
        assert_eq!(batch.records[0].body, b"{\"key\":\"a.png\"}");
    }

    #[test]
    fn carries_a_consumer_batch_with_every_record() {
        let event = EventData {
            id: "event-1".to_string(),
            event_type: EventType::ConsumerMessage,
            handler_tag: "source::orders::Process".to_string(),
            timestamp: 0,
            data: EventDataPayload::ConsumerMessageEventData(ConsumerEventData {
                messages: vec![
                    ConsumerMessage {
                        message_id: "message-1".to_string(),
                        body: "first".to_string(),
                        source: "orders-queue".to_string(),
                        source_type: Some("queue".to_string()),
                        source_name: None,
                        event_type: None,
                        event_name: None,
                        message_attributes: json!({}),
                        vendor: json!({}),
                    },
                    ConsumerMessage {
                        message_id: "message-2".to_string(),
                        body: "second".to_string(),
                        source: "orders-queue".to_string(),
                        source_type: Some("queue".to_string()),
                        source_name: None,
                        event_type: None,
                        event_name: None,
                        message_attributes: json!({}),
                        vendor: json!({}),
                    },
                ],
                vendor: json!({}),
            }),
            trace_context: None,
        };

        let Some(proto::dispatch::Source::Consumer(batch)) = dispatch_from_event(event, 0).source
        else {
            panic!("expected a consumer source");
        };
        assert_eq!(batch.records.len(), 2);
        assert_eq!(batch.source_id, "orders-queue");
        assert_eq!(batch.source_type, "queue");
    }

    #[test]
    fn returns_an_http_response_to_the_waiting_caller() {
        let result = event_result_from_frame(
            proto::Result {
                id: "event-1".to_string(),
                credit_grant: 1,
                outcome: Some(proto::result::Outcome::Http(proto::HttpResponse {
                    status: 201,
                    headers: HashMap::from([(
                        "content-type".to_string(),
                        proto::Values {
                            values: vec!["application/json".to_string()],
                        },
                    )]),
                    body: b"{\"id\":1}".to_vec(),
                })),
            },
            &EventType::HttpRequest,
        );

        assert_eq!(result.event_id, "event-1");
        let EventResultData::HttpResponse(response) = result.data else {
            panic!("expected an HTTP response");
        };
        assert_eq!(response.status, 201);
        assert_eq!(response.body, &b"{\"id\":1}"[..]);
        assert_eq!(
            response.headers.get("content-type"),
            Some(&vec!["application/json".to_string()])
        );
    }

    #[test]
    fn reports_partial_batch_failures() {
        let result = event_result_from_frame(
            proto::Result {
                id: "event-1".to_string(),
                credit_grant: 1,
                outcome: Some(proto::result::Outcome::Consumer(proto::BatchResult {
                    success: false,
                    failures: vec![proto::RecordFailure {
                        message_id: "message-2".to_string(),
                        error_message: "downstream rejected it".to_string(),
                    }],
                })),
            },
            &EventType::ConsumerMessage,
        );

        let EventResultData::MessageProcessingResponse(response) = result.data else {
            panic!("expected a message processing response");
        };
        assert!(!response.success);
        let failures = response.failures.expect("the failure should be reported");
        assert_eq!(failures.len(), 1);
        assert_eq!(failures[0].message_id, "message-2");
    }

    #[test]
    fn treats_a_result_with_no_outcome_as_a_failure() {
        let result = event_result_from_frame(
            proto::Result {
                id: "event-1".to_string(),
                credit_grant: 1,
                outcome: None,
            },
            &EventType::HttpRequest,
        );

        let EventResultData::HttpResponse(response) = result.data else {
            panic!("expected a synthesised failure response");
        };
        assert_eq!(response.status, 500);
    }

    fn handler_error(waiting_for: &EventType) -> EventResultData {
        event_result_from_frame(
            proto::Result {
                id: "event-1".to_string(),
                credit_grant: 1,
                outcome: Some(proto::result::Outcome::Error(proto::HandlerError {
                    message: "connection reset".to_string(),
                    r#type: "IOError".to_string(),
                    stack: String::new(),
                })),
            },
            waiting_for,
        )
        .data
    }

    #[test]
    fn refuses_a_status_that_cannot_be_one_rather_than_wrapping_it() {
        assert_eq!(status_code(201), 201);
        // Wraps to 200 when truncated, so a handler sending nonsense would be
        // reported as having succeeded.
        assert_eq!(status_code(65_736), 500);
        assert_eq!(status_code(u32::MAX), 500);
        // Fits in the narrower type but is still not a status. Left alone here
        // and caught again when the response is built.
        assert_eq!(status_code(4_464), 4_464);
    }

    /// A stack trace names the files, line numbers and internal frames of the
    /// process serving a request, and what the caller is told reaches whoever
    /// asked. So the stack belongs in the log and nowhere the caller can see.
    #[test]
    fn a_stack_trace_never_reaches_whoever_was_waiting() {
        const STACK: &str = "at handlers/orders.ts:42:9\nat /srv/app/node_modules/inner.js:7:1";

        let with_stack = |waiting_for: &EventType| {
            event_result_from_frame(
                proto::Result {
                    id: "event-1".to_string(),
                    credit_grant: 1,
                    outcome: Some(proto::result::Outcome::Error(proto::HandlerError {
                        message: "connection reset".to_string(),
                        r#type: "IOError".to_string(),
                        stack: STACK.to_string(),
                    })),
                },
                waiting_for,
            )
            .data
        };

        for waiting_for in [
            EventType::HttpRequest,
            EventType::WsMessage,
            EventType::ScheduleMessage,
            EventType::ConsumerMessage,
            EventType::EventMessage,
            EventType::CustomInvoke,
        ] {
            let told = format!("{:?}", with_stack(&waiting_for));
            assert!(
                !told.contains("orders.ts"),
                "a {waiting_for:?} caller was told the stack: {told}"
            );
            assert!(
                !told.contains("node_modules"),
                "a {waiting_for:?} caller was told the stack: {told}"
            );
        }

        // The message itself still reaches a caller that is not an HTTP
        // client, so the assertions above are not passing on an empty answer.
        let EventResultData::WebSocketResponse(ws) = with_stack(&EventType::WsMessage) else {
            panic!("expected a WebSocket response");
        };
        assert_eq!(
            ws.error_message.as_deref(),
            Some("IOError: connection reset")
        );
    }

    /// What went wrong inside a handler names types, libraries and internals
    /// of a process the client only reaches across a network. The request id
    /// on the response is what ties their report to the log that has it.
    #[test]
    fn a_client_is_not_told_what_went_wrong_inside_a_handler() {
        let EventResultData::HttpResponse(http) = handler_error(&EventType::HttpRequest) else {
            panic!("expected an HTTP response");
        };

        assert_eq!(http.status, 500);
        let body = String::from_utf8(http.body.to_vec()).expect("the body should be text");
        assert!(
            !body.contains("connection reset"),
            "told the client: {body}"
        );
        assert!(!body.contains("IOError"), "told the client: {body}");
        assert_eq!(body, UNEXPECTED_ERROR_BODY);
    }

    #[test]
    fn reports_a_handler_error_in_the_shape_the_caller_is_waiting_for() {
        // Answering every caller with an HTTP response means a WebSocket or a
        // custom invocation sees a result it cannot read, and reports that
        // instead of what actually went wrong.
        let EventResultData::WebSocketResponse(ws) = handler_error(&EventType::WsMessage) else {
            panic!("expected a WebSocket response");
        };
        assert!(!ws.success);
        assert_eq!(
            ws.error_message.as_deref(),
            Some("IOError: connection reset")
        );

        let EventResultData::CustomInvokeResponse(custom) = handler_error(&EventType::CustomInvoke)
        else {
            panic!("expected a custom invocation response");
        };
        assert_eq!(
            custom.error_message.as_deref(),
            Some("IOError: connection reset")
        );

        let EventResultData::ScheduledEventResponse(schedule) =
            handler_error(&EventType::ScheduleMessage)
        else {
            panic!("expected a scheduled event response");
        };
        assert!(!schedule.success);

        let EventResultData::MessageProcessingResponse(batch) =
            handler_error(&EventType::ConsumerMessage)
        else {
            panic!("expected a message processing response");
        };
        assert!(!batch.success);

        // The HTTP client is told nothing of what went wrong, see
        // `a_client_is_not_told_what_went_wrong_inside_a_handler`.
        let EventResultData::HttpResponse(http) = handler_error(&EventType::HttpRequest) else {
            panic!("expected an HTTP response");
        };
        assert_eq!(http.status, 500);
    }
}
