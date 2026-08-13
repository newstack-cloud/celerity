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

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use serde_json::Value;
use tracing::warn;

use crate::{
    ipc_proto as proto,
    types::{
        ConsumerEventData, EventData, EventDataPayload, EventMessageEventData, EventResult,
        EventResultData, HttpRequestEventData, HttpResponseData, MessageProcessingFailure,
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
        trace_context: HashMap::new(),
        source: Some(source_from_payload(event.data)),
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
    }
}

fn http_request(request: HttpRequestEventData) -> proto::HttpRequest {
    proto::HttpRequest {
        method: request.method.to_uppercase(),
        path: request.path,
        route: request.route,
        path_params: single_valued(request.path_params),
        query_params: multi_valued(request.query_params, request.multi_query_params),
        headers: multi_valued(request.headers, request.multi_headers),
        source_ip: request.source_ip,
        request_id: request.request_id,
        body: decode_body(request.body, request.is_binary),
    }
}

fn websocket_message(message: WebSocketEventData) -> proto::WebSocketMessage {
    proto::WebSocketMessage {
        route: message.route,
        connection_id: message.connection_id,
        source_ip: message.source_ip,
        request_id: message.request_id.unwrap_or_default(),
        message: decode_body(Some(message.message), message.is_binary),
        is_binary: message.is_binary,
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
pub fn event_result_from_frame(result: proto::Result) -> EventResult {
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
            EventResultData::EventResponse(SimpleResponseData {
                success: custom.error_message.is_empty(),
                error_message: none_if_empty(custom.error_message),
            })
        }
        Some(proto::result::Outcome::Error(error)) => {
            EventResultData::HttpResponse(handler_error_response(&error))
        }
        None => {
            warn!(
                event_id = %result.id,
                "handler returned a result with no outcome"
            );
            EventResultData::HttpResponse(HttpResponseData {
                status: 500,
                headers: HashMap::new(),
                body: "handler returned an empty result".to_string(),
            })
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
        status: response.status as u16,
        // The internal response type holds one value per header, so only the
        // first survives. Multi-valued responses arrive intact once that type
        // carries them too.
        headers: response
            .headers
            .into_iter()
            .filter_map(|(name, values)| {
                values.values.into_iter().next().map(|value| (name, value))
            })
            .collect(),
        body: String::from_utf8_lossy(&response.body).into_owned(),
    }
}

fn handler_error_response(error: &proto::HandlerError) -> HttpResponseData {
    HttpResponseData {
        status: 500,
        headers: HashMap::new(),
        body: error.message.clone(),
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

/// Recovers the bytes a body always was.
///
/// A body flagged as binary was base64 encoded to survive being carried as
/// JSON. Anything else is text, and its UTF-8 bytes are the body.
fn decode_body(body: Option<String>, is_binary: bool) -> Vec<u8> {
    let Some(body) = body else {
        return Vec::new();
    };
    if !is_binary {
        return body.into_bytes();
    }
    BASE64.decode(&body).unwrap_or_else(|err| {
        // Only reachable if something produced a body that claims to be binary
        // but is not valid base64, which would be a bug on the producing side.
        warn!("a body marked as binary was not valid base64: {err}");
        body.into_bytes()
    })
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

fn single_valued(values: HashMap<String, String>) -> HashMap<String, proto::Values> {
    values
        .into_iter()
        .map(|(name, value)| {
            (
                name,
                proto::Values {
                    values: vec![value],
                },
            )
        })
        .collect()
}

/// Prefers the multi-valued map, falling back to the single-valued one for any
/// name it does not cover.
///
/// The internal types carry both, and the two can disagree. The protocol has
/// one canonical multi-valued representation, so this is where the duplication
/// is resolved rather than being passed on to every SDK.
fn multi_valued(
    single: HashMap<String, String>,
    multi: HashMap<String, Vec<String>>,
) -> HashMap<String, proto::Values> {
    let mut out: HashMap<String, proto::Values> = multi
        .into_iter()
        .map(|(name, values)| (name, proto::Values { values }))
        .collect();

    for (name, value) in single {
        out.entry(name).or_insert(proto::Values {
            values: vec![value],
        });
    }
    out
}

fn none_if_empty(value: String) -> Option<String> {
    (!value.is_empty()).then_some(value)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;
    use crate::types::{ConsumerMessage, EventType};

    fn http_event(body: Option<String>, is_binary: bool) -> EventData {
        EventData {
            id: "event-1".to_string(),
            event_type: EventType::HttpRequest,
            handler_tag: "GET::/orders/{id}".to_string(),
            timestamp: 1_700_000_000,
            data: EventDataPayload::HttpRequestEventData(Box::new(HttpRequestEventData {
                method: "get".to_string(),
                path: "/orders/1".to_string(),
                route: "/orders/{id}".to_string(),
                path_params: HashMap::from([("id".to_string(), "1".to_string())]),
                query_params: HashMap::from([("expand".to_string(), "items".to_string())]),
                multi_query_params: HashMap::from([(
                    "expand".to_string(),
                    vec!["items".to_string(), "totals".to_string()],
                )]),
                headers: HashMap::from([("accept".to_string(), "application/json".to_string())]),
                multi_headers: HashMap::new(),
                body,
                is_binary,
                source_ip: "10.0.0.1".to_string(),
                request_id: "request-1".to_string(),
            })),
        }
    }

    #[test]
    fn carries_an_http_request_onto_the_wire() {
        let dispatch = dispatch_from_event(http_event(Some("{}".to_string()), false), 42);

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

    #[test]
    fn prefers_the_multi_valued_map_where_the_two_disagree() {
        let dispatch = dispatch_from_event(http_event(None, false), 0);
        let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
            panic!("expected an HTTP source");
        };

        // Both maps carry `expand`; the multi-valued one wins so no value is
        // lost, and the header only the single-valued map has still arrives.
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
    fn recovers_the_bytes_of_a_binary_body() {
        let raw = vec![0xff, 0xfe, 0x00, 0x80];
        let dispatch = dispatch_from_event(http_event(Some(BASE64.encode(&raw)), true), 0);

        let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
            panic!("expected an HTTP source");
        };
        assert_eq!(request.body, raw);
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
        let result = event_result_from_frame(proto::Result {
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
        });

        assert_eq!(result.event_id, "event-1");
        let EventResultData::HttpResponse(response) = result.data else {
            panic!("expected an HTTP response");
        };
        assert_eq!(response.status, 201);
        assert_eq!(response.body, "{\"id\":1}");
        assert_eq!(
            response.headers.get("content-type"),
            Some(&"application/json".to_string())
        );
    }

    #[test]
    fn reports_partial_batch_failures() {
        let result = event_result_from_frame(proto::Result {
            id: "event-1".to_string(),
            credit_grant: 1,
            outcome: Some(proto::result::Outcome::Consumer(proto::BatchResult {
                success: false,
                failures: vec![proto::RecordFailure {
                    message_id: "message-2".to_string(),
                    error_message: "downstream rejected it".to_string(),
                }],
            })),
        });

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
        let result = event_result_from_frame(proto::Result {
            id: "event-1".to_string(),
            credit_grant: 1,
            outcome: None,
        });

        let EventResultData::HttpResponse(response) = result.data else {
            panic!("expected a synthesised failure response");
        };
        assert_eq!(response.status, 500);
    }
}
