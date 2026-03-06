# Local Events Bridge Architecture

The local-events sidecar bridges event sources to the Celerity runtime's consumer infrastructure during local and CI development. Each bridge type captures events from a specific source technology and writes them to Valkey streams, where the runtime's stream consumers pick them up — exactly as they would in a cloud deployment.

## Overview

```
Event Sources                   local-events sidecar              Runtime Consumers
─────────────                   ────────────────────              ─────────────────

Valkey pub/sub channel ───────> Topic Bridge ──────────────────> Valkey stream(s)
                                (fan-out to N targets)

Cron / rate expressions ──────> Schedule Trigger ──────────────> Valkey stream
                                (evaluates on tick)

DynamoDB Local table ─────────> DynamoDB Stream Bridge ────────> Valkey stream
                                (polls for change records)

MinIO bucket ─────────────────> MinIO Notification Bridge ─────> Valkey stream
                                (listens for object events)
```

All bridges converge on Valkey streams as the single delivery mechanism. The runtime consumes from streams using `XREADGROUP`, regardless of the original event source. This mirrors cloud deployments where SNS, EventBridge, DynamoDB Streams, and S3 events all ultimately deliver to the runtime's consumer layer (SQS queues, Lambda event source mappings, etc.).

## Shared Infrastructure

A single Valkey instance (managed by the Celerity CLI) is shared across all bridges, the SDK's queue and topic packages, and the runtime's consumer layer. All components connect via the `CELERITY_LOCAL_REDIS_URL` environment variable (default: `redis://127.0.0.1:6379`).

## Configuration

All bridges are configured via a single JSON file (default: `/etc/celerity/local-events-config.json`, override with `CELERITY_LOCAL_EVENTS_CONFIG_FILE`). The file contains an array of bridge entries, each with a `type` field that determines the configuration shape:

```json
[
  { "type": "topic_bridge", ... },
  { "type": "schedule", ... },
  { "type": "dynamodb_stream", ... },
  { "type": "minio_notification", ... }
]
```

The Celerity CLI generates this file from the application's blueprint resource definitions. Developers do not typically edit it directly.

## Common Stream Record Format

All bridges write records to Valkey streams using `XADD`. Every record contains at least these fields:

| Field | Description |
|---|---|
| `body` | The event payload (JSON string) |
| `timestamp` | Unix seconds at the time the bridge writes the record |
| `message_type` | Always `"0"` (reserved for future use) |

Some bridges add additional fields depending on the event source (see per-bridge sections below).

---

## Topic Bridge

**Type:** `topic_bridge`
**Source:** Valkey pub/sub channel
**Cloud equivalent:** SNS topic -> SQS subscription(s)

### Purpose

Bridges `celerity/topic` publish operations to consumer streams. The SDK's Redis topic provider publishes a JSON envelope to a Valkey pub/sub channel. The bridge subscribes to that channel and fans out each message to one or more target streams — one per subscribing consumer or workflow. This mirrors SNS delivering to multiple SQS subscriptions.

### Data Flow

```
SDK (publish)                  Topic Bridge                    Runtime Consumer
     |                              |                              |
     |  PUBLISH channel envelope    |                              |
     |----------------------------->|                              |
     |                              |  parse JSON envelope         |
     |                              |                              |
     |                              |  XADD stream_A * fields...   |
     |                              |----------------------------->|  (consumer A)
     |                              |                              |
     |                              |  XADD stream_B * fields...   |
     |                              |----------------------------->|  (consumer B)
```

### SDK Envelope Format

The SDK publishes a structured JSON envelope to the pub/sub channel:

```json
{
  "body": "{\"orderId\":\"123\"}",
  "messageId": "550e8400-e29b-41d4-a716-446655440000",
  "subject": "OrderCreated",
  "attributes": { "env": "prod", "region": "us-east-1" }
}
```

The bridge parses this envelope and writes individual fields to each target stream:

```
XADD stream * body '{"orderId":"123"}' timestamp 1709740800 message_type 0 message_id 550e8400-... subject OrderCreated attributes '{"env":"prod","region":"us-east-1"}'
```

### Stream Record Fields

| Field | Source | Presence |
|---|---|---|
| `body` | Extracted from envelope `body` | Always |
| `timestamp` | Bridge wall clock (Unix seconds) | Always |
| `message_type` | Constant `"0"` | Always |
| `message_id` | Extracted from envelope `messageId` (SDK-generated UUID) | If present |
| `subject` | Extracted from envelope `subject` | If present |
| `attributes` | Extracted from envelope `attributes` (re-serialized as JSON) | If non-empty |

### Fallback Behavior

If the payload is not a valid JSON envelope (e.g. missing `body` field or invalid JSON), the bridge falls back to writing the raw payload as the `body` field. This provides robustness during development and debugging.

### Configuration

```json
{
  "type": "topic_bridge",
  "source": { "channel": "orders-topic" },
  "targets": [
    { "stream": "order-processor-stream" },
    { "stream": "order-analytics-stream" }
  ]
}
```

### Delivery Guarantees

- **Pub/sub leg** (SDK -> channel): At-most-once. If the bridge isn't subscribed when a message is published, it is lost.
- **Stream leg** (channel -> consumer streams): At-least-once. Streams are persistent append-only logs with consumer group acknowledgement.

### Runtime Message Attributes

The Rust runtime's Redis consumer extracts the topic envelope fields from the stream record and exposes them to handlers as `messageAttributes` in the same `{ dataType, stringValue }` format used by the SQS consumer. This means handler code works identically on cloud (SNS -> SQS) and locally (topic -> bridge -> stream).

| Stream Field | Handler Attribute Key | Description |
|---|---|---|
| `message_id` | `sourceMessageId` | SDK-generated UUID for cross-service log correlation |
| `subject` | `subject` | Message subject from the publisher |
| `attributes` | _(expanded)_ | Each key-value pair becomes its own attribute entry |

Example `messageAttributes` as seen by a handler:

```json
{
  "sourceMessageId": { "dataType": "String", "stringValue": "550e8400-..." },
  "subject": { "dataType": "String", "stringValue": "OrderCreated" },
  "env": { "dataType": "String", "stringValue": "prod" },
  "region": { "dataType": "String", "stringValue": "us-east-1" }
}
```

`ConsumerMessage.messageId` remains the **Redis stream ID** (e.g. `1709740800123-0`), which is unique per consumer stream and used internally for locking, checkpointing, and partial-failure reporting. The SDK-generated topic message ID is available as `sourceMessageId` for log correlation across service boundaries.

### Cross-Service Correlation

When multiple services communicate via topics, the `sourceMessageId` attribute enables end-to-end log correlation:

```
Service A (publisher)              Bridge                Service B (consumer)
     |                               |                        |
     |  publish({ orderId: "123" })  |                        |
     |  → messageId: "abc-def-..."   |                        |
     |  → log: [abc-def] published   |                        |
     |                               |                        |
     |  PUBLISH channel envelope     |                        |
     |------------------------------>|                        |
     |                               |  XADD stream ...       |
     |                               |----------------------->|
     |                               |                        |  handler receives:
     |                               |                        |  sourceMessageId = "abc-def-..."
     |                               |                        |  → log: [abc-def] processing
```

Both services can log with the same `sourceMessageId`, enabling developers to trace a message's journey through the system in local dev logs.

### Ordering

The bridge runs as a single goroutine per topic channel. Messages are processed sequentially from the subscription, and `fanOut()` iterates targets sequentially. This provides global message ordering — stronger than per-group FIFO on cloud providers. FIFO options (`groupId`, `deduplicationId`) are unnecessary locally and not included in the envelope.

---

## Schedule Trigger

**Type:** `schedule`
**Source:** Cron or rate expression (evaluated locally)
**Cloud equivalent:** EventBridge Scheduler -> SQS / Lambda

### Purpose

Evaluates schedule expressions and writes trigger events to Valkey streams on each tick. This enables local testing of scheduled handlers without waiting for cloud infrastructure.

### Data Flow

```
Schedule Trigger (internal timer)           Runtime Consumer
     |                                           |
     |  (cron/rate tick fires)                   |
     |                                           |
     |  XADD stream * body '{"scheduleId":...}' |
     |------------------------------------------>|
```

### Schedule Expression Formats

**Rate expressions** follow the AWS EventBridge format:

```
rate(5 minutes)
rate(1 hour)
rate(7 days)
```

Parsed via regex: `^rate\((\d+)\s+(minutes?|hours?|days?)\)$`. Implemented with `time.Ticker`.

**Cron expressions** follow the AWS EventBridge 6-field format:

```
cron(0 12 * * ? *)
cron(15 10 ? * 6L 2025)
```

Format: `minutes hours day-of-month month day-of-week year`. The bridge converts `?` to `*` and strips the year field (6th) for compatibility with `robfig/cron/v3`.

### Stream Record Fields

| Field | Source | Presence |
|---|---|---|
| `body` | JSON: `{"scheduleId", "scheduledTime", "input"}` | Always |
| `timestamp` | Bridge wall clock (Unix seconds) | Always |
| `message_type` | Constant `"0"` | Always |

The `body` payload contains:

```json
{
  "scheduleId": "daily-cleanup",
  "scheduledTime": "2026-03-06T12:00:00Z",
  "input": { "retentionDays": 30 }
}
```

- `scheduleId` — the configured schedule entry ID
- `scheduledTime` — RFC 3339 UTC timestamp of the trigger
- `input` — arbitrary JSON data from the schedule configuration, passed through to the handler

### Configuration

```json
{
  "type": "schedule",
  "schedules": [
    {
      "id": "daily-cleanup",
      "schedule": "rate(1 day)",
      "stream": "cleanup-handler-stream",
      "input": { "retentionDays": 30 }
    },
    {
      "id": "hourly-sync",
      "schedule": "cron(0 * * * ? *)",
      "stream": "sync-handler-stream",
      "input": null
    }
  ]
}
```

Each schedule entry spawns its own goroutine. Multiple entries can target the same or different streams.

### Behavior Notes

- Rate schedules fire at fixed intervals from startup (not aligned to wall clock).
- Cron schedules fire at the next matching wall-clock time after the previous tick.
- Unrecognised expressions are logged as errors and skipped.

---

## DynamoDB Stream Bridge

**Type:** `dynamodb_stream`
**Source:** DynamoDB Local table stream
**Cloud equivalent:** DynamoDB Streams -> Lambda event source mapping

### Purpose

Polls DynamoDB Local for stream records (inserts, modifications, deletions) and writes change events to a Valkey stream. This enables local testing of DynamoDB stream-triggered handlers.

### Data Flow

```
DynamoDB Local                 DynamoDB Stream Bridge           Runtime Consumer
     |                              |                              |
     |  (table mutation)            |                              |
     |                              |  DescribeTable (get ARN)     |
     |<-----------------------------|                              |
     |                              |  DescribeStream (get shards) |
     |<-----------------------------|                              |
     |                              |  GetRecords (poll shard)     |
     |<-----------------------------|                              |
     |  [stream records]            |                              |
     |----------------------------->|                              |
     |                              |  XADD stream * body ...      |
     |                              |----------------------------->|
```

### Polling Architecture

1. **`waitForStreamARN()`** — Polls `DescribeTable` every 2 seconds until the table has an active stream ARN. This handles the case where the table is created after the bridge starts.
2. **`pollShards()`** — Outer loop that discovers shards via `DescribeStream` and polls each one.
3. **`pollShard()`** — Per-shard loop using `GetRecords` at 1-second intervals. Refreshes the shard iterator every 14 minutes (DynamoDB iterators expire after 15 minutes).
4. **`writeRecord()`** — JSON-marshals the full DynamoDB stream record and writes it to the target Valkey stream.

### Stream Record Fields

| Field | Source | Presence |
|---|---|---|
| `body` | Full DynamoDB stream record (JSON-marshaled) | Always |
| `timestamp` | Bridge wall clock (Unix seconds) | Always |
| `message_type` | Constant `"0"` | Always |
| `event_name` | `INSERT`, `MODIFY`, or `REMOVE` | Always |

### Configuration

```json
{
  "type": "dynamodb_stream",
  "source": {
    "endpoint": "http://localhost:8000",
    "region": "us-east-1",
    "tableName": "orders"
  },
  "target": { "stream": "orders-change-stream" }
}
```

### Behavior Notes

- Uses `TRIM_HORIZON` shard iterator type (reads from the beginning of the shard).
- DynamoDB Local typically has a single shard per table.
- Static AWS credentials (`local`/`local`) are used — no real AWS authentication needed.
- If `DescribeStream` fails, retries with 5-second backoff.
- If `GetRecords` fails or the shard closes (nil `NextShardIterator`), the poller restarts shard discovery.

---

## MinIO Notification Bridge

**Type:** `minio_notification`
**Source:** MinIO bucket notifications
**Cloud equivalent:** S3 Event Notifications -> SQS / Lambda

### Purpose

Listens for MinIO bucket notifications (object created, removed, etc.) and writes events to a Valkey stream. This enables local testing of S3 event-triggered handlers.

### Data Flow

```
MinIO                          MinIO Notification Bridge        Runtime Consumer
     |                              |                              |
     |  (object PUT/DELETE)         |                              |
     |                              |  ListenBucketNotification    |
     |<-----------------------------|                              |
     |  [notification event]        |                              |
     |----------------------------->|                              |
     |                              |  XADD stream * body ...      |
     |                              |----------------------------->|
```

### Stream Record Fields

| Field | Source | Presence |
|---|---|---|
| `body` | Full MinIO notification event (JSON-marshaled) | Always |
| `timestamp` | Bridge wall clock (Unix seconds) | Always |
| `message_type` | Constant `"0"` | Always |
| `event_name` | Event type string (e.g. `s3:ObjectCreated:Put`) | Always |

### Configuration

```json
{
  "type": "minio_notification",
  "source": {
    "endpoint": "http://localhost:9000",
    "accessKey": "minioadmin",
    "secretKey": "minioadmin",
    "bucketName": "uploads",
    "events": ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"]
  },
  "target": { "stream": "upload-processor-stream" }
}
```

### Reconnection

The listener reconnects with a 5-second backoff if the connection to MinIO drops or the notification channel closes unexpectedly. This handles MinIO restarts during development.

### Behavior Notes

- Uses MinIO's `ListenBucketNotification` API (long-polling HTTP connection).
- The `endpoint` is stripped of its `http://` or `https://` scheme before passing to the `minio-go` client, which expects a bare `host:port`.
- Each notification may contain multiple records (one per affected object); all are written individually.

---

## Application Lifecycle

The `app.Run()` function orchestrates all bridges:

1. Load bridge configuration from JSON file.
2. Connect to Valkey and verify connectivity (`PING`).
3. Spawn a goroutine for each configured bridge.
4. Block until the context is cancelled (`SIGTERM` / `SIGINT`).
5. Wait for all bridge goroutines to finish.
6. Close the Valkey connection.

Each bridge goroutine runs independently and blocks until the context is cancelled. Unknown bridge types are logged as warnings and skipped.
