use std::{collections::HashMap, env::VarError};

use celerity_runtime_core::env::EnvVars;

pub struct MockEnvVars<'a> {
    var_map: HashMap<&'a str, String>,
}

impl<'a> MockEnvVars<'a> {
    pub fn new(env_vars: Option<HashMap<&'a str, String>>) -> Self {
        MockEnvVars {
            var_map: env_vars.or_else(|| Some(HashMap::new())).unwrap(),
        }
    }
}

impl<'a> EnvVars for MockEnvVars<'a> {
    fn var(&self, key: &str) -> Result<String, VarError> {
        return match self.var_map.get(key) {
            Some(value) => Ok(value.clone()),
            None => Err(VarError::NotPresent),
        };
    }
}
