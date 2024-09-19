use oo_bindgen::model::*;

pub(crate) fn define(
    lib: &mut LibraryBuilder,
    response: ClassDeclarationHandle,
) -> BackTraced<AsynchronousInterface> {
    let http_handler_interface = lib
        .define_interface(
            "http_handler",
            doc("Callback interface used to receive HTTP requests received by the runtime."),
        )?
        .begin_callback("on_request", "Called when a request is received")?
        .param("response", response, "The response to write to")?
        .end_callback()?
        .build_async()?;
    Ok(http_handler_interface)
}

pub(crate) fn define_future(lib: &mut LibraryBuilder) -> BackTraced<FutureInterfaceHandle> {
    let handle_request_error = lib
        .define_error_type(
            "handle_request_error",
            "handle_request_exception",
            ExceptionType::CheckedException,
        )?
        .add_error(
            "handler_failed",
            "The handler failed to process the request",
        )?
        .doc("Error that can be returned by the HTTP handler")?
        .build()?;

    let callback = lib.define_future_interface(
        "handle_http_request",
        "Handler for a HTTP request",
        StringType,
        "A response string (temp)",
        handle_request_error,
    )?;

    Ok(callback)
}
