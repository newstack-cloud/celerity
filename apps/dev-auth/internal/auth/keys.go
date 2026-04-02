package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
)

const keyID = "dev-key-1"

func generateKeyPair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// jwk represents a JSON Web Key for an RSA public key.
type jwk struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

func publicKeyToJWK(pub *rsa.PublicKey) jwk {
	return jwk{
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		Kid: keyID,
		N:   base64URLEncode(pub.N.Bytes()),
		E:   base64URLEncode(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
