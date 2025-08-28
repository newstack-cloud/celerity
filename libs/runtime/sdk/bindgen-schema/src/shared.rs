use oo_bindgen::model::*;

pub(crate) struct SharedDefinitions {
    pub general_error_type: ErrorType<Unvalidated>,
}

pub(crate) fn define(lib: &mut LibraryBuilder) -> BackTraced<SharedDefinitions> {
    let general_error_type = lib
        .define_error_type(
            "general_error",
            "general_exception",
            ExceptionType::UncheckedException,
        )?
        .add_error(
            "invalid_timeout",
            "The supplied timeout value is too small or too large",
        )?
        .add_error("null_parameter", "Null parameter")?
        .add_error(
            "application_not_initialised",
            "The application has not been initialised",
        )?
        .add_error(
            "no_support",
            "Native library was compiled without support for this feature",
        )?
        .add_error(
            "application_start_environment_error",
            "Error starting the application due to an environment issue",
        )?
        .add_error(
            "application_start_blueprint_parse_error",
            "Error starting the application due to failure to parse the configured blueprint",
        )?
        .add_error(
            "application_start_config_api_error",
            "Error starting the application due to invalid API configuration",
        )?
        .add_error(
            "application_start_config_api_missing",
            "Error starting the application due to a missing API configuration",
        )?
        .add_error(
            "application_start_task_wait_error",
            "Error starting the application due to a task failing or other runtime error",
        )?
        .add_error(
            "application_start_open_telemetry_trace_error",
            "Error starting the application due to failure in setting up tracing",
        )?
        .add_error(
            "application_start_tracer_try_init_error",
            "Error starting the application due to failure initialisating runtime tracer",
        )?
        .add_error(
            "application_start_tracing_filter_parse_error",
            "Error starting the application due to failure parsing tracing directives for the runtime",
        )?
        .add_error("runtime_creation_failure", "Failed to create Tokio runtime")?
        .add_error("runtime_destroyed", "Runtime has already been disposed")?
        .add_error(
            "runtime_cannot_block_within_async",
            "Runtime cannot execute blocking call within asynchronous context",
        )?
        .add_error(
            "application_start_http_client_error", 
            "Error starting the application due to failure in creating the HTTP client for the resource store"
        )?
        .doc("General error type used throughout the library")?
        .build()?;

    Ok(SharedDefinitions { general_error_type })
}
