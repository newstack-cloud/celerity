//! Conformance vectors for the Celerity Binary Message Format.
//!
//! The format is a published wire contract, and it has two implementations that
//! have to lay out the same bytes for the same clients.
//!
//! This one frames what the runtime sends. The other is the Go SDK's, which
//! exists because Go reaches the runtime over IPC and has no in-process binding
//! to call — unlike the Node and Python SDKs, which hand framing to this crate
//! through their native bindings and so hold no implementation of their own.
//!
//! A frame that differs by a byte still parses, as something other than what was
//! meant, so nothing fails until a client reads a payload short by its own
//! headers. These vectors are what keeps the two honest, and the file is shared:
//! the copy in the Go SDK at `handler/testdata/` is asserted there against the
//! Go encoder. Change the two together.

use base64::prelude::*;
use celerity_helpers::websockets::{
    encode_binary_message, parse_binary_message, BinaryMessageParts,
};
use serde::Deserialize;

const VECTORS: &str = include_str!("fixtures/binary-frame-vectors.json");

#[derive(Deserialize)]
struct Vectors {
    encodes: Vec<EncodeVector>,
    refusals: Vec<RefusalVector>,
}

#[derive(Deserialize)]
struct EncodeVector {
    name: String,
    route: String,
    #[serde(rename = "messageId")]
    message_id: Option<String>,
    #[serde(rename = "requireAck")]
    require_ack: bool,
    #[serde(rename = "payloadBase64")]
    payload_base64: String,
    #[serde(rename = "framedBase64")]
    framed_base64: String,
}

#[derive(Deserialize)]
struct RefusalVector {
    name: String,
    route: String,
    #[serde(rename = "messageId")]
    message_id: Option<String>,
    #[serde(rename = "requireAck")]
    require_ack: bool,
}

fn vectors() -> Vectors {
    serde_json::from_str(VECTORS).expect("the shared vectors should parse")
}

#[test]
fn test_encoder_agrees_with_the_shared_vectors() {
    for vector in vectors().encodes {
        let payload = BASE64_STANDARD
            .decode(&vector.payload_base64)
            .expect("vector payload should be base64");
        let expected = BASE64_STANDARD
            .decode(&vector.framed_base64)
            .expect("vector frame should be base64");

        let framed = encode_binary_message(BinaryMessageParts {
            route: &vector.route,
            message_id: vector.message_id.as_deref(),
            require_ack: vector.require_ack,
            message: &payload,
        })
        .unwrap_or_else(|err| panic!("framing {} should succeed, got {err:?}", vector.name));

        assert_eq!(
            framed, expected,
            "the frame for {} differs from the shared vector, so this encoder and the \
             SDK's no longer lay out the same bytes",
            vector.name
        );
    }
}

#[test]
fn test_encoder_refuses_what_the_shared_vectors_refuse() {
    for vector in vectors().refusals {
        let result = encode_binary_message(BinaryMessageParts {
            route: &vector.route,
            message_id: vector.message_id.as_deref(),
            require_ack: vector.require_ack,
            message: &[],
        });

        assert!(
            result.is_err(),
            "framing {} should be refused, since truncating it into a frame would be read \
             as something other than what was meant",
            vector.name
        );
    }
}

// The vectors pin the bytes; this pins that those bytes mean what they were
// composed to mean, which a shared fixture alone cannot say.
#[test]
fn test_shared_vectors_round_trip_through_the_parser() {
    for vector in vectors().encodes {
        let payload = BASE64_STANDARD
            .decode(&vector.payload_base64)
            .expect("vector payload should be base64");
        let framed = BASE64_STANDARD
            .decode(&vector.framed_base64)
            .expect("vector frame should be base64");

        let parsed = parse_binary_message(&framed)
            .unwrap_or_else(|err| panic!("parsing {} should succeed, got {err:?}", vector.name));

        assert_eq!(
            parsed.message, payload,
            "the payload read back from {} is not the one framed",
            vector.name
        );
    }
}
