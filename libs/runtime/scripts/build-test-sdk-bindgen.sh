#!/bin/bash

export CELERITY_VARIABLE_secretStoreId="test-secret-store"
export CELERITY_VARIABLE_certificateId="test-certificate"
export CELERITY_VARIABLE_logLevel="DEBUG"
export CELERITY_VARIABLE_paymentApiSecret="test-payment-api-secret"

cargo build -p celerity_runtime_bindgen_ffi -p celerity_runtime_bindgen_ffi_java --release

cargo run --bin celerity-runtime-bindings -- --java
cargo run --bin celerity-runtime-bindings -- --dotnet
