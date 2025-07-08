use oo_bindgen::model::*;

use crate::shared::SharedDefinitions;

pub(crate) fn define(
    lib: &mut LibraryBuilder,
    http_handler: AsynchronousInterface,
    _http_handler_future: FutureInterfaceHandle,
    app_config: ClassDeclarationHandle,
    shared_defs: SharedDefinitions,
) -> BackTraced<()> {
    // Declare the class
    let application_class = lib.declare_class("application")?;

    let runtime_class = sfio_tokio_ffi::define(lib, shared_defs.general_error_type.clone())?;

    let core_runtime_config_declr = lib.declare_universal_struct("core_runtime_config")?;
    let core_runtime_config = lib
        .define_universal_struct(core_runtime_config_declr)?
        .doc("Configuration ")?
        .add(
            "blueprint_config_path",
            StringType,
            "The path to the blueprint file for the application",
        )?
        .add(
            "server_port",
            Primitive::S32,
            "The port to run the application on if it is a HTTP and or WebSocket API",
        )?
        .add(
            "server_loopback_only",
            Primitive::Bool,
            "Whether to expose the server only on the loopback interface (localhost)",
        )?
        .end_fields()?
        .add_full_initializer("init")?
        .build()?;

    // Declare each native function
    let constructor = lib
        .define_constructor(application_class.clone())?
        .param("core_runtime_config", core_runtime_config, "Core runtime configuration")?
        .doc(doc("Create a new {class:application}")
            .details("Here are some details about {class:application}. You can call {class:application.get_value()} method."),
        )?
        .build()?;

    let destructor =
        lib.define_destructor(application_class.clone(), "Destroy a {class:application}")?;

    let get_value = lib
        .define_method("get_value", application_class.clone())?
        .returns(Primitive::U32, "Current value")?
        .doc("Get the value")?
        .build()?;

    let startup_error_type = lib
        .define_error_type(
            "application_startup_error",
            "application_startup_exception",
            ExceptionType::UncheckedException,
        )?
        .add_error(
            "setup_failed",
            "Application setup failed trying to resolve and parse configuration",
        )?
        .doc("Error returned on failure to start up an application")?
        .build()?;

    let setup = lib
        .define_method("setup", application_class.clone())?
        .doc("Sets up the runtime application, yielding parsed configuration for the application")?
        .returns(app_config, "Parsed configuration")?
        .fails_with(startup_error_type)?
        .build()?;

    let register_http_handler = lib
        .define_method("register_http_handler", application_class.clone())?
        .param("handler", http_handler, "HTTP handler")?
        .doc("Register a HTTP handler")?
        .build()?;

    let run = lib
        .define_method("run", application_class.clone())?
        .param(
            "runtime",
            runtime_class,
            "The runtime to run the server and or message polling app with",
        )?
        .param(
            "block",
            Primitive::Bool,
            "Whether to block the current thread to run the application",
        )?
        .doc("Run the application server and or message polling app with the provided runtime")?
        .fails_with(shared_defs.general_error_type.clone())?
        .build()?;

    let construction_counter = lib
        .define_function("construction_counter")?
        .returns(Primitive::U32, "Number of calls to the constructor")?
        .doc("Get number of calls to the constructor")?
        .build_static("construction_counter")?;

    // let register_http_handler_future = lib
    //     .define_future_method(
    //         "register_http_handler_future",
    //         application_class.clone(),
    //         http_handler_future,
    //     )?
    //     .doc("Register a handler for a HTTP request")?
    //     .build()?;

    // Define the class
    let _application = lib
        .define_class(&application_class)?
        .constructor(constructor)?
        .destructor(destructor)?
        .method(get_value)?
        .method(setup)?
        .method(register_http_handler)?
        .method(run)?
        // .async_method(register_http_handler_future)?
        .static_method(construction_counter)?
        .custom_destroy("shutdown")?
        .doc("A runtime application")?
        .build()?;

    Ok(())
}
