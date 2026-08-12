//! HTTP request handling for the IPC runtime call mode.
//!
//! In the FFI call mode the SDK registers a route per handler through
//! `Application::register_http_handler`, and the handler runs in-process. In
//! the IPC call mode there is no in-process handler to register: the handlers
//! live in a separate executable. This module provides the equivalent route
//! registration for that mode, where each route turns a request into an event,
//! puts it on the event queue and waits for the handlers executable to return
//! a result.
//!
//! The route side of this is independent of how events reach the handlers.
//! Only the draining of the queue changes when the gRPC stream replaces the
//! polling local runtime API.

use std::{collections::HashMap, time::Duration};

use axum::{
    body::{to_bytes, Body, Bytes},
    extract::{RawPathParams, Request},
    http::{header::RETRY_AFTER, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use http_body_util::LengthLimitError;
use tracing::{error, warn};

use crate::{
    consts::MAX_HTTP_REQUEST_BODY_BYTES,
    event_queue::{admission_wait, EventQueue, EventQueueError},
    request::{RequestId, ResolvedClientIp},
    types::{
        EventData, EventDataPayload, EventResultData, EventType, HttpRequestEventData,
        HttpResponseData,
    },
};

/// What a registered IPC route needs to turn a request into an event.
#[derive(Clone)]
pub struct IpcHttpRoute {
    pub event_queue: EventQueue,
    /// The tag identifying the handler that should run, as it appears on the
    /// dispatched event.
    pub handler_tag: String,
    /// The blueprint route template, e.g. `/orders/{orderId}`, as distinct from
    /// the concrete path of any one request.
    pub route: String,
    /// The handler's configured timeout. Known when the route is registered, so
    /// it is carried here rather than looked up per request.
    pub timeout: Duration,
}

/// Turns an HTTP request into an event, waits for a handler to process it and
/// renders the result as a response.
pub async fn handle_request(
    route: IpcHttpRoute,
    path_params: RawPathParams,
    request: Request,
) -> Response {
    let method = request.method().as_str().to_lowercase();
    let path = request.uri().path().to_string();
    let query = request.uri().query().unwrap_or_default().to_string();

    let request_id = request
        .extensions()
        .get::<RequestId>()
        .map(|id| id.0.clone())
        .unwrap_or_default();
    let source_ip = request
        .extensions()
        .get::<ResolvedClientIp>()
        .map(|ip| ip.0.to_string())
        .unwrap_or_default();

    let (single_headers, multi_headers) = collect_headers(request.headers());
    let path_params = collect_path_params(&path_params);
    let (query_params, multi_query_params) = collect_query_params(&query);

    let (body, is_binary) = match to_bytes(request.into_body(), MAX_HTTP_REQUEST_BODY_BYTES).await {
        Ok(bytes) if bytes.is_empty() => (None, false),
        Ok(bytes) => encode_body(bytes),
        Err(err) if is_length_limit(&err) => {
            warn!(
                limit = MAX_HTTP_REQUEST_BODY_BYTES,
                "rejecting request with an oversized body"
            );
            return (
                StatusCode::PAYLOAD_TOO_LARGE,
                "the request body is larger than the runtime accepts",
            )
                .into_response();
        }
        Err(err) => {
            warn!("failed to read request body: {err}");
            return (StatusCode::BAD_REQUEST, "failed to read request body").into_response();
        }
    };

    let event = EventData {
        id: nanoid::nanoid!(),
        event_type: EventType::HttpRequest,
        handler_tag: route.handler_tag.clone(),
        timestamp: std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs(),
        data: EventDataPayload::HttpRequestEventData(Box::new(HttpRequestEventData {
            method,
            path,
            route: route.route.clone(),
            path_params,
            query_params,
            multi_query_params,
            headers: single_headers,
            multi_headers,
            body,
            is_binary,
            source_ip,
            request_id,
        })),
    };

    dispatch(&route, event).await
}

/// Puts the event on the queue and waits for its result, bounded by the
/// handler's configured timeout.
async fn dispatch(route: &IpcHttpRoute, event: EventData) -> Response {
    let timeout = route.timeout;
    // Waiting for queue capacity spends the same budget the handler needs, so
    // both waits are anchored to a deadline taken before admission.
    let deadline = tokio::time::Instant::now() + timeout;

    let result_rx = match route
        .event_queue
        .enqueue(event, admission_wait(timeout))
        .await
    {
        Ok(rx) => rx,
        Err(EventQueueError::QueueFull) => {
            warn!(
                handler_tag = %route.handler_tag,
                "shedding request, no event queue capacity became available"
            );
            return queue_full_response();
        }
        Err(EventQueueError::Closed) => {
            error!(
                handler_tag = %route.handler_tag,
                "event queue is closed, no handlers executable is attached"
            );
            return (StatusCode::BAD_GATEWAY, "handlers are not available").into_response();
        }
    };

    match tokio::time::timeout_at(deadline, result_rx).await {
        Ok(Ok((_event, result))) => render_result(route, result.data),
        // The sender was dropped, which happens when the cleanup task removes
        // the in-flight entry after its deadline passed, or when the handlers
        // executable went away mid-request.
        Ok(Err(_)) => {
            warn!(
                handler_tag = %route.handler_tag,
                "handler did not return a result"
            );
            (StatusCode::BAD_GATEWAY, "handler did not return a result").into_response()
        }
        Err(_) => {
            warn!(
                handler_tag = %route.handler_tag,
                ?timeout,
                "handler did not respond within its timeout"
            );
            (StatusCode::GATEWAY_TIMEOUT, "handler timed out").into_response()
        }
    }
}

/// Carries the body without losing anything.
///
/// A text body travels as sent. One that is not valid UTF-8 is base64 encoded
/// rather than being forced through a lossy conversion, which would replace
/// every invalid sequence with the replacement character and silently corrupt
/// any payload that is not text, such as an upload or a protobuf request.
///
/// The event is serialised as JSON, which cannot represent raw bytes, so an
/// encoding is unavoidable here. It disappears when bodies become `bytes`.
fn encode_body(bytes: Bytes) -> (Option<String>, bool) {
    match String::from_utf8(bytes.into()) {
        Ok(text) => (Some(text), false),
        Err(err) => (Some(BASE64.encode(err.as_bytes())), true),
    }
}

/// Whether reading the body failed because it exceeded the limit, as opposed to
/// the connection breaking partway through.
fn is_length_limit(err: &axum::Error) -> bool {
    let mut source: Option<&(dyn std::error::Error + 'static)> = Some(err);
    while let Some(err) = source {
        if err.downcast_ref::<LengthLimitError>().is_some() {
            return true;
        }
        source = err.source();
    }
    false
}

/// A shed request is a capacity signal rather than a fault, so it is reported
/// as such and marked retryable. Rendering it as a 500 would make overload look
/// like a bug in the application.
fn queue_full_response() -> Response {
    (
        StatusCode::SERVICE_UNAVAILABLE,
        [(RETRY_AFTER, "1")],
        "the runtime is at capacity, retry shortly",
    )
        .into_response()
}

fn render_result(route: &IpcHttpRoute, data: EventResultData) -> Response {
    let EventResultData::HttpResponse(response) = data else {
        error!(
            handler_tag = %route.handler_tag,
            "handler returned a result that is not an HTTP response"
        );
        return (
            StatusCode::INTERNAL_SERVER_ERROR,
            "handler returned an unexpected result",
        )
            .into_response();
    };
    build_response(response)
}

fn build_response(response: HttpResponseData) -> Response {
    let status = StatusCode::from_u16(response.status).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);

    let mut builder = Response::builder().status(status);
    for (name, value) in response.headers {
        builder = builder.header(name, value);
    }

    builder
        .body(Body::from(response.body))
        .unwrap_or_else(|err| {
            error!("failed to build response from handler result: {err}");
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                "failed to build a response",
            )
                .into_response()
        })
}

/// Collects request headers into the single and multi valued maps the event
/// carries.
///
/// Header names are lowercased, which HTTP/2 requires on the wire and which
/// makes lookup unambiguous for handlers regardless of how a client cased them.
fn collect_headers(headers: &HeaderMap) -> (HashMap<String, String>, HashMap<String, Vec<String>>) {
    let mut single = HashMap::new();
    let mut multi: HashMap<String, Vec<String>> = HashMap::new();

    for (name, value) in headers {
        let Ok(value) = value.to_str() else {
            // A header whose bytes are not valid UTF-8 cannot be represented in
            // the current string-typed event, so it is dropped rather than
            // corrupted.
            warn!(header = %name, "skipping header with a non UTF-8 value");
            continue;
        };
        let name = name.as_str().to_lowercase();
        single
            .entry(name.clone())
            .or_insert_with(|| value.to_string());
        multi.entry(name).or_default().push(value.to_string());
    }

    (single, multi)
}

fn collect_path_params(params: &RawPathParams) -> HashMap<String, String> {
    params
        .iter()
        .map(|(name, value)| (name.to_string(), value.to_string()))
        .collect()
}

/// Parses the query string into the single and multi valued maps.
///
/// The single valued map keeps the first occurrence of a name, so that
/// `?tag=a&tag=b` yields `a` there and both values in the multi valued map.
fn collect_query_params(query: &str) -> (HashMap<String, String>, HashMap<String, Vec<String>>) {
    let mut single = HashMap::new();
    let mut multi: HashMap<String, Vec<String>> = HashMap::new();

    for (name, value) in form_urlencoded::parse(query.as_bytes()) {
        let (name, value) = (name.into_owned(), value.into_owned());
        single.entry(name.clone()).or_insert_with(|| value.clone());
        multi.entry(name).or_default().push(value);
    }

    (single, multi)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn collects_repeated_query_params_into_both_maps() {
        let (single, multi) = collect_query_params("tag=a&tag=b&page=2");

        assert_eq!(single.get("tag"), Some(&"a".to_string()));
        assert_eq!(single.get("page"), Some(&"2".to_string()));
        assert_eq!(
            multi.get("tag"),
            Some(&vec!["a".to_string(), "b".to_string()])
        );
        assert_eq!(multi.get("page"), Some(&vec!["2".to_string()]));
    }

    #[test]
    fn collects_percent_decoded_query_params() {
        let (single, _) = collect_query_params("q=hello%20world&filter=a%26b");

        assert_eq!(single.get("q"), Some(&"hello world".to_string()));
        assert_eq!(single.get("filter"), Some(&"a&b".to_string()));
    }

    #[test]
    fn collects_empty_query_as_empty_maps() {
        let (single, multi) = collect_query_params("");

        assert!(single.is_empty());
        assert!(multi.is_empty());
    }

    #[test]
    fn lowercases_header_names_and_keeps_repeats() {
        let mut headers = HeaderMap::new();
        headers.append("Set-Cookie", "a=1".parse().unwrap());
        headers.append("set-cookie", "b=2".parse().unwrap());
        headers.insert("Content-Type", "application/json".parse().unwrap());

        let (single, multi) = collect_headers(&headers);

        assert_eq!(single.get("content-type"), Some(&"application/json".into()));
        assert_eq!(single.get("set-cookie"), Some(&"a=1".to_string()));
        assert_eq!(
            multi.get("set-cookie"),
            Some(&vec!["a=1".to_string(), "b=2".to_string()])
        );
    }

    #[test]
    fn builds_a_response_from_handler_result_data() {
        let response = build_response(HttpResponseData {
            status: 201,
            headers: HashMap::from([("content-type".to_string(), "application/json".to_string())]),
            body: r#"{"id":1}"#.to_string(),
        });

        assert_eq!(response.status(), StatusCode::CREATED);
        assert_eq!(
            response.headers().get("content-type").unwrap(),
            "application/json"
        );
    }

    #[test]
    fn falls_back_to_500_for_an_invalid_status_from_a_handler() {
        // 100..=999 is the valid range, so anything outside it is malformed
        // rather than merely unusual.
        let response = build_response(HttpResponseData {
            status: 1000,
            headers: HashMap::new(),
            body: String::new(),
        });

        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
    }

    #[test]
    fn carries_a_text_body_as_sent() {
        let (body, is_binary) = encode_body(Bytes::from_static(br#"{"id":1}"#));

        assert_eq!(body.as_deref(), Some(r#"{"id":1}"#));
        assert!(!is_binary);
    }

    #[test]
    fn carries_a_non_utf8_body_without_corrupting_it() {
        let raw = vec![0xff, 0xfe, 0x00, 0x80, 0x01];
        let (body, is_binary) = encode_body(Bytes::from(raw.clone()));

        assert!(is_binary);
        let decoded = BASE64
            .decode(body.expect("a body should be carried"))
            .expect("the body should be base64 encoded");
        assert_eq!(decoded, raw);
    }

    #[tokio::test]
    async fn an_oversized_body_is_reported_as_too_large_not_a_bad_request() {
        let err = to_bytes(Body::from(vec![0u8; 64]), 8).await.unwrap_err();
        assert!(is_length_limit(&err));
    }

    #[tokio::test]
    async fn a_body_read_failure_is_not_mistaken_for_an_oversized_one() {
        let err = to_bytes(
            Body::from_stream(futures::stream::once(async {
                Err::<Bytes, std::io::Error>(std::io::Error::other("connection reset"))
            })),
            1024,
        )
        .await
        .unwrap_err();
        assert!(!is_length_limit(&err));
    }

    #[test]
    fn sheds_with_503_and_a_retry_after_header() {
        let response = queue_full_response();

        assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);
        assert_eq!(response.headers().get(RETRY_AFTER).unwrap(), "1");
    }
}
