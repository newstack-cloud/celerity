use std::path::PathBuf;

use oo_bindgen::model::*;

mod application;

pub fn build_lib() -> BackTraced<Library> {
    let lib_info = LibraryInfo {
        description: "Foo is an interesting library".to_string(),
        project_url: "https://celerityframework.com/".to_string(),
        repository: "two-hundred/celerity".to_string(),
        license_name: "MIT".to_string(),
        license_description: [
            "foo v1.2.3",
            "Copyright (C) 2020-2021 Step Function I/O",
            "",
            "This is my custom license.",
            "These views are not even my own. They belong to nobody.",
            "  - Frumious Scadateer (@scadateer)",
        ]
        .iter()
        .map(|s| s.to_string())
        .collect(),
        license_path: PathBuf::from("../../LICENSE"),
        developers: vec![DeveloperInfo {
            name: "Andre Sutherland".to_string(),
            email: "info@twohundred.cloud".to_string(),
            organization: "Two Hundred".to_string(),
            organization_url: "https://twohundred.cloud/".to_string(),
        }],
        logo_png: include_bytes!("../resources/logo.png"),
    };

    let settings = LibrarySettings::create(
        "celerity",
        "celerity_runtime_sdk",
        ClassSettings::default(),
        IteratorSettings::default(),
        CollectionSettings::default(),
        FutureSettings::default(),
        InterfaceSettings::default(),
    )?;

    let mut builder = LibraryBuilder::new(Version::parse("1.2.3").unwrap(), lib_info, settings);

    application::define(&mut builder)?;

    let library = builder.build()?;

    Ok(library)
}
