use std::{collections::HashMap, env::VarError, sync::Arc};

use celerity_helpers::env::EnvVars;

pub struct MockEnvVars<'a> {
    var_map: Arc<HashMap<&'a str, String>>,
}

impl<'a> MockEnvVars<'a> {
    pub fn new(env_vars: Option<HashMap<&'a str, String>>) -> Self {
        MockEnvVars {
            var_map: Arc::new(env_vars.or_else(|| Some(HashMap::new())).unwrap()),
        }
    }
}

impl EnvVars for MockEnvVars<'static> {
    fn var(&self, key: &str) -> Result<String, VarError> {
        match self.var_map.get(key) {
            Some(value) => Ok(value.clone()),
            None => Err(VarError::NotPresent),
        }
    }

    fn clone_env_vars(&self) -> Box<dyn EnvVars> {
        Box::new(MockEnvVars {
            var_map: Arc::clone(&self.var_map),
        })
    }
}
