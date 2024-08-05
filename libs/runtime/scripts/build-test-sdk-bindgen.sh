#!/bin/bash

cargo build -p celerity_runtime_bindgen_ffi -p celerity_runtime_bindgen_ffi_java --release

cargo run --bin celerity-runtime-bindings -- --java
cargo run --bin celerity-runtime-bindings -- --dotnet