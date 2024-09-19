#[allow(clippy::extra_unused_lifetimes)]
pub(crate) use crate::tracing::*;
pub use app_config::*;
pub use application::*;
pub use headers::*;
pub use http_handler::*;
pub use response::*;
pub use runtime::*;

mod app_config;
mod application;
mod headers;
mod http_handler;
mod response;
mod runtime;
mod tracing;

pub mod ffi;

static VERSION: &str = concat!("1.2.3", "\0");

fn version() -> &'static std::ffi::CStr {
    unsafe { std::ffi::CStr::from_bytes_with_nul_unchecked(VERSION.as_bytes()) }
}

impl From<crate::TracingInitError> for std::os::raw::c_int {
    fn from(_: crate::TracingInitError) -> Self {
        crate::ffi::LoggingError::LoggingAlreadyConfigured.into()
    }
}

impl From<crate::runtime::RuntimeError> for std::os::raw::c_int {
    fn from(_: crate::runtime::RuntimeError) -> Self {
        crate::ffi::ApplicationStartupError::SetupFailed.into()
    }
}
