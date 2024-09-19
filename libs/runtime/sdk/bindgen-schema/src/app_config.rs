use oo_bindgen::model::*;

pub(crate) fn define(lib: &mut LibraryBuilder) -> BackTraced<ClassHandle> {
    let app_config = lib.declare_class("app_config")?;

    let constructor = lib
        .define_constructor(app_config.clone())?
        .doc("Create a new set of configuration for an application")?
        .build()?;

    let destructor = lib.define_destructor(
        app_config.clone(),
        "Destroy app configuration created with {class:app_config.[constructor]}.",
    )?;

    let api_config = define_api_config(lib)?;
    let api_config_method = lib
        .define_method("get_api_config", app_config.clone())?
        .returns(api_config.declaration(), "API configuration")?
        .doc("Get the API configuration for the application")?
        .build()?;

    let app_config = lib
        .define_class(&app_config)?
        .constructor(constructor)?
        .method(api_config_method)?
        .destructor(destructor)?
        .doc("Configuration to run an application")?
        .build()?;

    Ok(app_config)
}

fn define_api_config(lib: &mut LibraryBuilder) -> BackTraced<ClassHandle> {
    let api_config = lib.declare_class("api_config")?;

    let constructor = lib
        .define_constructor(api_config.clone())?
        .doc("Create API configuration for an application")?
        .build()?;

    let destructor = lib.define_destructor(
        api_config.clone(),
        "Destroy api configuration created with {class:api_config.[constructor]}.",
    )?;

    let http_config = define_http_config(lib)?;
    let http_config_method = lib
        .define_method("get_http_config", api_config.clone())?
        .returns(http_config.declaration(), "HTTP API configuration")?
        .doc("Get the HTTP API configuration for the application")?
        .build()?;

    let api_config = lib
        .define_class(&api_config)?
        .constructor(constructor)?
        .method(http_config_method)?
        .destructor(destructor)?
        .doc("API configuration to run an application")?
        .build()?;

    Ok(api_config)
}

fn define_http_config(lib: &mut LibraryBuilder) -> BackTraced<ClassHandle> {
    let http_config = lib.declare_class("http_config")?;

    let constructor = lib
        .define_constructor(http_config.clone())?
        .doc("Create HTTP API configuration for an application")?
        .build()?;

    let destructor = lib.define_destructor(
        http_config.clone(),
        "Destroy api configuration created with {class:http_config.[constructor]}.",
    )?;

    let http_handlers_iterator = define_handlers_iterator(lib)?;

    let http_handlers_receiver = lib
        .define_interface(
            "http_handlers_receiver",
            "Callback interface for receiving HTTP handler definitions",
        )?
        .begin_callback(
            "on_http_handler_definitions",
            "callback to receive HTTP handler definitions",
        )?
        .param(
            "handler_definitions",
            http_handlers_iterator,
            "HTTP handler definitions",
        )?
        .enable_functional_transform()
        .end_callback()?
        .build_sync()?;

    let http_handlers_iterator_method = lib
        .define_method("receive_handlers", http_config.clone())?
        .param(
            "callback",
            http_handlers_receiver,
            "A callback to receive HTTP handler definitions",
        )?
        .doc("Iterate over HTTP handler definitions for the application")?
        .build()?;

    let http_config = lib
        .define_class(&http_config)?
        .constructor(constructor)?
        .method(http_handlers_iterator_method)?
        .destructor(destructor)?
        .doc("HTTP API configuration to run an application")?
        .build()?;

    Ok(http_config)
}

fn define_handlers_iterator(lib: &mut LibraryBuilder) -> BackTraced<AbstractIteratorHandle> {
    let handlers_struct_decl = lib.declare_universal_struct("http_handler_definition")?;
    let handlers_struct = lib
        .define_universal_struct(handlers_struct_decl)?
        .doc("A definition for a HTTP handler")?
        .add("path", StringType, "Path of the handler")?
        .add("method", StringType, "HTTP method of the handler")?
        .add("location", StringType, "Location of the handler")?
        .add("handler", StringType, "Handler name")?
        .end_fields()?
        .add_full_initializer("init")?
        .build()?;

    let handlers_iterator =
        lib.define_iterator("http_handler_iterator", handlers_struct.clone())?;

    Ok(handlers_iterator)
}
