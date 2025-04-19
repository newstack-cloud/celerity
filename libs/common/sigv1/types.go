package sigv1

// Kepair for signing and verifying messages.
type KeyPair struct {
	// A public key ID used to identify the key pair.
	KeyID string
	// A secret key that is used to sign and verify messages.
	SecretKey string
}

// SignatureParts contains the components of a signature
// extracted from a header.
type SignatureParts struct {
	// The public key ID used to identify the key pair.
	KeyID string
	// The signature of the message.
	Signature string
	// The headers that were included in the signature.
	Headers []string
}
