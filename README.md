# stoat-bridge

A webhook-to-chat bridge that receives alert webhooks from monitoring tools and delivers formatted messages to [Stoat](https://stoat.chat) chat channels.

## Features

- **Multiple webhook sources**: Grafana Alerting, Prometheus Alertmanager, Gatus
- **Auto-detection**: `/webhook` endpoint inspects payloads to route to the correct handler
- **Pluggable output**: Stoat is the first chat platform, extensible to others via the sender interface
- **Delivery resilience**: In-memory queue with exponential backoff retry
- **Configurable templates**: Customize message format per source via Go templates
- **Prometheus metrics**: Webhooks received, messages delivered/dropped, queue depth, delivery latency
- **Enterprise features**: Webhook authentication, rate limiting, request body size limits, structured JSON logging

## Quick Start

### Docker

```bash
docker run -d \
  -e STOAT_BOT_TOKEN=your-bot-token \
  -e STOAT_CHANNEL_ID=your-channel-id \
  -p 8080:8080 \
  ghcr.io/jadunawa/stoat-bridge:latest
```

### Helm

```bash
helm install stoat-bridge oci://ghcr.io/jadunawa/stoat-bridge-chart \
  --set existingSecret=stoat-bridge-secret
```

## API

### Webhook Endpoints

All webhook endpoints return `202 Accepted` on success. When `WEBHOOK_SECRET` is set, all webhook endpoints require `Authorization: Bearer <secret>`.

| Endpoint | Method | Description |
|---|---|---|
| `/webhook` | POST | Auto-detects source from payload shape |
| `/grafana` | POST | Grafana Alerting webhook |
| `/alertmanager` | POST | Prometheus Alertmanager webhook |
| `/gatus` | POST | Gatus webhook |

### Operational Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/healthz` | GET | Liveness probe (always 200) |
| `/readyz` | GET | Readiness probe (200 when ready, 503 during startup/shutdown) |
| `/metrics` | GET | Prometheus metrics |

## Configuration

All configuration via environment variables.

### Required

| Variable | Description |
|---|---|
| `STOAT_BOT_TOKEN` | Stoat bot authentication token |
| `STOAT_CHANNEL_ID` | Default/fallback channel ID |

### Optional

| Variable | Default | Description |
|---|---|---|
| `STOAT_API_URL` | `https://api.stoat.chat` | Stoat API base URL |
| `STOAT_CRITICAL_CHANNEL_ID` | `STOAT_CHANNEL_ID` | Channel for critical alerts |
| `STOAT_WARNING_CHANNEL_ID` | `STOAT_CHANNEL_ID` | Channel for warning alerts |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `QUEUE_SIZE` | `100` | Max buffered messages |
| `MAX_RETRIES` | `3` | Delivery attempts before dropping |
| `SHUTDOWN_TIMEOUT` | `10s` | Queue drain timeout on shutdown |
| `MAX_BODY_SIZE` | `1048576` | Max request body (bytes) |
| `MAX_MESSAGE_LENGTH` | `1900` | Max message length before truncation |
| `WEBHOOK_SECRET` | *(empty)* | Shared secret for webhook auth |
| `RATE_LIMIT` | `100` | Max requests per second |
| `GRAFANA_TEMPLATE` | *(built-in)* | Go template for Grafana messages |
| `ALERTMANAGER_TEMPLATE` | *(built-in)* | Go template for Alertmanager messages |
| `GATUS_TEMPLATE` | *(built-in)* | Go template for Gatus messages |

## Helm Values

See [charts/stoat-bridge/values.yaml](charts/stoat-bridge/values.yaml) for all configurable values.

Key settings:
- `existingSecret`: Name of a Kubernetes Secret containing `STOAT_BOT_TOKEN`, `STOAT_CHANNEL_ID`, and optionally `WEBHOOK_SECRET`
- `serviceMonitor.enabled`: `true` by default for Prometheus auto-discovery
- `autoscaling.enabled`: `false` by default
- `networkPolicy.enabled`: `false` by default

## Adding Webhook Sources

Implement the `Handler` interface in `internal/handler/`:

```go
type Handler interface {
    Name() string
    Parse(r *http.Request) ([]message.Message, error)
}
```

1. Create a new file (e.g., `internal/handler/discord.go`)
2. Implement `Name()` returning the handler's URL path name
3. Implement `Parse()` to convert the webhook payload into messages
4. Register it in `cmd/stoat-bridge/main.go`

The router automatically creates a `POST /<name>` endpoint.

## Adding Output Platforms

Implement the `Sender` interface in `internal/sender/`:

```go
type Sender interface {
    Send(ctx context.Context, msg message.Message) error
}
```

Return a `*PermanentError` for non-retryable failures (4xx). Return a regular error for transient failures (5xx, network) that should be retried.

## Architecture

```
[Grafana]      ─┐
[Alertmanager] ─┤─ POST ─▶ Ingestion ─▶ Translation ─▶ Queue ─▶ Sender ─▶ Stoat API
[Gatus]        ─┘
```

Four layers, each with a single responsibility:
1. **Ingestion**: HTTP server, auth, rate limits, body size enforcement
2. **Translation**: Source-specific handlers that parse payloads into normalized messages
3. **Queue**: Bounded in-memory buffer with exponential backoff retry
4. **Sender**: Delivers messages to chat platform APIs

## Metrics

| Metric | Type | Labels |
|---|---|---|
| `stoatbridge_webhooks_received_total` | Counter | `source`, `status_code` |
| `stoatbridge_messages_queued_total` | Counter | |
| `stoatbridge_messages_delivered_total` | Counter | `channel` |
| `stoatbridge_messages_dropped_total` | Counter | `reason` |
| `stoatbridge_delivery_duration_seconds` | Histogram | |
| `stoatbridge_queue_depth` | Gauge | |

## License

GPL v3 — see [LICENSE](LICENSE).
