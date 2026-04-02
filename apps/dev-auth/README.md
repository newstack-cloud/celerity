# Celerity Dev Auth

A lightweight OIDC provider for local development and testing. It generates RSA-signed JWTs on demand with customisable claims, exposing just enough of the OpenID Connect surface for Celerity runtime applications to validate tokens without an external identity provider.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/.well-known/openid-configuration` | OIDC discovery document |
| `GET` | `/.well-known/jwks.json` | JSON Web Key Set (public key) |
| `POST` | `/token` | Issue a signed JWT |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `9099` | Server listen port |
| `DEV_AUTH_ISSUER` | `http://host.docker.internal:<PORT>` | `iss` claim in issued tokens |
| `DEV_AUTH_AUDIENCE` | `celerity-test-app` | `aud` claim in issued tokens |
| `LOG_LEVEL` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |

## Token Request

```bash
curl -X POST http://localhost:9099/token \
  -H 'Content-Type: application/json' \
  -d '{
    "sub": "user-123",
    "claims": { "role": "admin" },
    "expiresIn": "2h"
  }'
```

### Request Body

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `sub` | yes | — | Subject claim |
| `claims` | no | `{}` | Additional JWT claims (reserved claims `iss`, `aud`, `sub`, `iat`, `exp` are ignored) |
| `expiresIn` | no | `1h` | Token lifetime as a Go duration string (e.g. `30m`, `2h`) |

### Response

```json
{
  "access_token": "<signed JWT>",
  "token_type": "Bearer",
  "expires_in": "2h"
}
```

## Docker

```bash
docker build -t celerity-dev-auth .
docker run -p 9099:9099 celerity-dev-auth
```

## Linting

```bash
# Requires staticcheck: go install honnef.co/go/tools/cmd/staticcheck@latest
bash scripts/lint.sh
```
