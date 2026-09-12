# cloud-gateway

`cloud-gateway` consumes router cloud requests, builds nearby log context, invokes an
OpenAI-compatible diagnosis model, computes CLOUD-DIAG feedback, writes
`cloud_feedback`, and publishes `cotriage.events.cloud.resp`.

## Run

The server must have one active cloud provider. The database stores only the name
of the API-key environment variable.

```bash
export MYSQL_DSN='cotriage:cotriage-pass@tcp(localhost:3306)/cotriage?parseTime=true&loc=UTC'
export REDIS_ADDR=localhost:6379
export SERVER_BASE_URL=http://localhost:8080
export DEEPSEEK_API_KEY='...'
go run ./cmd/cloud-gateway
```

Create and activate a provider through:

```text
POST /api/v1/cloud-providers
POST /api/v1/cloud-providers/{id}/activate
GET  /api/v1/cloud-config
```

See `internal/config/config.go` and `design_doc/cloud-gateway.md` for all
environment variables and contracts.
