use std::time::Instant;

use crate::{
    aws_telemetry::XrayTraceId,
    config::{RuntimeConfig, RuntimePlatform},
    errors::ApplicationStartError,
    request::RequestId,
    types::ApiAppState,
};
use axum::{
    extract::{Request, State},
    http::StatusCode,
    middleware::Next,
    response::Response,
    Extension,
};
use axum_client_ip::SecureClientIp;
use axum_extra::{headers, TypedHeader};
use opentelemetry::{global, trace::TraceContextExt};
use opentelemetry_aws::trace::XrayPropagator as AwsXrayPropagator;
use opentelemetry_otlp::WithExportConfig;
use opentelemetry_sdk::trace::Config as TraceConfig;
use tracing::{info, level_filters::LevelFilter};
use tracing_opentelemetry::OpenTelemetrySpanExt;
use tracing_subscriber::{
    fmt::{self, format},
    prelude::__tracing_subscriber_SubscriberExt,
    util::SubscriberInitExt,
    EnvFilter, Layer,
};

/// Sets up tracing for the runtime.
/// This tracing configuration is only for the runtime, applications that provide handlers
/// should set up their own tracing and logging configuration native to the language of choice.
/// The various handler SDKs provide helpers for developers to more easily set up instrumentation
/// for their handlers.
pub fn setup_tracing(runtime_config: &RuntimeConfig) -> Result<(), ApplicationStartError> {
    if runtime_config.platform == RuntimePlatform::AWS {
        global::set_text_map_propagator(AwsXrayPropagator::new());
    }

    let trace_config = opentelemetry_sdk::trace::config()
        .with_sampler(opentelemetry_sdk::trace::Sampler::AlwaysOn);
    let trace_config = attach_id_generator(&runtime_config.platform, trace_config);

    let tracer = opentelemetry_otlp::new_pipeline()
        .tracing()
        .with_exporter(
            opentelemetry_otlp::new_exporter()
                .tonic()
                .with_endpoint(runtime_config.trace_otlp_collector_endpoint.clone()),
        )
        .with_trace_config(
            trace_config.with_resource(opentelemetry_sdk::Resource::new(vec![
                opentelemetry::KeyValue::new("service.name", runtime_config.service_name.clone()),
            ])),
        )
        .install_batch(opentelemetry_sdk::runtime::Tokio)?;

    let level_filter = LevelFilter::from_level(runtime_config.runtime_max_diagnostics_level);

    let otel_layer = tracing_opentelemetry::layer()
        .with_tracer(tracer)
        .with_filter(
            EnvFilter::from_default_env()
                .add_directive(level_filter.into())
                .add_directive("celerity_runtime_core".parse()?)
                .add_directive("tower_http=info".parse()?)
                .add_directive("hyper=info".parse()?)
                .add_directive("axum::rejection=trace".parse()?),
        )
        .with_filter(level_filter);

    let fmt_layer_prod = fmt::layer()
        .event_format(format().json().with_span_list(true))
        // Since we're using the JSON event formatter, we must also
        // use the JSON field formatter.
        .fmt_fields(format::JsonFields::default())
        .with_filter(level_filter);

    let fmt_layer_local = fmt::layer()
        .event_format(format().pretty())
        .with_filter(level_filter);

    if runtime_config.test_mode {
        // If we're in test mode, we don't want to initialize the subscriber
        // globally as it will fail tests due to the global trace subscriber
        // having already been registered.
        // To get around this, we set the default subscriber for the current
        // thread only.
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(fmt_layer_local)
            .set_default();
    } else if runtime_config.platform == RuntimePlatform::Local {
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(fmt_layer_local)
            .try_init()?;
    } else {
        tracing_subscriber::registry()
            .with(otel_layer)
            .with(fmt_layer_prod)
            .try_init()?;
    }

    Ok(())
}

fn attach_id_generator(platform: &RuntimePlatform, config: TraceConfig) -> TraceConfig {
    match platform {
        RuntimePlatform::AWS => {
            config.with_id_generator(opentelemetry_aws::trace::XrayIdGenerator::default())
        }
        _ => config.with_id_generator(opentelemetry_sdk::trace::RandomIdGenerator::default()),
    }
}

// A middleware function for enriching span context with values
// obtained from extractors.
pub async fn enrich_span(
    State(state): State<ApiAppState>,
    Extension(request_id): Extension<RequestId>,
    user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    secure_ip: SecureClientIp,
    request: Request,
    next: Next,
) -> Result<Response, StatusCode> {
    let client_ip = secure_ip;

    let span = tracing::Span::current();
    let span_context = span.context();
    let otel_span = span_context.span();
    let trace_id = otel_span.span_context().trace_id();

    let user_agent = match user_agent_header {
        Some(header) => header.to_string(),
        None => "Unknown User Agent".to_string(),
    };

    span.record("client_ip", format!("{:?}", client_ip.0));
    span.record("trace_id", trace_id.to_string());
    span.record("request_id", request_id.0.clone());
    // Connection ID is the same as the request ID but is associated with long-lived
    // connections like WebSockets.
    span.record("connection_id", request_id.0);
    span.record("user_agent", user_agent);

    if state.platform == RuntimePlatform::AWS {
        let xray_trace_id = XrayTraceId::from(trace_id);
        span.record("xray_trace_id", xray_trace_id.to_string());
    }

    let response = next.run(request).await;

    Ok(response)
}

// A middleware function for logging the entry and exit of a request,
// including logging the time taken to process the request.
// This won't be precise to the nanosecond or even microsecond
// as there will be middleware that runs after this, there shouldn't be
// any middleware that does heavy lifting after this yields a response
// so we can still be confident in the millisecond precision.
pub async fn log_request(request: Request, next: Next) -> Result<Response, StatusCode> {
    info!("HTTP request received");
    let start = Instant::now();
    let response = next.run(request).await;
    let duration = start.elapsed();
    let millis_precise = duration.as_micros() as f64 / 1000.0;
    info!(
        status_code = response.status().as_u16(),
        "HTTP request processed in {:?} milliseconds", millis_precise
    );
    Ok(response)
}
