#[allow(clippy::extra_unused_lifetimes)]
pub mod ffi;

pub use application::*;

mod application;

static VERSION: &str = concat!("1.2.3", "\0");

fn version() -> &'static std::ffi::CStr {
    unsafe { std::ffi::CStr::from_bytes_with_nul_unchecked(VERSION.as_bytes()) }
}
