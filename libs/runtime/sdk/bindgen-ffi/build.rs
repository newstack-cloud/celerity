use std::env;
use std::io::Write;
use std::path::Path;

fn write_tracing_ffi() {
    let mut file =
        std::fs::File::create(Path::new(&env::var_os("OUT_DIR").unwrap()).join("tracing.rs"))
            .unwrap();
    file.write_all(sfio_tracing_ffi::get_impl_file().as_bytes())
        .unwrap();
}

fn write_tokio_ffi() {
    let mut file =
        std::fs::File::create(Path::new(&env::var_os("OUT_DIR").unwrap()).join("runtime.rs"))
            .unwrap();
    file.write_all(sfio_tokio_ffi::get_impl_file().as_bytes())
        .unwrap();
}

fn main() {
    println!("cargo:rerun-if-changed=build.rs");

    write_tracing_ffi();
    write_tokio_ffi();

    match celerity_runtime_bindgen_schema::build_lib() {
        Err(err) => {
            eprintln!("{err}");
            std::process::exit(-1);
        }
        Ok(lib) => {
            oo_bindgen::backend::rust::generate_ffi(&lib).unwrap();
        }
    }
}
