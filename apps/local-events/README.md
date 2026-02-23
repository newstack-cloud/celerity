# Celerity Local Events

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=newstack-cloud_celerity-local-events&metric=coverage)](https://sonarcloud.io/summary/new_code?id=newstack-cloud_celerity-local-events)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=newstack-cloud_celerity-local-events&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=newstack-cloud_celerity-local-events)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=newstack-cloud_celerity-local-events&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=newstack-cloud_celerity-local-events)

Celerity Local Events is a sidecar application that can be used to capture events from numerous sources and forward them to applications running locally in the Celerity runtime.

This is a key component that enables fully integrated testing of reactive Celerity applications in local and CI environments.

This helps with testing the contract between the developer's application and the Celerity framework, it won't test vendor-specific configurations, Celerity is expected to make sure that the contract in terms of the vendor-agnsotic API for configuration and implementing handlers is consistent regardless of the deployment target.

The sidecar provides the following features:

- **Topic Bridge** — Subscribes to a Valkey pub/sub channel and fans out each published message to one or more Valkey streams, enabling local testing of pub/sub-triggered handlers.
- **Schedule Trigger** — Evaluates cron and rate schedule expressions (AWS EventBridge format) and writes trigger events to Valkey streams on each tick, enabling local testing of scheduled handlers.
- **DynamoDB Stream Bridge** — Polls DynamoDB Local for stream records (inserts, modifications, deletions) and writes change events to Valkey streams, enabling local testing of DynamoDB stream-triggered handlers.
- **MinIO Notification Bridge** — Listens for MinIO bucket notifications (object created, removed, etc.) and writes events to Valkey streams, enabling local testing of S3 event-triggered handlers.

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
