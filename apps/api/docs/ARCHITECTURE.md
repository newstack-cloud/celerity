# Architecture

The Celerity Deploy Engine API provides a HTTP API for managing Celerity applications and blueprints.

## Authentication

The API can be protected by an API Key or the OAuth2 client credentials grant type, when running on a developer's machine locally, an API key is generated and stored in the `~/.celerity/api` directory on the first run. The key is automatically picked up by the CLI and used for authentication when making requests to the local API.
