use std::path::PathBuf;

use oo_bindgen::model::*;

mod app_config;
mod application;
mod headers;
mod http_handler;
mod response;
mod shared;

pub fn build_lib() -> BackTraced<Library> {
    let lib_info = LibraryInfo {
        description: "Foo is an interesting library".to_string(),
        project_url: "https://celerityframework.com/".to_string(),
        repository: "newstack-cloud/celerity".to_string(),
        license_name: "MIT".to_string(),
        license_description: [
            "foo v1.2.3",
            "Copyright (C) 2020-2021 Step Function I/O",
            "",
            "This is my custom license.",
            "These views are not even my own. They belong to nobody.",
            "  - Frumious Scadateer (@scadateer)",
        ]
        .iter()
        .map(|s| s.to_string())
        .collect(),
        license_path: PathBuf::from("../../LICENSE"),
        developers: vec![DeveloperInfo {
            name: "Andre Sutherland".to_string(),
            email: "info@newstack.cloud".to_string(),
            organization: "Two Hundred".to_string(),
            organization_url: "https://newstack.cloud/".to_string(),
        }],
        logo_png: include_bytes!("../resources/logo.png"),
    };

    let settings = LibrarySettings::create(
        "celerity",
        "celerity_runtime_sdk",
        ClassSettings::default(),
        IteratorSettings::default(),
        CollectionSettings::default(),
        FutureSettings::default(),
        InterfaceSettings::default(),
    )?;

    let mut builder = LibraryBuilder::new(Version::parse("1.2.3").unwrap(), lib_info, settings);

    let shared_definitions = shared::define(&mut builder)?;

    let headers = headers::define(&mut builder)?;
    let response = response::define(&mut builder, headers.declaration())?;
    let http_handler = http_handler::define(&mut builder, response.declaration())?;
    let http_handler_future = http_handler::define_future(&mut builder)?;
    let app_conf = app_config::define(&mut builder)?;
    application::define(
        &mut builder,
        http_handler,
        http_handler_future,
        app_conf.declaration(),
        shared_definitions,
    )?;

    let logging_error_type = builder
        .define_error_type(
            "logging_error",
            "logging_exception",
            ExceptionType::UncheckedException,
        )?
        .add_error(
            "logging_already_configured",
            "Logging can only be configured once",
        )?
        .doc("Error returned on failure to setup logging")?
        .build()?;

    // common logging interface with other libraries
    sfio_tracing_ffi::define(&mut builder, logging_error_type)?;

    let library = builder.build()?;

    Ok(library)
}
