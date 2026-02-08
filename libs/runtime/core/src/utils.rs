use std::time::{SystemTime, UNIX_EPOCH};

use celerity_blueprint_config_parser::blueprint::{
    CelerityApiAuthGuard, CelerityApiAuthGuardScheme, CelerityApiAuthGuardValueSource,
    CelerityApiProtocol,
};

pub fn get_epoch_seconds() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

pub fn strip_auth_scheme(token: &str, auth_guard_config: &CelerityApiAuthGuard) -> String {
    if let Some(auth_scheme) = &auth_guard_config.auth_scheme {
        let auth_scheme_str = to_auth_scheme_string(auth_scheme);
        if token.to_lowercase().starts_with(&auth_scheme_str) {
            return token[auth_scheme_str.len() + 1..].to_string();
        }
    }
    token.to_string()
}

fn to_auth_scheme_string(auth_scheme: &CelerityApiAuthGuardScheme) -> String {
    match auth_scheme {
        CelerityApiAuthGuardScheme::Bearer => "bearer".to_string(),
        CelerityApiAuthGuardScheme::Basic => "basic".to_string(),
        CelerityApiAuthGuardScheme::Digest => "digest".to_string(),
    }
}

pub fn get_websocket_token_source(auth_guard_config: &CelerityApiAuthGuard) -> Option<String> {
    match &auth_guard_config.token_source {
        Some(CelerityApiAuthGuardValueSource::Str(value_source)) => Some(value_source.clone()),
        Some(CelerityApiAuthGuardValueSource::ValueSourceConfiguration(value_source_configs)) => {
            for value_source_config in value_source_configs {
                match value_source_config.protocol {
                    CelerityApiProtocol::WebSocket => {
                        return Some(value_source_config.source.clone());
                    }
                    CelerityApiProtocol::WebSocketConfig(_) => {
                        return Some(value_source_config.source.clone());
                    }
                    _ => {}
                }
            }
            None
        }
        _ => None,
    }
}

pub fn get_http_token_source(auth_guard_config: &CelerityApiAuthGuard) -> Option<String> {
    match &auth_guard_config.token_source {
        Some(CelerityApiAuthGuardValueSource::Str(value_source)) => Some(value_source.clone()),
        Some(CelerityApiAuthGuardValueSource::ValueSourceConfiguration(value_source_configs)) => {
            for value_source_config in value_source_configs {
                if value_source_config.protocol == CelerityApiProtocol::Http {
                    return Some(value_source_config.source.clone());
                }
            }
            None
        }
        _ => None,
    }
}
