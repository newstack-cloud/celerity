$env:CELERITY_VARIABLE_secretStoreId="test-secret-store"
$env:CELERITY_VARIABLE_certificateId="test-certificate"
$env:CELERITY_VARIABLE_logLevel="DEBUG"
$env:CELERITY_VARIABLE_paymentApiSecret="test-payment-api-secret"

cargo build -p celerity_runtime_bindgen_ffi -p celerity_runtime_bindgen_ffi_java --release

cargo run --bin celerity-runtime-bindings -- --java
cargo run --bin celerity-runtime-bindings -- --dotnet
