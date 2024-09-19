use oo_bindgen::model::*;

pub(crate) fn define(
    lib: &mut LibraryBuilder,
    headers: ClassDeclarationHandle,
) -> BackTraced<ClassHandle> {
    let response = lib.declare_class("response")?;

    let constructor = lib
        .define_constructor(response.clone())?
        .param("status", Primitive::U16, "Status code")?
        .param("headers", headers.clone(), "Headers")?
        .param("body", StringType, "Body")?
        .doc("Create a new response")?
        .build()?;

    let destructor = lib.define_destructor(
        response.clone(),
        "Destroy a request created with {class:response.[constructor]}.",
    )?;

    let set_status = lib
        .define_method("set_status", response.clone())?
        .param(
            "status",
            Primitive::U16,
            "Status code for the HTTP response",
        )?
        .doc("Sets the status code for the HTTP response, must be called before send")?
        .build()?;

    let set_headers = lib
        .define_method("set_headers", response.clone())?
        .param("headers", headers, "Headers to set for the HTTP response")?
        .doc("Sets the headers for the HTTP response, must be called before send")?
        .build()?;

    let send = lib
        .define_method("send", response.clone())?
        .param("body", StringType, "Body to send")?
        .doc("Sends the response to the client with the provided body")?
        .build()?;

    let response = lib
        .define_class(&response)?
        .constructor(constructor)?
        .method(set_status)?
        .method(set_headers)?
        .method(send)?
        .destructor(destructor)?
        .doc("A response to write a HTTP response to")?
        .build()?;

    Ok(response)
}
