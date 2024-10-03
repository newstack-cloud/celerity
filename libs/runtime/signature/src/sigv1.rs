use std::collections::HashMap;

use base64::{prelude::BASE64_URL_SAFE_NO_PAD, Engine};
use celerity_helpers::time::Clock;
use hmac::{Hmac, Mac};
use http::HeaderMap;
use sha2::Sha256;

use crate::{
    errors::{SignatureMessageCreationError, SignatureVerificationError},
    types::{KeyPair, SignatureParts},
};

/// The name of the signature header used for Celerity Signature v1.
pub const SIGNATURE_HEADER_NAME: &str = "Celerity-Signature-V1";

// The name of the header used to store the data as a UNIX timestamp in seconds.
pub const DATE_HEADER_NAME: &str = "Celerity-Date";

/// The default clock skew in seconds.
pub const DEFAULT_CLOCK_SKEW: u64 = 300;

/// Verify a signature header with
/// [Celerity Signature v1](https://www.celerityframework.com/docs/auth/signature-v1).
///
/// # Errors
///
/// This function will return an error if signature verification fails
/// for reasons such as an invalid signature or missing headers.
///
/// # Examples
///
/// ```
/// # use celerity_signature::sigv1::*;
/// # use celerity_helpers::time::DefaultClock;
///
/// let clock = DefaultClock::new();
/// // Uses the default clock skew of 5 minutes.
/// match verify_signature(key_pairs, headers, clock, None) {
///     Ok(_) => println!("Signature verified"),
///     Err(e) => eprintln!("Signature verification failed: {:?}", e),
/// }
///
/// ```
pub fn verify_signature(
    key_pairs: &HashMap<String, KeyPair>,
    headers: &HeaderMap,
    clock: &impl Clock,
    clock_skew: Option<u64>,
) -> Result<(), SignatureVerificationError> {
    let signature_header = match headers.get(SIGNATURE_HEADER_NAME) {
        Some(signature) => signature
            .to_str()
            .map(|header_str| header_str.to_string())?,
        None => return Err(SignatureVerificationError::SignatureHeadingMissing),
    };
    let signature_parts = unpack_signature(signature_header)?;
    let key_pair = key_pairs.get(&signature_parts.key_id).ok_or(
        SignatureVerificationError::InvalidSignature("Invalid key ID".to_string()),
    )?;
    let message = create_message(key_pair, &headers, &signature_parts.headers)?;

    match verify_message(
        key_pair,
        message.as_slice(),
        signature_parts.signature.as_str(),
    ) {
        Ok(_) => {
            let final_clock_skew = clock_skew.unwrap_or(DEFAULT_CLOCK_SKEW);
            let current_time = clock.now();
            let provided_date = extract_date_from_header(&headers)?;
            if current_time > provided_date + final_clock_skew
                || current_time < provided_date - final_clock_skew
            {
                return Err(SignatureVerificationError::InvalidSignature(
                    "Signature has expired".to_string(),
                ));
            }
            Ok(())
        }
        Err(err) => Err(err),
    }
}

/// Creates a signature header value to be attached to a request
/// for [Celerity Signature v1](https://www.celerityframework.com/docs/auth/signature-v1).
/// This function will return the value of the signature header that should be
/// set in the "Celerity-Signature-V1" header of the request.
///
/// The `Celerity-Date` header does not need to be set in the provided headers,
/// as it will be automatically added to the signature message using the provided clock
/// and inserted into the provided mutable headers map.
///
/// # Errors
///
/// This function will return an error if there are missing or invalid
/// custom headers in the provided request headers.
///
/// # Examples
///
/// ```
/// # use celerity_signature::sigv1::*;
/// # use celerity_helpers::time::DefaultClock;
/// # use http::HeaderMap;
///
/// let key_pair = KeyPair {
///    key_id: "key-id".to_string(),    
///    secret_key: "secret-key".to_string()
/// };
/// let headers = HeaderMap::new();
/// headers.insert("X-Custom-Header", "custom-value".parse().unwrap());
/// let custom_header_names = vec!["X-Custom-Header".to_string()];
/// let clock = DefaultClock::new();
/// let signature_header = create_signature_header(key_pair, headers, custom_header_names, clock)
///     .expect("signature header to be created without any issues");
///
/// assert!(signature_header.starts_with("key-id"));
/// assert!(headers.get("Celerity-Date").is_some());
/// ```
pub fn create_signature_header(
    key_pair: &KeyPair,
    headers: &mut HeaderMap,
    custom_header_names: Vec<String>,
    clock: &impl Clock,
) -> Result<String, SignatureMessageCreationError> {
    if headers.get(DATE_HEADER_NAME).is_none() {
        let date = clock.now();
        headers.insert(
            DATE_HEADER_NAME,
            date.to_string()
                .parse()
                .expect("timestamp string should be a valid header value"),
        );
    }
    let message = create_message(&key_pair, &headers, &custom_header_names)?;
    let signature = sign_message(&key_pair, message.as_slice());
    let mut final_header_names = vec![DATE_HEADER_NAME.to_string().to_lowercase()];
    final_header_names.extend(
        custom_header_names
            .iter()
            .map(|header| header.to_lowercase()),
    );
    let signature_header_names = final_header_names.join(" ");
    Ok(format!(
        "keyId=\"{}\", headers=\"{}\", signature=\"{}\"",
        key_pair.key_id, signature_header_names, signature
    ))
}

fn unpack_signature(
    signature_header: String,
) -> Result<SignatureParts, SignatureVerificationError> {
    let parts: Vec<&str> = signature_header.split(',').collect();
    if parts.len() != 3 {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid signature header format".to_string(),
        ));
    }

    let key_id = unpack_key_id(parts[0])?;
    let headers = unpack_headers(parts[1])?;
    let signature = unpack_signature_value(parts[2])?;
    Ok(SignatureParts {
        key_id,
        signature,
        headers,
    })
}

fn unpack_key_id(key_id_header_part: &str) -> Result<String, SignatureVerificationError> {
    let key_id_parts: Vec<&str> = key_id_header_part.split('=').collect();
    if key_id_parts.len() != 2 {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid key ID header part format".to_string(),
        ));
    }

    if key_id_parts[0].trim() != "keyId" {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid key ID header part format".to_string(),
        ));
    }

    // Remove the quotes around the key ID.
    let key_id = key_id_parts[1][1..key_id_parts[1].len() - 1].to_string();
    Ok(key_id)
}

fn unpack_headers(header_parts: &str) -> Result<Vec<String>, SignatureVerificationError> {
    let headers_parts: Vec<&str> = header_parts.split('=').collect();
    if headers_parts.len() != 2 {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid \"headers\" header part format".to_string(),
        ));
    }

    if headers_parts[0].trim() != "headers" {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid \"headers\" header part format".to_string(),
        ));
    }

    // Remove the quotes around the headers.
    let headers = headers_parts[1][1..headers_parts[1].len() - 1]
        .split(' ')
        .map(|header| header.to_string())
        .collect();
    Ok(headers)
}

fn unpack_signature_value(signature_part: &str) -> Result<String, SignatureVerificationError> {
    let signature_parts: Vec<&str> = signature_part.split('=').collect();
    if signature_parts.len() != 2 {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid signature header part format".to_string(),
        ));
    }

    if signature_parts[0].trim() != "signature" {
        return Err(SignatureVerificationError::InvalidSignature(
            "Invalid signature header part format".to_string(),
        ));
    }

    // Remove the quotes around the signature.
    let signature = signature_parts[1][1..signature_parts[1].len() - 1].to_string();
    Ok(signature)
}

fn create_message(
    key_pair: &KeyPair,
    headers: &HeaderMap,
    custom_header_names: &Vec<String>,
) -> Result<Vec<u8>, SignatureMessageCreationError> {
    let date = extract_date_from_header(headers)?;
    let custom_headers = custom_header_names
        .iter()
        .filter(|header_name| header_name.as_str() != DATE_HEADER_NAME.to_lowercase())
        .map(|header_name| match headers.get(header_name) {
            Some(header) => {
                let header_value = header
                    .to_str()
                    .map_err(|_| SignatureMessageCreationError::InvalidCustomHeader)?;
                // Normalise all header names to lower case to avoid mismatch due to header name
                // case differences.
                Ok(format!(",{}={}", header_name.to_lowercase(), header_value))
            }
            None => Err(SignatureMessageCreationError::CustomHeadersMissing(vec![
                header_name.clone(),
            ])),
        })
        .collect::<Result<String, SignatureMessageCreationError>>()?;
    let message = format!(
        "{},celerity-date={}{}",
        key_pair.key_id, date, custom_headers
    );
    Ok(message.as_bytes().to_vec())
}

fn sign_message(key_pair: &KeyPair, message: &[u8]) -> String {
    let mut mac = Hmac::<Sha256>::new_from_slice(key_pair.secret_key.as_bytes())
        .expect("HMAC can take key of any size");
    mac.update(message);
    let result = mac.finalize();
    let code_bytes = result.into_bytes();
    BASE64_URL_SAFE_NO_PAD.encode(&code_bytes[..])
}

fn verify_message(
    key_pair: &KeyPair,
    message: &[u8],
    signature: &str,
) -> Result<(), SignatureVerificationError> {
    let signature_bytes = BASE64_URL_SAFE_NO_PAD
        .decode(signature.as_bytes())
        .map_err(|_| {
            SignatureVerificationError::InvalidSignature("Invalid signature".to_string())
        })?;
    let mut mac = Hmac::<Sha256>::new_from_slice(key_pair.secret_key.as_bytes())
        .expect("HMAC can take key of any size");
    mac.update(message);
    mac.verify_slice(&signature_bytes)
        .map_err(|_| SignatureVerificationError::InvalidSignature("Invalid signature".to_string()))
}

fn extract_date_from_header(headers: &HeaderMap) -> Result<u64, SignatureVerificationError> {
    match headers.get(DATE_HEADER_NAME) {
        Some(date_header) => {
            let date_str = date_header.to_str().map_err(|_| {
                SignatureVerificationError::InvalidSignature("Invalid date header".to_string())
            })?;
            date_str.parse::<u64>().map_err(|_| {
                SignatureVerificationError::InvalidSignature("Invalid date".to_string())
            })
        }
        None => Err(SignatureVerificationError::DateHeaderMissing),
    }
}

#[cfg(test)]
mod tests {

    use super::*;
    use celerity_helpers::time::Clock;
    use http::HeaderMap;
    use pretty_assertions::assert_eq;

    // 2nd October 2024 19:00:52 UTC
    const TEST_TIMESTAMP: u64 = 1727895652;

    struct TestClock {
        now: u64,
    }

    impl Clock for TestClock {
        fn now(&self) -> u64 {
            self.now
        }
    }

    #[test]
    fn test_create_signature_header() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_pair = KeyPair {
            key_id: "test-key-id".to_string(),
            secret_key: "test-secret_key".to_string(),
        };
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header =
            create_signature_header(&key_pair, &mut headers, custom_header_names, &clock)
                .expect("signature header to be created without any issues");

        assert_eq!(
            signature_header,
            "keyId=\"test-key-id\", headers=\"celerity-date x-custom-header\", signature=\"ppBsB6jEDm48SoYcXmfpu-IWshzWI5S8b_MmLDXFy_4\""
        );
    }

    #[test]
    fn test_returns_expected_error_when_custom_header_is_missing() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_pair = KeyPair {
            key_id: "test-key-id".to_string(),
            secret_key: "test-secret_key".to_string(),
        };
        let mut headers = HeaderMap::new();
        // Custom header not set in the headers.

        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let result =
            create_signature_header(&key_pair, &mut headers, custom_header_names.clone(), &clock);
        assert!(matches!(
            result,
            Err(SignatureMessageCreationError::CustomHeadersMissing(_))
        ));

        let reported_missing_headers = match result {
            Err(SignatureMessageCreationError::CustomHeadersMissing(headers)) => headers,
            _ => vec![],
        };
        assert_eq!(reported_missing_headers, custom_header_names);
    }

    #[test]
    fn test_verify_valid_signature_header() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &headers, &clock, None);
        assert!(result.is_ok());
    }

    #[test]
    fn test_verify_valid_signature_header_with_time_difference_within_skew_1() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let clock2 = TestClock {
            // -3 minutes from the original timestamp,
            // but within the default clock skew of 5 minutes.
            now: TEST_TIMESTAMP - 180,
        };
        let key_id = "test-key-id".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &headers, &clock2, None);
        assert!(result.is_ok());
    }

    #[test]
    fn test_verify_valid_signature_header_with_time_difference_within_skew_2() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let clock2 = TestClock {
            // +4 minutes from the original timestamp,
            // but within the default clock skew of 5 minutes.
            now: TEST_TIMESTAMP + 240,
        };
        let key_id = "test-key-id-2".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-2".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-2".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &headers, &clock2, None);
        assert!(result.is_ok());
    }

    #[test]
    fn test_fails_verifying_signature_that_has_expired() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let clock2 = TestClock {
            // +6 minutes, beyond the default clock skew of 5 minutes.
            now: TEST_TIMESTAMP + 360,
        };
        let key_id = "test-key-id-3".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-3".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-3".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &headers, &clock2, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Signature has expired"
        ));
    }

    #[test]
    fn test_fails_verifying_signature_for_invalid_key_id() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id-4".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-4".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-4".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        let invalid_signature_header = signature_header.replace(&key_id, "invalid-key-id");

        headers.insert(
            SIGNATURE_HEADER_NAME,
            invalid_signature_header.parse().unwrap(),
        );
        let result = verify_signature(&key_pairs, &headers, &clock, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Invalid key ID"
        ));
    }

    #[test]
    fn test_fails_verifying_signature_for_invalid_signature_signed_with_a_different_secret_key() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id-4".to_string();
        // Key pairs with the same key ID but different secret keys are used
        // to sign and verify the message to test expected error.
        let sign_key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-other_secret_key".to_string(),
            },
        )]);
        let verify_key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-4".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-4".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &sign_key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&verify_key_pairs, &headers, &clock, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Invalid signature"
        ));
    }

    #[test]
    fn test_fails_verifying_signature_for_invalid_signature_due_to_date_header_mismatch() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id-4".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-4".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-4".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        let mut other_headers = headers.clone();
        other_headers.insert(
            DATE_HEADER_NAME,
            // A different date header value is set to test expected error.
            format!("{}", TEST_TIMESTAMP + 60).parse().unwrap(),
        );
        other_headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &other_headers, &clock, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Invalid signature"
        ));
    }

    #[test]
    fn test_fails_verifying_signature_for_invalid_signature_due_to_custom_header_mismatch() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id-4".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-4".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-4".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");

        let mut other_headers = headers.clone();
        other_headers.insert(
            "X-Custom-Header",
            // A different custom header value is set to test expected error.
            "custom-value-5".parse().unwrap(),
        );
        other_headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &other_headers, &clock, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Invalid signature"
        ));
    }

    #[test]
    fn test_fails_verifying_signature_for_invalid_signature_that_is_not_a_base64_encoded_string() {
        let clock = TestClock {
            now: TEST_TIMESTAMP,
        };
        let key_id = "test-key-id-4".to_string();
        let key_pairs = HashMap::from([(
            key_id.clone(),
            KeyPair {
                key_id: key_id.clone(),
                secret_key: "test-secret_key-4".to_string(),
            },
        )]);
        let mut headers = HeaderMap::new();
        headers.insert("X-Custom-Header", "custom-value-4".parse().unwrap());
        let custom_header_names = vec!["X-Custom-Header".to_string()];

        let mut signature_header = create_signature_header(
            &key_pairs[&key_id],
            &mut headers,
            custom_header_names,
            &clock,
        )
        .expect("signature header to be created without any issues");
        signature_header = signature_header.replace("signature=\"", "signature=\"invalid");

        headers.insert(SIGNATURE_HEADER_NAME, signature_header.parse().unwrap());
        let result = verify_signature(&key_pairs, &headers, &clock, None);
        assert!(matches!(
            result,
            Err(SignatureVerificationError::InvalidSignature(msg)) if msg == "Invalid signature"
        ));
    }
}
