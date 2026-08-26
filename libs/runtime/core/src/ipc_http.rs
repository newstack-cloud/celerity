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
    body::{to_bytes, Body},
    extract::{RawPathParams, Request},
    http::{header::RETRY_AFTER, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
};
use http_body_util::LengthLimitError;
use tracing::{error, warn};

use crate::{
    consts::{
        MAX_HTTP_REQUEST_BODY_BYTES, NO_RESPONSE_BODY, REQUEST_TIMED_OUT_BODY,
        UNEXPECTED_ERROR_BODY,
    },
    event_queue::{admission_wait, EventQueue, EventQueueError},
    request::{RequestId, ResolvedClientIp},
    telemetry_utils::extract_trace_context,
    types::{
        EventData, EventDataPayload, EventOutcome, EventResultData, EventType,
        HttpRequestEventData, HttpResponseData,
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

    let headers = collect_headers(request.headers());
    let path_params = collect_path_params(&path_params, &route.route, &path);
    let query_params = collect_query_params(&query);

    let body = match to_bytes(request.into_body(), MAX_HTTP_REQUEST_BODY_BYTES).await {
        Ok(bytes) => bytes,
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
            headers,
            body,
            source_ip,
            request_id,
        })),
        trace_context: extract_trace_context(),
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
    let event_id = event.id.clone();

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
            return unavailable_response();
        }
        Err(EventQueueError::Closed) => {
            // Reached once the dispatcher has stopped, which is either a
            // runtime that is shutting down or one that never started
            // dispatching. Both are reasons to send the caller elsewhere
            // rather than to report a fault in the application.
            error!(
                handler_tag = %route.handler_tag,
                "event queue is closed, the runtime is not dispatching events"
            );
            return unavailable_response();
        }
    };

    // Armed until this function returns through one of the branches below.
    // Anything else that ends this future ends it by dropping it, which is the
    // only signal there is that nobody is waiting for the response any more.
    // A client disconnecting is the usual cause as hyper ends the connection task
    // and the response future goes with it.
    //
    // Without this the handler keeps a worker slot, and whatever it called
    // downstream, producing a response that will be discarded. That matters
    // under load, because a caller that gave up usually retries, so the
    // abandoned work and the retry pile up together.
    let cancel_on_caller_gone = route.event_queue.cancel_on_drop(event_id);

    let response = match tokio::time::timeout_at(deadline, result_rx).await {
        Ok(Ok(EventOutcome::Completed(_event, result))) => render_result(route, result.data),
        Ok(Ok(EventOutcome::Unservable(reason))) => {
            warn!(
                handler_tag = %route.handler_tag,
                %reason,
                "shedding request, the runtime will not dispatch it"
            );
            unavailable_response()
        }
        // The sender was dropped, which happens when the cleanup task removes
        // the in-flight entry after its deadline passed, or when the handlers
        // executable went away mid-request. Which of the two is not knowable
        // here, so the client is told neither.
        Ok(Err(_)) => {
            warn!(
                handler_tag = %route.handler_tag,
                "handler did not return a result"
            );
            (StatusCode::BAD_GATEWAY, NO_RESPONSE_BODY).into_response()
        }
        Err(_) => {
            warn!(
                handler_tag = %route.handler_tag,
                ?timeout,
                "handler did not respond within its timeout"
            );
            (StatusCode::GATEWAY_TIMEOUT, REQUEST_TIMED_OUT_BODY).into_response()
        }
    };

    // Every branch above is a reason the runtime already knows about, so none
    // of them should be reported as the caller going away.
    cancel_on_caller_gone.disarm();
    response
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

/// A shed request is a capacity or availability signal rather than a fault, so
/// it is reported as such and marked retryable. Rendering it as a 500 would
/// make overload, or handlers that have not started yet, look like a bug in the
/// application.
fn unavailable_response() -> Response {
    (
        StatusCode::SERVICE_UNAVAILABLE,
        [(RETRY_AFTER, "1")],
        "the runtime cannot serve this request at the moment, retry shortly",
    )
        .into_response()
}

fn render_result(route: &IpcHttpRoute, data: EventResultData) -> Response {
    let EventResultData::HttpResponse(response) = data else {
        error!(
            handler_tag = %route.handler_tag,
            "handler returned a result that is not an HTTP response"
        );
        return (StatusCode::INTERNAL_SERVER_ERROR, UNEXPECTED_ERROR_BODY).into_response();
    };
    build_response(response)
}

fn build_response(response: HttpResponseData) -> Response {
    let status = StatusCode::from_u16(response.status).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);

    let mut builder = Response::builder().status(status);
    for (name, values) in response.headers {
        // Appended one at a time rather than folded together, so two
        // `Set-Cookie` headers stay two headers.
        for value in values {
            builder = builder.header(&name, value);
        }
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

/// Collects request headers, keeping every value a name was sent with.
///
/// Header names are lowercased, which HTTP/2 requires on the wire and which
/// makes lookup unambiguous for handlers regardless of how a client cased them.
fn collect_headers(headers: &HeaderMap) -> HashMap<String, Vec<String>> {
    let mut collected: HashMap<String, Vec<String>> = HashMap::new();

    for (name, value) in headers {
        let Ok(value) = value.to_str() else {
            // A header whose bytes are not valid UTF-8 cannot be represented in
            // the string-typed event, so it is dropped rather than corrupted.
            warn!(header = %name, "skipping header with a non UTF-8 value");
            continue;
        };
        collected
            .entry(name.as_str().to_lowercase())
            .or_default()
            .push(value.to_string());
    }

    collected
}

/// Collects path parameters, splitting a catch-all into one value per segment.
///
/// An ordinary parameter arrives already decoded from the extractor and is
/// taken as it is; decoding it again would turn a literal `%20` a client took
/// care to escape into a space.
///
/// A catch-all cannot be taken from the extractor at all. It matches the whole
/// remaining path, and by the time the extractor has decoded it an encoded
/// separator is indistinguishable from a real one, so `a%2Fb` would be torn
/// into two segments. The segments are therefore recovered from the request's
/// own path, which is still encoded, and decoded one at a time afterwards.
fn collect_path_params(
    params: &RawPathParams,
    route: &str,
    raw_path: &str,
) -> HashMap<String, Vec<String>> {
    params
        .iter()
        .map(|(name, value)| {
            let values = if is_catch_all(route, name) {
                catch_all_segments(route, name, raw_path)
            } else {
                vec![value.to_string()]
            };
            (name.to_string(), values)
        })
        .collect()
}

/// The segments a catch-all matched, taken from the still-encoded request path.
///
/// A catch-all is always the last thing in a route, so everything after the
/// segments the template spells out belongs to it.
fn catch_all_segments(route: &str, name: &str, raw_path: &str) -> Vec<String> {
    let placeholder = format!("{{*{name}}}");
    let consumed_by_template = route
        .trim_start_matches('/')
        .split('/')
        .take_while(|segment| *segment != placeholder)
        .count();

    raw_path
        .trim_start_matches('/')
        .split('/')
        .skip(consumed_by_template)
        .map(percent_decode)
        .collect()
}

/// Whether a parameter is declared as a catch-all.
///
/// Read from the route template rather than from the matched value, since a
/// catch-all that happened to match a single segment is indistinguishable from
/// an ordinary parameter by its value alone.
///
/// The template is the router's own form, `{*name}`, not the blueprint's
/// `{name+}`, because the blueprint path is normalised for the router when the
/// configuration is transformed and that normalised form is what the runtime
/// carries from then on, handler tags included.
fn is_catch_all(route: &str, name: &str) -> bool {
    route.contains(&format!("{{*{name}}}"))
}

/// Percent-decodes one path segment.
///
/// Deliberately not the query string decoder, which reads `&` and `=` as
/// separators and `+` as a space. All three are legal literals in a path
/// segment, so decoding one that way would silently drop or alter them.
fn percent_decode(value: &str) -> String {
    percent_encoding::percent_decode_str(value)
        .decode_utf8_lossy()
        .into_owned()
}

/// Parses the query string, keeping every value a name was sent with, so that
/// `?tag=a&tag=b` yields both.
fn collect_query_params(query: &str) -> HashMap<String, Vec<String>> {
    let mut collected: HashMap<String, Vec<String>> = HashMap::new();

    for (name, value) in form_urlencoded::parse(query.as_bytes()) {
        collected
            .entry(name.into_owned())
            .or_default()
            .push(value.into_owned());
    }

    collected
}

#[cfg(test)]
mod tests {
    use axum::body::Bytes;

    use super::*;

    #[test]
    fn keeps_every_value_of_a_repeated_query_param() {
        let collected = collect_query_params("tag=a&tag=b&page=2");

        assert_eq!(
            collected.get("tag"),
            Some(&vec!["a".to_string(), "b".to_string()])
        );
        assert_eq!(collected.get("page"), Some(&vec!["2".to_string()]));
    }

    #[test]
    fn collects_percent_decoded_query_params() {
        let collected = collect_query_params("q=hello%20world&filter=a%26b");

        assert_eq!(collected.get("q"), Some(&vec!["hello world".to_string()]));
        assert_eq!(collected.get("filter"), Some(&vec!["a&b".to_string()]));
    }

    #[test]
    fn collects_empty_query_as_an_empty_map() {
        assert!(collect_query_params("").is_empty());
    }

    #[test]
    fn lowercases_header_names_and_keeps_repeats() {
        let mut headers = HeaderMap::new();
        headers.append("Set-Cookie", "a=1".parse().unwrap());
        headers.append("set-cookie", "b=2".parse().unwrap());
        headers.insert("Content-Type", "application/json".parse().unwrap());

        let collected = collect_headers(&headers);

        assert_eq!(
            collected.get("content-type"),
            Some(&vec!["application/json".to_string()])
        );
        assert_eq!(
            collected.get("set-cookie"),
            Some(&vec!["a=1".to_string(), "b=2".to_string()])
        );
    }

    #[test]
    fn builds_a_response_from_handler_result_data() {
        let response = build_response(HttpResponseData {
            status: 201,
            headers: HashMap::from([(
                "content-type".to_string(),
                vec!["application/json".to_string()],
            )]),
            body: Bytes::from_static(br#"{"id":1}"#),
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
            body: Bytes::new(),
        });

        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
    }

    #[test]
    fn emits_a_repeated_response_header_once_per_value() {
        let response = build_response(HttpResponseData {
            status: 200,
            headers: HashMap::from([(
                "set-cookie".to_string(),
                vec!["a=1".to_string(), "b=2".to_string()],
            )]),
            body: Bytes::new(),
        });

        // Two headers rather than one folded value, which RFC 9110 forbids for
        // Set-Cookie.
        let cookies: Vec<_> = response.headers().get_all("set-cookie").iter().collect();
        assert_eq!(cookies, vec!["a=1", "b=2"]);
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
    fn takes_catch_all_segments_from_the_still_encoded_path() {
        // The middle segment carries an encoded separator, which belongs inside
        // that segment rather than splitting it.
        assert_eq!(
            catch_all_segments(
                "/files/{*filePath}",
                "filePath",
                "/files/docs/a%2Fb/report%20final.pdf"
            ),
            vec!["docs", "a/b", "report final.pdf"]
        );
        // Segments the template spells out are not part of the catch-all.
        assert_eq!(
            catch_all_segments("/files/{bucket}/{*path}", "path", "/files/main/a/b"),
            vec!["a", "b"]
        );
    }

    #[test]
    fn decodes_a_percent_encoded_path_parameter() {
        assert_eq!(percent_decode("order%20one"), "order one");
        assert_eq!(percent_decode("plain"), "plain");
        // Characters the query string decoder would treat as syntax are
        // ordinary literals in a path segment and must survive.
        assert_eq!(percent_decode("a+b"), "a+b");
        assert_eq!(percent_decode("a=b"), "a=b");
        assert_eq!(percent_decode("a&b"), "a&b");
        assert_eq!(percent_decode("k=v&x=y"), "k=v&x=y");
    }

    #[test]
    fn reads_catch_all_parameters_from_the_route_template() {
        assert!(is_catch_all("/files/{*path}", "path"));
        assert!(!is_catch_all("/orders/{orderId}", "orderId"));
        // A catch-all elsewhere in the route does not make this one a catch-all.
        assert!(!is_catch_all("/files/{bucket}/{*path}", "bucket"));
    }

    #[test]
    fn sheds_with_503_and_a_retry_after_header() {
        let response = unavailable_response();

        assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);
        assert_eq!(response.headers().get(RETRY_AFTER).unwrap(), "1");
    }
}
