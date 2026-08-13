//! Regenerates the checked-in Rust stubs for the IPC handler protocol.
//!
//! The generated code is committed rather than produced during `cargo build`,
//! so that building the runtime needs no tooling beyond a Rust toolchain. That
//! matters because the SDK release workflows cross-compile inside containers
//! and virtual machines, where a build-time compiler would have to be present
//! in every one of them.
//!
//! Run this after changing the `.proto`:
//!
//! ```text
//! cargo run -p celerity-proto-gen
//! ```
//!
//! Requires `buf` on the path. See `proto/README.md`.

use std::{path::PathBuf, process::Command};

use prost::Message;
use prost_types::FileDescriptorSet;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let runtime_root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("the generator should live one level below the runtime root")
        .to_path_buf();

    let proto_root = runtime_root.join("proto");
    let out_dir = runtime_root.join("core/src/generated");
    std::fs::create_dir_all(&out_dir)?;

    let descriptor_set = build_descriptor_set(&proto_root)?;

    tonic_prost_build::configure()
        .out_dir(&out_dir)
        .build_server(true)
        .build_client(true)
        .compile_fds(descriptor_set)?;

    println!("generated stubs into {}", out_dir.display());
    Ok(())
}

/// Compiles the protocol with `buf` and reads back the descriptor set.
///
/// `buf` rather than `protoc` because it is the only tool the protocol needs
/// otherwise, for linting and for checking compatibility, and one toolchain
/// means CI and a developer's machine cannot disagree about which compiler
/// produced the checked-in stubs.
fn build_descriptor_set(
    proto_root: &PathBuf,
) -> Result<FileDescriptorSet, Box<dyn std::error::Error>> {
    // `--as-file-descriptor-set` strips buf's own extensions, leaving what
    // prost expects. Source info is kept, since that is where the comments on
    // the generated types come from.
    let output = Command::new("buf")
        .args(["build", "--as-file-descriptor-set", "--output", "-"])
        .current_dir(proto_root)
        .output()
        .map_err(|err| format!("could not run buf, is it installed? {err}"))?;

    if !output.status.success() {
        return Err(format!(
            "buf could not build the protocol:\n{}",
            String::from_utf8_lossy(&output.stderr)
        )
        .into());
    }

    Ok(FileDescriptorSet::decode(output.stdout.as_slice())?)
}
