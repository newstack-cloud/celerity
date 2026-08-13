pub mod application;
pub mod auth_custom;
pub mod auth_http;
mod auth_jwt;
pub mod blueprint_helpers;
pub mod body_transform;
pub mod config;
pub mod consts;
pub mod consumer_handler;
pub mod dispatcher;
pub mod errors;
pub mod event_queue;
pub mod handler_invoke;
/// The IPC handler protocol, generated from
/// `libs/runtime/proto/celerity/runtime/v1/runtime.proto`.
///
/// This is checked in rather than generated during the build, so that building
/// the runtime needs no tooling beyond a Rust toolchain. Do not edit it by
/// hand: change the `.proto` and run `cargo run -p celerity-proto-gen`.
#[allow(clippy::all, clippy::pedantic, missing_docs)]
#[rustfmt::skip]
pub mod ipc_proto {
    include!("generated/celerity.runtime.v1.rs");
}

pub mod ipc_frames;
pub mod ipc_http;
pub mod ipc_stream;
pub mod ipc_websocket;
pub mod request;
pub(crate) mod runtime_local_api;
pub(crate) mod telemetry;
pub mod telemetry_utils;
mod transform_config;
pub mod types;
pub(crate) mod utils;
mod value_sources;
pub mod websocket;
