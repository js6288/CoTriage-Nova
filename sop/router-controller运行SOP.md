# router-controller 运行 SOP

## 1. 前置依赖

- Redis 服务：`redis:6379` 或本地 `127.0.0.1:6379`
- MySQL 服务：数据库 `cotriage`
- 已启动 `edge-infer`，并能写入 `cotriage.events.inferred`
- 可选：server 已启动，用于 Dashboard 查看 `/api/v1/router/*`

## 2. 本地端口转发

```bash
./script/forward-datastores.sh
```

默认端口：

- MySQL: `127.0.0.1:3306`
- Redis: `127.0.0.1:6379`
- onnx-classifier: `127.0.0.1:18080`
- server: `127.0.0.1:8080`

## 3. 本地启动 router-controller

```bash
cd router-controller
REDIS_ADDR=127.0.0.1:6379 \
MYSQL_DSN='cotriage:cotriage-pass@tcp(127.0.0.1:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC' \
INFERRED_STREAM=cotriage.events.inferred \
ROUTED_STREAM=cotriage.events.routed \
CLOUD_REQ_STREAM=cotriage.events.cloud.req \
DLQ_STREAM=cotriage.events.dlq \
CONSUMER_GROUP=router-g \
CONSUMER_NAME=router-controller-local \
AUTO_MIGRATE=1 \
java -Xms64m -Xmx256m -jar target/router-controller-0.1.0-SNAPSHOT.jar
```

也可以使用 VSCode `router-controller` debug 配置启动。

## 4. 注入测试事件

```bash
redis-cli -h 127.0.0.1 -p 6379 XADD cotriage.events.inferred '*' payload '{"event_id":"router-smoke-1","source":"local","equipment":"host-1","template_id":"tpl-smoke","edge_pred":"DISK_ERROR","severity":90,"confidence":0.42,"uncertainty":0.58,"inference_path":"MODEL","latency_ms":12}'
```

## 5. 查看 routed stream

```bash
redis-cli -h 127.0.0.1 -p 6379 XREVRANGE cotriage.events.routed + - COUNT 5
```

若动作是 `CLOUD`，继续查看：

```bash
redis-cli -h 127.0.0.1 -p 6379 XREVRANGE cotriage.events.cloud.req + - COUNT 5
```

若动作是 `DEFER`，查看 MySQL outbox：

```sql
SELECT event_id, status, priority_score, defer_reason, attempt, next_retry_at
FROM cloud_outbox
ORDER BY id DESC
LIMIT 10;
```

## 6. 查看 Dashboard

启动 server 和 web 后打开：

```text
http://127.0.0.1:5173/
```

Dashboard 中应能看到：

- RAISE 路由总量；
- LOCAL/CLOUD/DEFER 分布；
- Outbox 补偿队列；
- 实时事件表中的路由动作；
- 事件展开详情中的 score、threshold、reason、cloud health。

## 7. 集群部署

```bash
kubectl --kubeconfig kind-host apply -f charts/router-controller/deployment.yaml
kubectl --kubeconfig kind-host -n cotriage rollout status deploy/router-controller
kubectl --kubeconfig kind-host -n cotriage logs deploy/router-controller -f
```

## 8. 常见排查

1. 没有 routed event：
   - 检查 `cotriage.events.inferred` 是否有输入；
   - 检查 `router-controller` 日志；
   - 检查 Redis consumer group pending。
2. Dashboard 显示 degraded：
   - 检查 MySQL 中 `router_decisions`、`cloud_outbox`、`router_state_snapshots` 是否存在；
   - 检查 server 是否配置了正确的 `MYSQL_DSN`；
   - 检查 router-controller 是否启动且 `AUTO_MIGRATE=1`。
3. Outbox 长时间积压：
   - 检查 `CLOUD_HEALTH_URL`；
   - 检查 `CLOUD_BUDGET_*` 与 `CLOUD_RATE_LIMIT`；
   - 检查 `cotriage.events.cloud.req` 是否被 cloud-gateway 消费。
