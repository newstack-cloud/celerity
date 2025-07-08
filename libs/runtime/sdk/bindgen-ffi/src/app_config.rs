use std::{ffi::CString, vec};

use celerity_runtime_core::config::HttpHandlerDefinition;

use crate::ffi;

pub struct AppConfig {
    api_config: ApiConfig,
}

impl AppConfig {
    pub fn new(api_config: ApiConfig) -> Self {
        Self { api_config }
    }
}

/// # Safety
/// The caller must ensure the returned pointer is properly managed and eventually deallocated using app_config_destroy.
pub unsafe fn app_config_create() -> *mut AppConfig {
    let http_config = HttpConfig::new(vec![].into_iter());
    let api_config = ApiConfig::new(http_config);
    let app_config = Box::new(AppConfig::new(api_config));
    Box::into_raw(app_config)
}

/// # Safety
/// The caller must ensure that `instance` is a valid pointer to AppConfig.
pub unsafe fn app_config_get_api_config(instance: *mut crate::AppConfig) -> *mut crate::ApiConfig {
    let handlers = (*instance).api_config.http_config.handlers.clone();
    let http_config = HttpConfig::new(handlers.into_iter());
    let api_config = Box::new(ApiConfig::new(http_config));
    Box::into_raw(api_config)
}

/// # Safety
/// The caller must ensure that `app_config` is a valid pointer to AppConfig and has not already been deallocated.
pub unsafe fn app_config_destroy(app_config: *mut AppConfig) {
    if !app_config.is_null() {
        drop(Box::from_raw(app_config));
    };
}

pub struct ApiConfig {
    http_config: HttpConfig,
}

impl ApiConfig {
    pub fn new(http_config: HttpConfig) -> Self {
        Self { http_config }
    }
}

/// # Safety
/// The caller must ensure the returned pointer is properly managed and eventually deallocated using api_config_destroy.
pub unsafe fn api_config_create() -> *mut ApiConfig {
    let http_config = HttpConfig::new(vec![].into_iter());
    let api_config = Box::new(ApiConfig::new(http_config));
    Box::into_raw(api_config)
}

/// # Safety
/// The caller must ensure that `api_config` is a valid pointer to ApiConfig and has not already been deallocated.
pub unsafe fn api_config_destroy(api_config: *mut ApiConfig) {
    if !api_config.is_null() {
        drop(Box::from_raw(api_config));
    };
}

/// # Safety
/// The caller must ensure that `instance` is a valid pointer to ApiConfig.
pub unsafe fn api_config_get_http_config(
    instance: *mut crate::ApiConfig,
) -> *mut crate::HttpConfig {
    let handlers = (*instance).http_config.handlers.clone();
    let http_config = Box::new(HttpConfig::new(handlers.into_iter()));
    Box::into_raw(http_config)
}

pub struct HttpConfig {
    handlers: vec::IntoIter<HttpHandlerDefinition>,
}

impl HttpConfig {
    pub fn new(handlers: vec::IntoIter<HttpHandlerDefinition>) -> Self {
        Self { handlers }
    }
}

/// # Safety
/// The caller must ensure the returned pointer is properly managed and eventually deallocated using http_config_destroy.
pub unsafe fn http_config_create() -> *mut HttpConfig {
    let http_config = Box::new(HttpConfig::new(vec![].into_iter()));
    Box::into_raw(http_config)
}

/// # Safety
/// The caller must ensure that `instance` is a valid pointer to HttpConfig and `callback` is a valid callback function.
pub unsafe fn http_config_receive_handlers(
    instance: *mut crate::HttpConfig,
    callback: ffi::HttpHandlersReceiver,
) {
    let mut iter = HttpHandlerIterator::new((*instance).handlers.clone());
    callback.on_http_handler_definitions(&mut iter);
}

/// # Safety
/// The caller must ensure that `http_config` is a valid pointer to HttpConfig and has not already been deallocated.
pub unsafe fn http_config_destroy(http_config: *mut HttpConfig) {
    if !http_config.is_null() {
        drop(Box::from_raw(http_config));
    };
}

pub struct HttpHandlerIterator {
    inner: vec::IntoIter<HttpHandlerDefinition>,
    current_path: CString,
    current_method: CString,
    current_handler: CString,
    current_location: CString,
    current: Option<ffi::HttpHandlerDefinition>,
}

impl HttpHandlerIterator {
    pub(crate) fn new(handlers: vec::IntoIter<HttpHandlerDefinition>) -> Self {
        Self {
            inner: handlers.into_iter(),
            current_path: Default::default(),
            current_method: Default::default(),
            current_handler: Default::default(),
            current_location: Default::default(),
            current: None,
        }
    }

    pub(crate) fn next(&mut self) -> Option<&ffi::HttpHandlerDefinition> {
        let next = self.inner.next()?;
        self.current_path = CString::new(next.path).unwrap();
        self.current_method = CString::new(next.method).unwrap();
        self.current_handler = CString::new(next.handler).unwrap();
        self.current_location = CString::new(next.location).unwrap();

        self.current = Some(ffi::HttpHandlerDefinition {
            path: self.current_path.as_ptr(),
            method: self.current_method.as_ptr(),
            handler: self.current_handler.as_ptr(),
            location: self.current_location.as_ptr(),
        });
        self.current.as_ref()
    }
}

/// # Safety
/// The caller must ensure that `iter` is a valid pointer to HttpHandlerIterator.
pub unsafe fn http_handler_iterator_next<'a>(
    iter: *mut HttpHandlerIterator,
) -> Option<&'a ffi::HttpHandlerDefinition> {
    iter.as_mut()?.next()
}
