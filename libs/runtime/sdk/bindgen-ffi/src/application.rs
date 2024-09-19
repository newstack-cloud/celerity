use std::{collections::HashMap, sync::Arc};

use axum::{body::Body, http::Request};
use celerity_runtime_core::{
    application::Application as RuntimeApp,
    config::{RuntimeCallMode, RuntimeConfig},
    errors::{ApplicationStartError, ConfigError},
};
use tokio::select;

use crate::{
    ffi::{self, ApplicationStartupError, CoreRuntimeConfig, GeneralError},
    http_config_create, ApiConfig, AppConfig, HandlerError, HttpConfig, HttpHeaders, Response,
    ResponseData, RuntimeError,
};

static mut CONSTRUCTION_COUNTER: u32 = 0;

pub struct Application {
    runtime_app: RuntimeApp,
    handler: Option<Arc<dyn Listener<Response>>>,
}

pub unsafe fn application_create(core_runtime_config: CoreRuntimeConfig) -> *mut Application {
    CONSTRUCTION_COUNTER += 1;

    let blueprint_path = core_runtime_config
        .blueprint_config_path()
        .to_string_lossy()
        .to_string();

    let application = Box::new(Application {
        runtime_app: RuntimeApp::new(RuntimeConfig {
            blueprint_config_path: blueprint_path,
            runtime_call_mode: RuntimeCallMode::Ffi,
            server_port: core_runtime_config.server_port(),
            server_loopback_only: Some(true),
            local_api_port: 3001,
            use_custom_health_check: Some(false),
        }),
        handler: None,
    });

    Box::into_raw(application)
}

pub unsafe fn application_get_value(application: *const Application) -> u32 {
    200
}

pub unsafe fn application_setup(
    instance: *mut Application,
) -> Result<*mut AppConfig, ApplicationStartupError> {
    if !instance.is_null() {
        let app = &mut *instance;

        match app.runtime_app.setup() {
            Ok(config) => {
                let mut handlers = vec![];
                if let Some(core_api_config) = config.api {
                    if let Some(core_http_config) = core_api_config.http {
                        handlers = core_http_config.handlers;
                    }
                }

                let api_config = ApiConfig::new(HttpConfig::new(handlers.into_iter()));
                let app_config = AppConfig::new(api_config);
                Ok(Box::into_raw(Box::new(app_config)))
            }
            Err(err) => {
                println!("Application Start Error: {:?}", err);
                Err(ApplicationStartupError::SetupFailed)
            }
        }
    } else {
        Err(ApplicationStartupError::SetupFailed)
    }
}

pub unsafe fn application_register_http_handler(
    instance: *mut Application,
    handler: ffi::HttpHandler,
) {
    if !instance.is_null() {
        let app = &mut *instance;
        app.handler = Some(Arc::new(handler));
        let hndlr_clone = app.handler.as_ref().unwrap().clone();

        let final_handler = move |_req: Request<Body>| async move {
            let (tx, rx) = tokio::sync::oneshot::channel::<ResponseData>();
            let mut resp = Response {
                status: 0,
                headers: HttpHeaders {},
                body: None,
                send_resp_channel: Some(tx),
            };
            // Ok::<ResponseData, HandlerError>(ResponseData {
            //     status: 200,
            //     headers: Some(HashMap::new()),
            //     body: Some("Hello, World!".to_string()),
            // })
            hndlr_clone.on_event(&mut resp);
            select! {
                result = tokio::time::timeout(tokio::time::Duration::from_secs(30), rx) => {
                    match result {
                        Ok(resp_result) => match resp_result {
                            Ok(resp_data) => {
                                Ok(resp_data)
                            }
                            Err(err) => {
                                Err(HandlerError::new(format!("Handler failed: {}", err)))
                            }
                        },
                        Err(_) => {
                            Err(HandlerError::new("Handler timed out".to_string()))
                        }
                    }
                }
            }
        };
        app.runtime_app
            .register_http_handler("/orders/:orderId", "POST", final_handler);
    }
}

pub unsafe fn application_run(
    instance: *mut crate::Application,
    runtime: *mut crate::Runtime,
    block: bool,
) -> Result<(), GeneralError> {
    if !instance.is_null() {
        let app = &mut *instance;
        let runtime = runtime.as_ref().ok_or(GeneralError::NullParameter)?;
        let future = app.runtime_app.run(true);
        if block {
            runtime.handle().block_on(future)??;
        } else {
            runtime.handle().spawn(future)?;
        }
        Ok(())
    } else {
        Err(GeneralError::ApplicationNotInitialised)
    }
}

// A generic listener type that can be invoked multiple times.
trait Listener<T>: Send + Sync {
    fn on_event(&self, item: &mut T);
}

impl Listener<Response> for ffi::HttpHandler {
    fn on_event(&self, item: &mut Response) {
        self.on_request(item);
    }
}

impl From<ApplicationStartError> for GeneralError {
    fn from(error: ApplicationStartError) -> Self {
        match error {
            ApplicationStartError::Environment(_) => {
                // todo: log specific error before passing to ffi
                GeneralError::ApplicationStartEnvironmentError
            }
            ApplicationStartError::BlueprintParse(_) => {
                // todo: log specific error before passing to ffi
                GeneralError::ApplicationStartBlueprintParseError
            }
            ApplicationStartError::Config(config_err) => match config_err {
                ConfigError::Api(_) => GeneralError::ApplicationStartConfigApiError,
                ConfigError::ApiMissing => GeneralError::ApplicationStartConfigApiMissing,
            },
            ApplicationStartError::TaskWaitError(_) => GeneralError::ApplicationStartTaskWaitError,
        }
    }
}

impl From<RuntimeError> for GeneralError {
    fn from(error: RuntimeError) -> Self {
        match error {
            RuntimeError::FailedToCreateRuntime => GeneralError::RuntimeCreationFailure,
            RuntimeError::RuntimeDestroyed => GeneralError::RuntimeDestroyed,
            RuntimeError::CannotBlockWithinAsync => GeneralError::RuntimeCannotBlockWithinAsync,
        }
    }
}

// pub unsafe fn application_register_http_handler_future(
//     instance: *mut Application,
//     handler: ffi::HandleHttpRequest,
// ) {
//     if !instance.is_null() {
//         let app = &mut *instance;

//         let final_handler = |_req: Request<Body>| async move {
//             let (tx, mut rx) = tokio::sync::oneshot::channel();
//             let resp = Response {
//                 status: 0,
//                 headers: std::ptr::null_mut(),
//                 body: None,
//                 send_resp_channel: Some(tx),
//             };
//             handler.on_complete(&mut resp);
//             select! {
//                 result = tokio::time::timeout(tokio::time::Duration::from_secs(30), rx) => {
//                     match result {
//                         Ok(_) => {
//                             Ok(resp)
//                         }
//                         Err(_) => {
//                             Err(HandlerError::new("Handler timed out".to_string()))
//                         }
//                     }
//                 }
//             }
//         };
//         app.runtime_app
//             .register_http_handler("/orders/:orderId", "POST", final_handler);
//     }
// }

pub unsafe fn application_destroy(instance: *mut Application) {
    CONSTRUCTION_COUNTER -= 1;
    if !instance.is_null() {
        let app = &mut *instance;
        app.runtime_app.shutdown();
        drop(Box::from_raw(instance));
    };
}

pub unsafe fn construction_counter() -> u32 {
    CONSTRUCTION_COUNTER
}
