use std::path::Path;
use std::rc::Rc;

fn main() {
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
        .with_target(false)
        .init();

    let builder_settings = oo_bindgen::cli::BindingBuilderSettings {
        ffi_target_name: "celerity-runtime-bindgen-ffi",
        jni_target_name: "celerity-runtime-bindgen-ffi-api",
        ffi_name: "celerity_runtime_bindgen_ffi",
        ffi_path: Path::new("sdk/bindgen-ffi").into(),
        java_group_id: "com.newstack",
        destination_path: Path::new("sdk/bindings").into(),
        library: Rc::new(celerity_runtime_bindgen_schema::build_lib().unwrap()),
    };

    oo_bindgen::cli::run(builder_settings);
}
