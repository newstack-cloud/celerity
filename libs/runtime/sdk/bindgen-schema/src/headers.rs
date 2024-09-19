use oo_bindgen::model::*;

pub(crate) fn define(lib: &mut LibraryBuilder) -> BackTraced<ClassHandle> {
    let headers = lib.declare_class("http_headers")?;

    let constructor = lib
        .define_constructor(headers.clone())?
        .doc("Create a new set of headers")?
        .build()?;

    let destructor = lib.define_destructor(
        headers.clone(),
        "Destroy a set of headers created with {class:http_headers.[constructor]}.",
    )?;

    let headers = lib
        .define_class(&headers)?
        .constructor(constructor)?
        .destructor(destructor)?
        .doc("A set of headers from a HTTP request")?
        .build()?;

    Ok(headers)
}
