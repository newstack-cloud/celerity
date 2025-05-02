# OIDC Local Server

A simple local server for testing OIDC-issued JWT authentication for the deploy engine.
This server is prepared to run in a Docker container as a part of the deploy engine local development compose stack.
It needs to run on a container that is reachable from the deploy engine container via the DNS alias `oidc-local-server` on port `80`.
