use std::{collections::HashMap, fmt::Display, sync::Arc};

use axum::http::HeaderMap;
use axum_extra::extract::CookieJar;
use celerity_blueprint_config_parser::blueprint::{
    CelerityApiAuthGuard, CelerityApiAuthGuardDiscoveryMode,
};
use celerity_helpers::http::{FetchResourceError, ResourceStore};
use jsonwebtoken::{decode, decode_header, jwk::JwkSet};
use serde::Deserialize;
use serde_json::json;

use crate::{
    consts::JWT_VALIDATION_CLOCK_SKEW_LEEWAY,
    utils::{get_websocket_token_source, strip_auth_scheme},
    value_sources::{extract_value_from_request_elements, ExtractValueError},
};

/// Validates a JWT on a WebSocket connection for the `connect` auth strategy.
pub async fn validate_jwt_on_ws_connect(
    auth_guard_config: &CelerityApiAuthGuard,
    headers: &HeaderMap,
    query: &HashMap<String, Vec<String>>,
    cookies: &CookieJar,
    resource_store: Arc<ResourceStore>,
) -> Result<serde_json::Value, ValidateJwtError> {
    let token_source_opt = get_websocket_token_source(auth_guard_config);

    match token_source_opt {
        Some(token_source) => {
            let token = extract_value_from_request_elements(
                token_source,
                serde_json::Value::Null,
                headers,
                query,
                cookies,
            )?;

            match token {
                serde_json::Value::String(token) => {
                    let stripped_token = strip_auth_scheme(&token, auth_guard_config);
                    let jwks = get_jwks(auth_guard_config, resource_store).await?;
                    let header = decode_header(&stripped_token)
                        .map_err(ValidateJwtError::FailedToDecodeHeader)?;
                    let kid = header.kid.clone().ok_or(ValidateJwtError::MissingKid)?;
                    let jwk = jwks.find(&kid).ok_or(ValidateJwtError::JwkNotFound(kid))?;
                    let mut validation =
                        jsonwebtoken::Validation::new(select_allowed_algorithm(&header));

                    if let Some(audience) = &auth_guard_config.audience {
                        validation.set_audience(audience);
                    }

                    if let Some(issuer) = &auth_guard_config.issuer {
                        validation.set_issuer(std::slice::from_ref(issuer));
                    }

                    validation.leeway = JWT_VALIDATION_CLOCK_SKEW_LEEWAY;

                    let token_data = decode::<serde_json::Value>(
                        &stripped_token,
                        &jsonwebtoken::DecodingKey::from_jwk(jwk)
                            .map_err(ValidateJwtError::FailedToExtractJwk)?,
                        &validation,
                    )?;
                    Ok(json!({
                        "claims": token_data.claims,
                    }))
                }
                _ => Err(ValidateJwtError::InvalidTokenValue(token.to_string())),
            }
        }
        None => Err(ValidateJwtError::TokenSourceMissing),
    }
}

#[derive(Debug, Deserialize)]
struct DiscoveryDocument {
    jwks_uri: String,
}

async fn get_jwks(
    auth_guard_config: &CelerityApiAuthGuard,
    resource_store: Arc<ResourceStore>,
) -> Result<JwkSet, ValidateJwtError> {
    let doc_url = match &auth_guard_config.issuer {
        Some(issuer) => match auth_guard_config
            .discovery_mode
            .clone()
            .unwrap_or(CelerityApiAuthGuardDiscoveryMode::Oidc)
        {
            CelerityApiAuthGuardDiscoveryMode::Oidc => {
                let final_issuer = add_trailing_slash(issuer);
                format!("{final_issuer}.well-known/openid-configuration")
            }
            CelerityApiAuthGuardDiscoveryMode::OAuth2 => {
                let final_issuer = add_trailing_slash(issuer);
                format!("{final_issuer}.well-known/oauth-authorization-server")
            }
        },
        None => {
            return Err(ValidateJwtError::IssuerMissing);
        }
    };

    let discovery_document_str = resource_store
        .get(&doc_url)
        .await
        .map_err(ValidateJwtError::FailedToGetDiscoveryDocument)?;

    let discovery_document: DiscoveryDocument = serde_json::from_str(&discovery_document_str)
        .map_err(ValidateJwtError::FailedToParseDiscoveryDocument)?;

    let jwks_str = resource_store
        .get(&discovery_document.jwks_uri)
        .await
        .map_err(ValidateJwtError::FailedToGetJwks)?;

    let jwks: JwkSet =
        serde_json::from_str(&jwks_str).map_err(ValidateJwtError::FailedToParseJwks)?;

    Ok(jwks)
}

fn add_trailing_slash(issuer: &str) -> String {
    if issuer.ends_with("/") {
        issuer.to_string()
    } else {
        format!("{issuer}/")
    }
}

#[derive(Debug)]
pub enum ValidateJwtError {
    JwtInvalid(String),
    JwtExpired,
    JwtNotValidForAudience,
    JwtNotValidForIssuer,
    JwtNotValidForSubject,
    InvalidAlgorithm,
    JwkNotFound(String),
    TokenSourceMissing,
    IssuerMissing,
    MissingKid,
    FailedToDecodeHeader(jsonwebtoken::errors::Error),
    FailedToGetDiscoveryDocument(FetchResourceError),
    FailedToParseDiscoveryDocument(serde_json::Error),
    FailedToGetJwks(FetchResourceError),
    FailedToParseJwks(serde_json::Error),
    FailedToExtractJwk(jsonwebtoken::errors::Error),
    InvalidTokenValue(String),
    ExtractTokenFailed(ExtractValueError),
}

impl Display for ValidateJwtError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            ValidateJwtError::JwtInvalid(e) => write!(f, "JWT invalid: {e}"),
            ValidateJwtError::JwtExpired => write!(f, "JWT expired"),
            ValidateJwtError::JwtNotValidForAudience => write!(f, "JWT not valid for audience"),
            ValidateJwtError::JwtNotValidForIssuer => write!(f, "JWT not valid for issuer"),
            ValidateJwtError::JwtNotValidForSubject => write!(f, "JWT not valid for subject"),
            ValidateJwtError::InvalidAlgorithm => write!(f, "Invalid algorithm"),
            ValidateJwtError::JwkNotFound(kid) => write!(f, "JWK not found for kid: {kid}"),
            ValidateJwtError::TokenSourceMissing => write!(f, "Token source missing"),
            ValidateJwtError::IssuerMissing => write!(f, "Issuer missing from configuration"),
            ValidateJwtError::MissingKid => write!(f, "Missing kid in header"),
            ValidateJwtError::InvalidTokenValue(e) => write!(f, "Invalid token value: {e}"),
            ValidateJwtError::ExtractTokenFailed(e) => write!(f, "Extract token failed: {e}"),
            ValidateJwtError::FailedToDecodeHeader(e) => write!(f, "Failed to decode header: {e}"),
            ValidateJwtError::FailedToGetDiscoveryDocument(e) => {
                write!(f, "Failed to get discovery document: {e}")
            }
            ValidateJwtError::FailedToParseDiscoveryDocument(e) => {
                write!(f, "Failed to parse discovery document: {e}")
            }
            ValidateJwtError::FailedToGetJwks(e) => write!(f, "Failed to get JWKS: {e}"),
            ValidateJwtError::FailedToParseJwks(e) => write!(f, "Failed to parse JWKS: {e}"),
            ValidateJwtError::FailedToExtractJwk(e) => write!(f, "Failed to extract JWK: {e}"),
        }
    }
}

impl From<ExtractValueError> for ValidateJwtError {
    fn from(e: ExtractValueError) -> Self {
        ValidateJwtError::ExtractTokenFailed(e)
    }
}

impl From<jsonwebtoken::errors::Error> for ValidateJwtError {
    fn from(e: jsonwebtoken::errors::Error) -> Self {
        match e.kind() {
            jsonwebtoken::errors::ErrorKind::InvalidAlgorithmName => {
                ValidateJwtError::InvalidAlgorithm
            }
            jsonwebtoken::errors::ErrorKind::ExpiredSignature => ValidateJwtError::JwtExpired,
            jsonwebtoken::errors::ErrorKind::InvalidAudience => {
                ValidateJwtError::JwtNotValidForAudience
            }
            jsonwebtoken::errors::ErrorKind::InvalidIssuer => {
                ValidateJwtError::JwtNotValidForIssuer
            }
            jsonwebtoken::errors::ErrorKind::InvalidSubject => {
                ValidateJwtError::JwtNotValidForSubject
            }
            _ => ValidateJwtError::JwtInvalid(e.to_string()),
        }
    }
}

fn select_allowed_algorithm(header: &jsonwebtoken::Header) -> jsonwebtoken::Algorithm {
    if SUPPORTED_ALGORITHMS.contains(&header.alg) {
        header.alg
    } else {
        jsonwebtoken::Algorithm::RS256
    }
}

const SUPPORTED_ALGORITHMS: [jsonwebtoken::Algorithm; 7] = [
    jsonwebtoken::Algorithm::RS256,
    jsonwebtoken::Algorithm::RS384,
    jsonwebtoken::Algorithm::RS512,
    jsonwebtoken::Algorithm::PS256,
    jsonwebtoken::Algorithm::ES256,
    jsonwebtoken::Algorithm::ES384,
    jsonwebtoken::Algorithm::EdDSA,
];

#[cfg(test)]
mod tests {
    use std::{collections::HashMap, sync::Arc};

    use axum::http::{HeaderMap, HeaderName, HeaderValue};
    use axum_extra::extract::CookieJar;
    use biscuit::{
        jwa::SignatureAlgorithm,
        jwk::{JWKSet, RSAKeyParameters},
        jws::{RegisteredHeader, Secret},
        ClaimsSet, Empty, RegisteredClaims, SingleOrMultiple, JWT,
    };
    use celerity_blueprint_config_parser::blueprint::{
        CelerityApiAuthGuard, CelerityApiAuthGuardDiscoveryMode, CelerityApiAuthGuardScheme,
        CelerityApiAuthGuardType, CelerityApiAuthGuardValueSource, CelerityApiProtocol,
        ValueSourceConfiguration,
    };
    use celerity_helpers::http::ResourceStore;
    use chrono::{DateTime, Duration, Utc};
    use httptest::{
        any_of,
        matchers::request,
        responders::{json_encoded, status_code},
        Expectation, Server,
    };
    use reqwest::Client;
    use ring::{
        error::KeyRejected,
        rsa::{KeyPairComponents, PublicKeyComponents},
        signature::RsaKeyPair,
    };
    use serde_json::json;

    use crate::auth_jwt::{validate_jwt_on_ws_connect, ValidateJwtError};

    fn setup_test_oidc_oauth2_server() -> Server {
        let server = Server::run();
        let jwks_uri = server.url("/.well-known/jwks.json").to_string();
        let metadata = json!({
            "jwks_uri": jwks_uri,
        });
        server.expect(
            Expectation::matching(any_of![
                request::method_path("GET", "/.well-known/openid-configuration",),
                request::method_path("GET", "/.well-known/oauth-authorization-server",),
            ])
            .respond_with(json_encoded(metadata)),
        );
        let jwks_str = include_str!("../tests/data/fixtures/public-jwks.json");
        server.expect(
            Expectation::matching(request::method_path("GET", "/.well-known/jwks.json"))
                .respond_with(
                    status_code(200)
                        .append_header("Content-Type", "application/json")
                        .body(jwks_str),
                ),
        );
        server
    }

    fn create_jwt(
        subject: String,
        audience: String,
        issuer: String,
        expiry: DateTime<Utc>,
    ) -> Result<String, ()> {
        let private_jwks = serde_json::from_str::<JWKSet<RSAKeyParameters>>(include_str!(
            "../tests/data/fixtures/private-jwks.json"
        ))
        .unwrap();
        let private_jwk = private_jwks.keys[0].clone();
        let claims = ClaimsSet::<Empty> {
            registered: RegisteredClaims {
                issuer: Some(issuer),
                subject: Some(subject),
                audience: Some(SingleOrMultiple::Single(audience)),
                expiry: Some(expiry.into()),
                ..Default::default()
            },
            private: Default::default(),
        };
        let jwt = JWT::new_decoded(
            From::from(RegisteredHeader {
                algorithm: SignatureAlgorithm::RS256,
                key_id: private_jwk.common.key_id,
                ..Default::default()
            }),
            claims,
        );
        let rsa_key_pair = jwk_to_rsa_key_pair(&private_jwk.additional).unwrap();
        let token = jwt
            .into_encoded(&Secret::RsaKeyPair(Arc::new(rsa_key_pair)))
            .unwrap();
        Ok(token.unwrap_encoded().to_string())
    }

    fn jwk_to_rsa_key_pair(jwk: &RSAKeyParameters) -> Result<RsaKeyPair, KeyRejected> {
        RsaKeyPair::from_components(&KeyPairComponents {
            public_key: PublicKeyComponents {
                n: jwk.n.to_bytes_be(),
                e: jwk.e.to_bytes_be(),
            },
            d: jwk
                .d
                .as_ref()
                .expect("d is required for signing RSA key")
                .to_bytes_be(),
            p: jwk
                .p
                .as_ref()
                .expect("p is required for signing RSA key")
                .to_bytes_be(),
            q: jwk
                .q
                .as_ref()
                .expect("q is required for signing RSA key")
                .to_bytes_be(),
            dP: jwk
                .dp
                .as_ref()
                .expect("dP is required for signing RSA key")
                .to_bytes_be(),
            dQ: jwk
                .dq
                .as_ref()
                .expect("dQ is required for signing RSA key")
                .to_bytes_be(),
            qInv: jwk
                .qi
                .as_ref()
                .expect("qi is required for signing RSA key")
                .to_bytes_be(),
        })
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_jwt_on_ws_connect_for_valid_token_from_oidc_provider() {
        run_validate_jwt_on_ws_connect_test(ValidateJwtOnWsConnectTestConfig {
            discovery_mode: CelerityApiAuthGuardDiscoveryMode::Oidc,
            invalid_audience: false,
        })
        .await;
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_jwt_on_ws_connect_for_valid_token_from_oauth2_provider() {
        run_validate_jwt_on_ws_connect_test(ValidateJwtOnWsConnectTestConfig {
            discovery_mode: CelerityApiAuthGuardDiscoveryMode::OAuth2,
            invalid_audience: false,
        })
        .await;
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_jwt_on_ws_connect_fails_for_invalid_token() {
        run_validate_jwt_on_ws_connect_test(ValidateJwtOnWsConnectTestConfig {
            discovery_mode: CelerityApiAuthGuardDiscoveryMode::Oidc,
            invalid_audience: true,
        })
        .await;
    }

    struct ValidateJwtOnWsConnectTestConfig {
        discovery_mode: CelerityApiAuthGuardDiscoveryMode,
        invalid_audience: bool,
    }

    async fn run_validate_jwt_on_ws_connect_test(config: ValidateJwtOnWsConnectTestConfig) {
        let server = setup_test_oidc_oauth2_server();
        let resource_store = Arc::new(ResourceStore::new(Client::new(), 600));
        let test_audience = "test-audience";
        let auth_guard_config = CelerityApiAuthGuard {
            guard_type: CelerityApiAuthGuardType::Jwt,
            issuer: Some(server.url("").to_string()),
            discovery_mode: Some(config.discovery_mode),
            token_source: Some(CelerityApiAuthGuardValueSource::ValueSourceConfiguration(
                vec![ValueSourceConfiguration {
                    protocol: CelerityApiProtocol::WebSocket,
                    source: "$.headers.Authorization".to_string(),
                }],
            )),
            audience: Some(vec![test_audience.to_string()]),
            auth_scheme: Some(CelerityApiAuthGuardScheme::Bearer),
        };
        let audience = if config.invalid_audience {
            "invalid-audience".to_string()
        } else {
            test_audience.to_string()
        };
        let token = create_jwt(
            "test-subject".to_string(),
            audience,
            auth_guard_config.issuer.clone().unwrap(),
            Utc::now() + Duration::hours(1),
        )
        .unwrap();

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str(format!("Bearer {token}").as_str()).unwrap(),
        )]);

        let result = validate_jwt_on_ws_connect(
            &auth_guard_config,
            &headers,
            &HashMap::new(),
            &CookieJar::new(),
            resource_store,
        )
        .await;

        if config.invalid_audience {
            assert!(result.is_err());
            assert!(matches!(
                result.err().unwrap(),
                ValidateJwtError::JwtNotValidForAudience
            ));
        } else {
            assert!(result.is_ok());
            assert_eq!(
                result.unwrap(),
                json!({
                    "claims": {
                        "sub": "test-subject",
                        "aud": test_audience,
                        "iss": auth_guard_config.issuer.clone().unwrap(),
                        "exp": (Utc::now() + Duration::hours(1)).timestamp(),
                    },
                })
            );
        }
    }
}
