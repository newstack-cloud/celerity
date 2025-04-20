package auth

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

const (
	// 2nd October 2024 19:00:52 UTC
	testTimestamp int64 = 1727895652
)

type testClock struct {
	timestamp int64
}

func (c *testClock) Now() time.Time {
	return time.Unix(c.timestamp, 0)
}

func loadPrivateKey(keyName string) (jwk.Key, error) {
	privateKeyData, err := os.ReadFile(fmt.Sprintf("__testdata/jwt/%s.json", keyName))
	if err != nil {
		return nil, err
	}
	return jwk.ParseKey(privateKeyData)
}

func createToken(
	privateKey jwk.Key,
	subject string,
	issuer string,
	audience []string,
	kid string,
	customClaims map[string]any,
) (string, error) {
	privateKeyRaw := &rsa.PrivateKey{}
	err := privateKey.Raw(privateKeyRaw)
	if err != nil {
		return "", nil
	}
	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       privateKeyRaw,
		},
		(&jose.SignerOptions{
			ExtraHeaders: map[jose.HeaderKey]any{
				"kid": kid,
			},
		}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}
	claims := jwt.Claims{
		Subject:  subject,
		Issuer:   issuer,
		Audience: jwt.Audience(audience),
	}
	return jwt.Signed(sig).Claims(claims).Claims(customClaims).CompactSerialize()
}
