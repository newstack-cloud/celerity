# Workflow API

The Celerity Workflow API allows for triggering and monitoring the workflow along with the ability to retrieve workflow execution history.

## Authentication

API calls to a workflow application must be authenticated with an [API key](https://celerityframework.io/docs/auth/api-keys), [Celerity Signature v1](https://celerityframework.io/docs/auth/signature-v1) or a [JWT access token issued by an OAuth2/OpenID connect provider](https://celerityframework.io/docs/auth/jwts).

An OAuth2/OpenID Connect provider must publish a JWKS (JSON Web Key Set) for the public key used to verify the JWT signature in the `{issuer}/.well-known/openid-configuration` or `{issuer}/.well-known/oauth-authorization-server` endpoint.

Public clients such as web applications running in a browser should **not** interact with the workflow API directly.

When you provision a new workflow application, the default behaviour is to issue an API key pair for the application that is used to authenticate API calls with the Celerity Signature v1.

## API Specifications

[workflow-api-v1](./workflow-api-v1.yaml) - The Celerity Workflow API v1 specification.
[workflow-stream-api-v1](./workflow-stream-api-v1.yaml) - The Celerity Workflow Stream API v1 specification for streaming events for a current workflow execution.
