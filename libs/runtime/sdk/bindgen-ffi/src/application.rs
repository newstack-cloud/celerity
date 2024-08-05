use celerity_runtime_core::{
    application::Application as RuntimeApp,
    config::{RuntimeCallMode, RuntimeConfig},
};

static mut CONSTRUCTION_COUNTER: u32 = 0;

pub struct Application {
    runtime_app: RuntimeApp,
}

pub unsafe fn application_create(value: u32) -> *mut Application {
    CONSTRUCTION_COUNTER += 1;
    let application = Box::new(Application {
        runtime_app: RuntimeApp::new(RuntimeConfig {
            blueprint_config_path: "".to_string(),
            runtime_call_mode: RuntimeCallMode::Ffi,
            server_port: 3000,
            server_loopback_only: Some(true),
        }),
    });
    Box::into_raw(application)
}

pub unsafe fn application_get_value(application: *const Application) -> u32 {
    200
}

pub unsafe fn application_destroy(application: *mut Application) {
    CONSTRUCTION_COUNTER -= 1;
    if !application.is_null() {
        drop(Box::from_raw(application));
    };
}

pub unsafe fn construction_counter() -> u32 {
    CONSTRUCTION_COUNTER
}
