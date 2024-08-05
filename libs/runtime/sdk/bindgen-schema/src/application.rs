use oo_bindgen::model::*;

pub fn define(lib: &mut LibraryBuilder) -> BackTraced<()> {
    // Declare the class
    let application_class = lib.declare_class("application")?;

    // Declare each native function
    let constructor = lib
        .define_constructor(application_class.clone())?
        .param("value", Primitive::U32, "Value")?
        .doc(doc("Create a new {class:application}")
            .details("Here are some details about {class:application}. You can call {class:application.get_value()} method."),
        )?
        .build()?;

    let destructor =
        lib.define_destructor(application_class.clone(), "Destroy a {class:application}")?;

    let get_value = lib
        .define_method("get_value", application_class.clone())?
        .returns(Primitive::U32, "Current value")?
        .doc("Get the value")?
        .build()?;

    let construction_counter = lib
        .define_function("construction_counter")?
        .returns(Primitive::U32, "Number of calls to the constructor")?
        .doc("Get number of calls to the constructor")?
        .build_static("construction_counter")?;

    // Define the class
    let _application = lib
        .define_class(&application_class)?
        .constructor(constructor)?
        .destructor(destructor)?
        .method(get_value)?
        .static_method(construction_counter)?
        .custom_destroy("shutdown")?
        .doc("A runtime application")?
        .build()?;

    Ok(())
}
