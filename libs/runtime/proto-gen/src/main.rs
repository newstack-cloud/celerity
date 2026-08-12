//! Regenerates the checked-in Rust stubs for the IPC handler protocol.
//!
//! The generated code is committed rather than produced during `cargo build`,
//! so that building the runtime needs no tooling beyond a Rust toolchain. That
//! matters because the SDK release workflows cross-compile inside containers
//! and virtual machines, where a build-time `protoc` would have to be present
//! in every one of them.
//!
//! Run this after changing the `.proto`:
//!
//! ```text
//! cargo run -p celerity-proto-gen
//! ```
//!
//! Requires `protoc` on the path. See `proto/README.md`.

use std::path::PathBuf;

const PROTO: &str = "celerity/runtime/v1/runtime.proto";

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let runtime_root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("the generator should live one level below the runtime root")
        .to_path_buf();

    let proto_root = runtime_root.join("proto");
    let out_dir = runtime_root.join("core/src/generated");
    std::fs::create_dir_all(&out_dir)?;

    tonic_prost_build::configure()
        .out_dir(&out_dir)
        .build_server(true)
        .build_client(true)
        .compile_protos(&[proto_root.join(PROTO)], &[proto_root])?;

    println!("generated stubs into {}", out_dir.display());
    Ok(())
}
