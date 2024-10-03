/// A key pair for signing and verifying messages.
#[derive(Clone)]
pub struct KeyPair {
    // A public key ID used to identify the key pair.
    pub key_id: String,
    // A secret key that is used to sign and verify messages.
    pub secret_key: String,
}

/// The components of a signature extracted from a header.
pub struct SignatureParts {
    // The public key ID used to identify the key pair.
    pub key_id: String,
    // The signature of the message.
    pub signature: String,
    // The headers that were included in the signature.
    pub headers: Vec<String>,
}
