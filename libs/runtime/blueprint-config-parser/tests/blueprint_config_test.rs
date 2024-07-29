use pretty_assertions::assert_eq;
use std::collections::HashMap;

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintLinkSelector, BlueprintResourceMetadata, CelerityApiAuth,
    CelerityApiAuthGuard, CelerityApiAuthGuardType, CelerityApiAuthGuardValueSource,
    CelerityApiCors, CelerityApiCorsConfiguration, CelerityApiDomain,
    CelerityApiDomainSecurityPolicy, CelerityApiProtocol, CelerityApiSpec, CelerityResourceSpec,
    CelerityResourceType, RuntimeBlueprintResource,
};

#[test_log::test]
fn parses_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/http-api.yaml").unwrap();

    assert_eq!(
        blueprint_config,
        BlueprintConfig {
            version: "2023-04-20".to_string(),
            transform: Some(vec!["celerity-2024-07-22".to_string()]),
            variables: None,
            resources: HashMap::from([(
                "ordersApi".to_string(),
                RuntimeBlueprintResource {
                    resource_type: CelerityResourceType::CelerityApi,
                    metadata: BlueprintResourceMetadata {
                        display_name: "Orders API".to_string(),
                        annotations: None,
                        labels: None,
                    },
                    description: None,
                    link_selector: Some(BlueprintLinkSelector {
                        by_label: HashMap::from([(
                            "application".to_string(),
                            "orders".to_string()
                        )])
                    }),
                    spec: CelerityResourceSpec::Api(CelerityApiSpec {
                        protocols: vec![CelerityApiProtocol::Http],
                        cors: Some(CelerityApiCors::CorsConfiguration(
                            CelerityApiCorsConfiguration {
                                allow_credentials: Some(true),
                                allow_origins: Some(vec![
                                    "https://example.com".to_string(),
                                    "https://another.example.com".to_string()
                                ]),
                                allow_methods: Some(vec!["GET".to_string(), "POST".to_string()]),
                                allow_headers: Some(vec![
                                    "Content-Type".to_string(),
                                    "Authorization".to_string()
                                ]),
                                expose_headers: Some(vec!["Content-Length".to_string()]),
                                max_age: Some(3600),
                            }
                        )),
                        domain: Some(CelerityApiDomain {
                            domain_name: "api.example.com".to_string(),
                            base_paths: vec!["/".to_string()],
                            normalize_base_path: Some(false),
                            certificate_id: "${variables.certificateId}".to_string(),
                            security_policy: Some(CelerityApiDomainSecurityPolicy::Tls1_2),
                        }),
                        tracing_enabled: Some(true),
                        auth: Some(CelerityApiAuth {
                            default_guard: Some("jwt".to_string()),
                            guards: HashMap::from([(
                                "jwt".to_string(),
                                CelerityApiAuthGuard {
                                    guard_type: CelerityApiAuthGuardType::Jwt,
                                    issuer: Some(
                                        "https://identity.twohundred.cloud/oauth2/v1/".to_string()
                                    ),
                                    token_source: Some(CelerityApiAuthGuardValueSource::Str(
                                        "$.headers.Authorization".to_string()
                                    )),
                                    audience: Some(vec![
                                        "https://identity.twohundred.cloud/api/manage/v1/"
                                            .to_string()
                                    ]),
                                    api_key_source: None,
                                }
                            )]),
                        })
                    })
                }
            )]),
        }
    );
}
