> CoTriage-Nova: this top-level service now uses Java 25. Build from the repository root with `./mvnw -pl router-controller -am -DskipTests package`. See [migration notes](../migration/README.md) for compatibility boundaries.

# router-controller

`router-controller` consumes `cotriage.events.inferred`, applies the RAISE routing algorithm, writes lightweight display decisions to MySQL, and publishes routing decisions to downstream streams.

Current implementation status:

- independent Go module and process entrypoint;
- MySQL schema models for:
  - `router_decisions`
  - `router_state_snapshots`
  - `cloud_outbox`
  - `cloud_feedback`
- idempotent store methods for decisions, outbox rows, and feedback rows;
- schema initialization through GORM AutoMigrate or SQL script.

The first development round intentionally keeps MySQL lightweight: full RAISE decision details stay in `cotriage.events.routed` payloads or later offline replay/export files. MySQL stores only fields needed by the dashboard, event list, outbox, and feedback loop.

## Local run

Start datastore port-forwards first:

```bash
./script/forward-datastores.sh
```

Then run router-controller:

```bash
cd router-controller
REDIS_ADDR=127.0.0.1:6379 \
MYSQL_DSN='cotriage:cotriage-pass@tcp(127.0.0.1:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC' \
java -Xms64m -Xmx256m -jar target/router-controller-0.1.0-SNAPSHOT.jar
```

Inject a minimal inferred event:

```bash
redis-cli -h 127.0.0.1 -p 6379 XADD cotriage.events.inferred '*' payload '{"event_id":"router-smoke-1","source":"local","equipment":"host-1","template_id":"tpl-smoke","edge_pred":"DISK_ERROR","severity":90,"confidence":0.42,"uncertainty":0.58,"inference_path":"MODEL","latency_ms":12}'
```

Check downstream streams:

```bash
redis-cli -h 127.0.0.1 -p 6379 XREVRANGE cotriage.events.routed + - COUNT 5
redis-cli -h 127.0.0.1 -p 6379 XREVRANGE cotriage.events.cloud.req + - COUNT 5
```

Check MySQL display tables:

```bash
mysql -h 127.0.0.1 -P 3306 -ucotriage -pcotriage-pass cotriage
SELECT event_id, action, reason, score, threshold, routed_at FROM router_decisions ORDER BY id DESC LIMIT 5;
SELECT event_id, status, priority_score, attempt, last_error FROM cloud_outbox ORDER BY id DESC LIMIT 5;
SELECT snapshot_type, snapshot_key, version, created_at FROM router_state_snapshots ORDER BY id DESC LIMIT 5;
```

## Kubernetes

Build the image:

```bash
cd router-controller
make docker-build IMAGE=crpi-fq916lepz6l0u7gr.cn-guangzhou.personal.cr.aliyuncs.com/pangjian001/router-controller:v0.0.1
```

Apply the deployment:

```bash
kubectl --kubeconfig kind-host apply -f charts/router-controller/deployment.yaml
kubectl --kubeconfig kind-host -n cotriage rollout status deploy/router-controller
kubectl --kubeconfig kind-host -n cotriage logs deploy/router-controller -f
```

Useful environment variables:

| Name | Default |
| --- | --- |
| `REDIS_ADDR` | `redis:6379` |
| `INFERRED_STREAM` | `cotriage.events.inferred` |
| `ROUTED_STREAM` | `cotriage.events.routed` |
| `CLOUD_REQ_STREAM` | `cotriage.events.cloud.req` |
| `DLQ_STREAM` | `cotriage.events.dlq` |
| `CONSUMER_GROUP` | `router-g` |
| `CONSUMER_NAME` | `router-controller-1` |
| `MYSQL_DSN` | cluster-local cotriage DSN |
| `AUTO_MIGRATE` | `1` |
| `OUTBOX_REPLAYER_ENABLED` | `1` |
| `OUTBOX_REPLAYER_INTERVAL` | `2s` |
| `OUTBOX_REPLAYER_BATCH` | `20` |
| `OUTBOX_LOCK_TTL` | `30s` |
| `OUTBOX_RETRY_BASE_DELAY` | `5s` |
| `OUTBOX_MAX_ATTEMPTS` | `5` |
| `FEEDBACK_PROCESSOR_ENABLED` | `1` |
| `FEEDBACK_PROCESSOR_INTERVAL` | `5s` |
| `FEEDBACK_PROCESSOR_BATCH` | `20` |
| `STATE_WINDOW` | `10m` |
| `GAIN_UCB_ALPHA` | `0.1` |
| `WORKER_BLOCK_TIMEOUT` | `2s` |
| `WORKER_BATCH` | `20` |
| `ROUTER_STREAM_MAXLEN` | `100000` |

RAISE parameters can be adjusted with environment variables such as:

| Name | Default |
| --- | --- |
| `RAISE_THRESHOLD_BASE` | `0.55` |
| `RAISE_WEIGHT_RISK` | `0.30` |
| `RAISE_WEIGHT_UNCERTAINTY` | `0.25` |
| `RAISE_SCORE_VALUE` | `0.55` |
| `RAISE_SCORE_GAIN` | `0.35` |
| `RAISE_SCORE_COST` | `0.30` |
| `RAISE_FAIL_RATE_LIMIT` | `0.20` |
