use std::time::Instant;

use crate::{
    config::RuntimeConfig,
    errors::ApplicationStartError,
    request::{MatchedRoute, RequestId, ResolvedClientIp, ResolvedUserAgent},
    types::ApiAppState,
};
use axum::{
    extract::{MatchedPath, Request, State},
    http::StatusCode,
    middleware::Next,
    response::Response,
    Extension,
};
use axum_client_ip::ClientIp;
use axum_extra::{headers, TypedHeader};
use celerity_helpers::{aws_telemetry::XrayTraceId, runtime_types::RuntimePlatform};
use opentelemetry::{
    global,
    metrics::{Counter, Histogram, UpDownCounter},
    propagation::TextMapCompositePropagator,
    trace::TraceContextExt,
    KeyValue,
};
use opentelemetry_aws::trace::XrayPropagator as AwsXrayPropagator;
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::{propagation::TraceContextPropagator, trace::Config as TraceConfig};
use tracing::{info, level_filters::LevelFilter};
use tracing_opentelemetry::OpenTelemetrySpanExt;
use tracing_subscriber::{
    fmt::{self, format},
    prelude::__tracing_subscriber_SubscriberExt,
    util::SubscriberInitExt,
    EnvFilter, Layer,
};

// ---------------------------------------------------------------------------
// RuntimeMetrics — pre-created OTel metric instruments
// ---------------------------------------------------------------------------

/// Holds pre-created OpenTelemetry metric instruments for the runtime.
///
/// When `CELERITY_METRICS_ENABLED=true`, these are real instruments backed by
/// an OTLP exporter. When metrics are disabled, they are no-op instruments
/// from the global meter (which has no MeterProvider set).
#[derive(Clone)]
pub struct RuntimeMetrics {
    pub http_request_counter: Counter<u64>,
    pub http_request_duration: Histogram<f64>,
    pub ws_connections: UpDownCounter<i64>,
}

impl RuntimeMetrics {
    /// Creates metric instruments from the global meter.
    /// If no MeterProvider has been set (metrics disabled), these will be no-op.
    pub fn new() -> Self {
        let meter = global::meter("celerity_runtime");
        Self {
            http_request_counter: meter
                .u64_counter("http.server.request.count")
                .with_description("Total number of HTTP requests processed")
                .init(),
            http_request_duration: meter
                .f64_histogram("http.server.request.duration")
                .with_description("HTTP request duration in seconds")
                .with_unit(opentelemetry::metrics::Unit::new("s"))
                .init(),
            ws_connections: meter
                .i64_up_down_counter("ws.server.active_connections")
                .with_description("Number of active WebSocket connections")
                .init(),
        }
    }
}

impl Default for RuntimeMetrics {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tracing & metrics setup
// ---------------------------------------------------------------------------

/// Sets up logging and optionally OpenTelemetry tracing for the runtime.
///
/// Console logging (the fmt layer) is always enabled. The OTel tracing layer
/// is only added when `otel_enabled` is true (controlled by the blueprint's
/// `tracingEnabled` field on the API resource).
///
/// Format selection:
/// - Test mode: always pretty-printed
/// - CELERITY_LOG_FORMAT=json: JSON output regardless of platform
/// - CELERITY_LOG_FORMAT=pretty|human: pretty-printed regardless of platform
/// - Otherwise: Local platform -> pretty, all others -> JSON
pub fn setup_tracing(
    runtime_config: &RuntimeConfig,
    otel_enabled: bool,
) -> Result<(), ApplicationStartError> {
    let level_filter = LevelFilter::from_level(runtime_config.runtime_max_diagnostics_level);
    let use_pretty = if runtime_config.test_mode {
        true
    } else {
        match runtime_config.log_format.as_deref() {
            Some("json") => false,
            Some("pretty") | Some("human") => true,
            _ => runtime_config.platform == RuntimePlatform::Local,
        }
    };

    let otel_layer = if otel_enabled {
        let propagator = TextMapCompositePropagator::new(vec![
            Box::new(TraceContextPropagator::new()),
            Box::new(AwsXrayPropagator::new()),
        ]);
        global::set_text_map_propagator(propagator);

        let sampler = build_sampler(runtime_config.trace_sample_ratio);
        let trace_config = opentelemetry_sdk::trace::config().with_sampler(sampler);
        let trace_config = attach_id_generator(&runtime_config.platform, trace_config);

        let tracer =
            opentelemetry_otlp::new_pipeline()
                .tracing()
                .with_exporter(
                    opentelemetry_otlp::new_exporter()
                        .tonic()
                        .with_endpoint(runtime_config.trace_otlp_collector_endpoint.clone()),
                )
                .with_trace_config(trace_config.with_resource(opentelemetry_sdk::Resource::new(
                    vec![opentelemetry::KeyValue::new(
                        "service.name",
                        runtime_config.service_name.clone(),
                    )],
                )))
                .install_batch(opentelemetry_sdk::runtime::Tokio)?;

        Some(
            tracing_opentelemetry::layer()
                .with_tracer(tracer)
                .with_filter(
                    EnvFilter::from_default_env()
                        .add_directive(level_filter.into())
                        .add_directive("celerity_runtime_core".parse()?)
                        .add_directive("tower_http=info".parse()?)
                        .add_directive("hyper=info".parse()?)
                        .add_directive("axum::rejection=trace".parse()?),
                )
                .with_filter(level_filter),
        )
    } else {
        None
    };

    if runtime_config.test_mode {
        // In test mode, use a thread-local subscriber to avoid conflicts
        // with the global subscriber across concurrent tests.
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(
                fmt::layer()
                    .event_format(format().pretty())
                    .with_filter(level_filter),
            )
            .set_default();
    } else if use_pretty {
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(
                fmt::layer()
                    .event_format(format().pretty())
                    .with_filter(level_filter),
            )
            .try_init()?;
    } else {
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(
                fmt::layer()
                    .event_format(format().json().with_span_list(true))
                    .fmt_fields(format::JsonFields::default())
                    .with_filter(level_filter),
            )
            .try_init()?;
    }

    Ok(())
}

/// Sets up the OpenTelemetry metrics pipeline with OTLP export.
///
/// Only called when `CELERITY_METRICS_ENABLED=true`. Uses the same OTLP
/// endpoint and service name as tracing.
pub fn setup_metrics(runtime_config: &RuntimeConfig) -> Result<(), ApplicationStartError> {
    let exporter = opentelemetry_otlp::new_exporter()
        .tonic()
        .with_endpoint(runtime_config.trace_otlp_collector_endpoint.clone());

    let meter_provider = opentelemetry_otlp::new_pipeline()
        .metrics(opentelemetry_sdk::runtime::Tokio)
        .with_exporter(exporter)
        .with_period(std::time::Duration::from_secs(30))
        .with_resource(opentelemetry_sdk::Resource::new(vec![
            opentelemetry::KeyValue::new("service.name", runtime_config.service_name.clone()),
        ]))
        .build()
        .map_err(|e| ApplicationStartError::OpenTelemetryMetrics(e.to_string()))?;

    global::set_meter_provider(meter_provider);
    Ok(())
}

/// Builds a sampler based on the configured trace sample ratio.
fn build_sampler(ratio: f64) -> opentelemetry_sdk::trace::Sampler {
    use opentelemetry_sdk::trace::Sampler;
    if ratio >= 1.0 {
        Sampler::AlwaysOn
    } else if ratio <= 0.0 {
        Sampler::AlwaysOff
    } else {
        // ParentBased ensures child spans inherit the parent's sampling decision.
        Sampler::ParentBased(Box::new(Sampler::TraceIdRatioBased(ratio)))
    }
}

fn attach_id_generator(platform: &RuntimePlatform, config: TraceConfig) -> TraceConfig {
    match platform {
        RuntimePlatform::AWS => {
            config.with_id_generator(opentelemetry_aws::trace::XrayIdGenerator::default())
        }
        _ => config.with_id_generator(opentelemetry_sdk::trace::RandomIdGenerator::default()),
    }
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// A middleware function for enriching span context with values
// obtained from extractors.
pub async fn enrich_span(
    State(state): State<ApiAppState>,
    Extension(request_id): Extension<RequestId>,
    user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    ClientIp(client_ip): ClientIp,
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    let span = tracing::Span::current();
    let span_context = span.context();
    let otel_span = span_context.span();
    let trace_id = otel_span.span_context().trace_id();

    let user_agent = match user_agent_header {
        Some(header) => header.to_string(),
        None => "Unknown User Agent".to_string(),
    };

    span.record("client_ip", format!("{client_ip:?}"));
    span.record("trace_id", trace_id.to_string());
    span.record("request_id", request_id.0.clone());
    // Connection ID is the same as the request ID but is associated with long-lived
    // connections like WebSockets.
    span.record("connection_id", request_id.0);
    span.record("user_agent", user_agent.as_str());

    if state.platform == RuntimePlatform::AWS {
        let xray_trace_id = XrayTraceId::from(trace_id);
        span.record("xray_trace_id", xray_trace_id.to_string());
    }

    // Insert resolved values as extensions for downstream handlers and SDK bindings.
    let matched_path = request.extensions().get::<MatchedPath>();
    let matched_route = matched_path.map(|mp| mp.as_str().to_string());
    if let Some(mp) = matched_path {
        let method = request.method().as_str().to_uppercase();
        if let Some(name) = state.handler_names.get(&(method, mp.as_str().to_string())) {
            span.record("handler_name", name.as_str());
        }
    }
    let mut request = request;
    request.extensions_mut().insert(ResolvedClientIp(client_ip));
    request
        .extensions_mut()
        .insert(ResolvedUserAgent(user_agent));
    if let Some(route) = matched_route {
        request.extensions_mut().insert(MatchedRoute(route));
    }

    let response = next.run(request).await;

    Ok(response)
}

// A middleware function for logging the entry and exit of a request,
// including logging the time taken to process the request and recording
// HTTP request metrics when metrics are enabled.
// This won't be precise to the nanosecond or even microsecond
// as there will be middleware that runs after this, there shouldn't be
// any middleware that does heavy lifting after this yields a response
// so we can still be confident in the millisecond precision.
pub async fn log_request(
    State(state): State<ApiAppState>,
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    info!("HTTP request received");
    let method = request.method().to_string();
    let matched_path = request
        .extensions()
        .get::<MatchedPath>()
        .map(|mp| mp.as_str().to_string())
        .unwrap_or_else(|| "unknown".to_string());

    let start = Instant::now();
    let response = next.run(request).await;
    let duration = start.elapsed();
    let millis_precise = duration.as_micros() as f64 / 1000.0;
    let status_code = response.status().as_u16();
    info!(
        status_code = status_code,
        "HTTP request processed in {:?} milliseconds", millis_precise
    );

    if let Some(metrics) = &state.metrics {
        let attrs = [
            KeyValue::new("http.request.method", method),
            KeyValue::new("http.route", matched_path),
            KeyValue::new("http.response.status_code", status_code.to_string()),
        ];
        metrics.http_request_counter.add(1, &attrs);
        metrics
            .http_request_duration
            .record(duration.as_secs_f64(), &attrs);
    }

    Ok(response)
}
