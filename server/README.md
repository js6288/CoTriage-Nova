# CoTriage Server (MVP)

入口：`cmd/server`

## Device API

- `GET /api/v1/devices`

Query params (optional):
- `site`
- `equipment`
- `status`: `Ready|NotReady|Unknown`
- `limit` (default 200, max 1000)
- `offset`

### Env

- `LISTEN_ADDR` (default `:8080`)
- `KUBECONFIG` (local run)
- `REDIS_ADDR` (default `redis:6379`)
- `REDIS_STREAM` (default `cotriage.events.raw`)
- `INFERRED_STREAM` (default `cotriage.events.inferred`)
- `ROUTED_STREAM` (default `cotriage.events.routed`)
- `CLOUD_REQ_STREAM` (default `cotriage.events.cloud.req`)
- `CLOUD_RESP_STREAM` (default `cotriage.events.cloud.resp`)
- `DLQ_STREAM` (default `cotriage.events.dlq`)

## Router API

Base path: `/api/v1/router`

- `GET /summary`
- `GET /decisions` (query: `action`, `template_id`, `source`, `limit`, `offset`)
- `GET /decisions/:event_id`
- `GET /outbox` (query: `status`, `limit`, `offset`)
- `GET /state` (query: `type`, such as `router_state` or `system_state`)
- `GET /stream` (SSE over `ROUTED_STREAM`)

The router API is degraded-safe: if router-controller has not created its MySQL tables yet, list and summary endpoints return empty data instead of crashing the server.

## Edge Provider API (Edge Model Management)

Base path: `/api/v1`

- `POST /edge-providers`
- `PATCH /edge-providers/:id`
- `GET /edge-providers/:id`
- `GET /edge-providers` (query: `status`, `active`, `q`, `page`, `page_size`)
- `DELETE /edge-providers/:id` (soft delete; if active => 409)
- `POST /edge-providers/:id/activate` (force `status=published`, `is_active=true`, bump `config_version`, generate `ETag`)
- `GET /edge-config` (supports `If-None-Match` -> 304)

### Env

- `MYSQL_DSN`
	- default: `cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC`
	- note: will auto create `edge_providers` table if missing (MVP)

## Notes

- `last_event_time` / `ingest_rate_1m` are computed from Redis Stream payload. MVP implementation is O(N) over last 60s events.

## Layout

- `cmd/server`: main entry
- `internal/api`: Gin handlers + router
- `internal/config`: env/config
- `internal/kube`: Kubernetes client
- `internal/redisclient`: Redis client
- `internal/model`: response models
- `internal/util`: small helpers
