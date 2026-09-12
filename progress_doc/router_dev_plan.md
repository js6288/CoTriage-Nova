# router-controller 开发计划

本文档用于在正式编码前拆解 `router-controller` 的开发工作。目标不是实现一个临时阈值路由器，而是直接按照 `design_doc/router_design.md` 中的最终论文版 RAISE 算法落地：校准、conformal 风险集合、在线收益学习、动态阈值、策略 hard constraint、token bucket 和优先级 outbox 都进入主实现范围。MySQL 只保留必要展示字段，完整论文复现数据通过 routed stream、日志和离线 replay/export 工具获得。

## 1. 开发目标

`router-controller` 是 CoTriage 的云边协同决策组件，位于 `edge-infer` 与 `server/cloud-gateway` 之间。

输入：

- Redis Stream：`cotriage.events.inferred`
- 消费组：`router-g`
- payload：`edge-infer` 输出的结构化推理结果

输出：

- `cotriage.events.routed`：所有事件的路由决策结果；
- `cotriage.events.cloud.req`：需要立即上云的事件；
- `cloud_outbox`：值得上云但当前不可发送的事件；
- MySQL 展示表：保存事件列表、大盘统计和链路追踪所需的最小决策字段。

核心动作：

- `LOCAL`：边缘结果可直接进入后续展示、落库或告警；
- `CLOUD`：立即发送到云端处理链路；
- `DEFER`：进入 outbox，网络或预算恢复后按优先级补传。

## 2. 当前代码基线

现有仓库结构：

- `log-collector`：已向 `cotriage.events.raw` 写入原始日志；
- `edge-infer`：已消费 raw stream 并向 `cotriage.events.inferred` 写入推理结果；
- `server`：已提供大盘、事件列表、SSE、规则、模板、模型配置等 API；
- `web`：已展示 inferred event 和运行态大盘；
- 当前尚无独立 `router-controller` 服务。

可复用模式：

- `edge-infer/internal/worker/worker.go` 已实现 Redis Streams consumer group、pending claim、DLQ、幂等键和自动裁剪，可作为 router worker 的工程模板；
- `edge-infer/internal/event/event.go` 已定义 `InferredEvent` 字段，可作为 router 输入契约；
- `server/internal/api/handlers_dashboard.go` 已从 inferred stream 构造展示数据，后续需切换或补充 routed stream；
- `server/internal/config/config.go` 目前只有 raw/inferred/dlq stream 配置，需补充 routed/cloud.req 配置，并通过 MySQL `cloud_feedback` 表暴露云端反馈。

## 3. 总体实现策略

采用独立 Go module/service：

```text
router-controller/
  cmd/router-controller/main.go
  internal/config/
  internal/event/
  internal/worker/
  internal/raise/
  internal/state/
  internal/store/
  internal/cloud/
  internal/policy/
```

理由：

- router 是独立 Deployment，生命周期和扩缩容不应绑在 server；
- RAISE 需要维护在线状态、窗口统计、token bucket 和 outbox replayer，放在 server 会污染 API 层；
- 独立服务更容易做端到端实验和论文复现。

与 server 的边界：

- router-controller 负责路由决策、outbox 和 cloud.req 发布；
- server/cloud-gateway 负责云端 LLM 调用、云端结果落库和反馈写回；
- server 提供管理 API 和可视化 API；
- web 展示 routed 决策、上云比例、outbox 状态、RAISE 特征解释。

已确认的 review 决策：

- 新增独立 `router-controller` 服务，不合入 server；
- 第一轮开发先完成主链路，后续再接完整 web/API/baseline；
- `DEFER` 直接落入 MySQL `cloud_outbox`，不额外引入 Redis priority queue；
- cloud feedback 由 server 写 MySQL `cloud_feedback`，router-controller 从 DB 扫描并处理；
- 第一轮 RAISE 允许使用配置参数启动，但配置项必须完整映射最终论文版 RAISE 的公式参数，不能退化为简单阈值路由；
- 每轮开发进展必须持续追加到本文档的“开发进展记录”章节。

## 4. 数据契约

### 4.1 输入：inferred event

沿用 `edge-infer` 当前输出：

```json
{
  "event_id": "01J...",
  "source": "host-5-worker",
  "equipment": "LinuxServer",
  "event_ts": "2026-06-22T03:00:00Z",
  "inferred_at": "2026-06-22T03:00:00.073Z",
  "raw_log": "[error] connect to 10.0.0.2:3306 failed: connection refused",
  "template_id": "a81c70b9e4af0c21",
  "template_text": "connect to <IP>:<NUM> failed: connection refused",
  "edge_pred": "NET_ERROR",
  "severity": 78,
  "confidence": 0.93,
  "uncertainty": 0.07,
  "evidence": ["connection refused", "3306"],
  "advice_code": "CHECK_NETWORK_AND_SERVICE",
  "advice": "检查目标服务状态、端口监听和网络策略",
  "inference_path": "MODEL",
  "model_version": "edge-log-classifier-v1",
  "rule_version": "edge-rules-v1",
  "latency_ms": 73,
  "stage_latency_ms": {
    "normalize": 2,
    "cache": 1,
    "model": 68,
    "publish": 2
  }
}
```

### 4.2 输出：routed event

`cotriage.events.routed` payload：

```json
{
  "event_id": "01J...",
  "source": "host-5-worker",
  "equipment": "LinuxServer",
  "template_id": "a81c70b9e4af0c21",
  "edge_pred": "NET_ERROR",
  "severity": 78,
  "action": "CLOUD",
  "policy_ref": "raise-v1",
  "reason": "HIGH_SCORE_SENDABLE",
  "score": 0.83,
  "threshold": 0.61,
  "features": {
    "risk": 0.78,
    "calibrated_uncertainty": 0.42,
    "conformal_set_size": 3,
    "novelty": 0.90,
    "historical_disagreement": 0.31,
    "fleet_impact": 0.20,
    "path_risk": 0.45,
    "expected_gain": 0.48,
    "ucb_bonus": 0.08
  },
  "costs": {
    "cloud_cost": 0.12,
    "latency_cost": 0.04,
    "network_cost": 0.02,
    "privacy_cost": 0.0
  },
  "system_snapshot": {
    "queue_pressure": 0.18,
    "budget_pressure": 0.21,
    "network_pressure": 0.03,
    "audit_pressure": 0.10,
    "cloud_health": "UP",
    "rtt_ms": 86,
    "fail_rate": 0.02
  },
  "budget_snapshot": {
    "minute_remaining": 17,
    "day_remaining": 4312
  },
  "latency_ms": 4,
  "routed_at": "2026-06-30T12:00:30Z"
}
```

### 4.3 输出：cloud request

`cotriage.events.cloud.req` payload 应包含原 inferred event、router 决策解释和幂等键：

```json
{
  "event_id": "01J...",
  "request_id": "01J...:cloud:1",
  "idempotency_key": "cloud:01J...",
  "routed_at": "2026-06-30T12:00:30Z",
  "route_decision": {
    "action": "CLOUD",
    "score": 0.83,
    "threshold": 0.61,
    "reason": "HIGH_SCORE_SENDABLE"
  },
  "inferred_event": {}
}
```

### 4.4 Outbox 数据

`cloud_outbox` 用于 `DEFER` 事件：

```text
id
event_id
payload_json
route_decision_json
priority_score
defer_reason
status              PENDING | SENDING | SENT | DONE | RETRY | DLQ
attempt
max_attempts
next_retry_at
deadline_at
created_at
updated_at
sent_at
done_at
last_error
```

## 5. MySQL 表设计

### 5.1 MySQL 字段分层原则

MySQL 表不应把所有论文复现字段都展开到热表列中。router 是流式组件，`router_decisions` 会随事件量线性增长，如果每条事件都写入几十个独立列并建立查询条件，容易带来以下问题：

- 写入放大：每条事件都要写大量列，影响 router ACK 速度；
- 索引膨胀：如果为过多字段建索引，会显著增加磁盘和写入成本；
- schema 僵硬：算法字段迭代时频繁改表；
- 查询混乱：业务查询、调试查询和论文复现查询混在同一张热表中。

因此采用展示优先的轻量存储：

1. `router_decisions`：只保存页面展示、列表筛选和链路追踪所需字段；
2. `cloud_outbox`：只保存 `DEFER` 补传所需 payload 和状态；
3. `cloud_feedback`：只保存 server 云端反馈闭环所需字段；
4. 论文复现数据不进入 MySQL，由后续离线 replay/export 工具从 Redis Stream、日志或专用实验文件导出。

第一轮开发不创建决策审计表。RAISE 的完整决策解释只写入 `cotriage.events.routed` payload，MySQL 只保留必要展示字段。

### 5.2 router_decisions

展示决策表。记录大盘查询、事件列表、动作统计、排障定位所需的最小字段。

```text
id
event_id unique
trace_id
schema_version
source
equipment
event_ts
inferred_at
template_id
raw_log_hash
edge_pred
severity
confidence
edge_uncertainty
inference_path
model_version
rule_version
action
reason
policy_ref
score
threshold
cloud_health
raise_version
latency_ms
routed_at
created_at
updated_at
```

说明：

- `raw_log_hash` 用于论文复现和隐私保护，默认不在该表长期保存完整原始日志；
- 独立列只保留高频查询字段和页面展示字段；
- 第一轮只为 `event_id`、`routed_at`、`action`、`template_id`、`source` 建索引；
- `score`、`threshold`、`severity` 可以保留普通列，但第一轮不建复杂组合索引；
- RAISE 完整特征、成本、系统快照不落 MySQL，只保留在 routed stream payload 或运行日志中。

建议索引：

```text
unique(event_id)
index(routed_at)
index(action, routed_at)
index(template_id, routed_at)
index(source, routed_at)
```

### 5.3 router_state_snapshots

周期性保存在线状态，支持重启恢复和论文实验复现。

```text
id
snapshot_type       calibrator | conformal | template_stats | gain_model | token_bucket | policy
snapshot_key
payload_json
version
created_at
```

补充字段：

```text
checksum
is_active
restored_at
metadata_json
```

### 5.4 cloud_outbox

见 4.4。

完整表字段：

```text
id
event_id unique
request_id unique
idempotency_key unique
payload_json
route_decision_json
priority_score
defer_reason
status              PENDING | SENDING | SENT | DONE | RETRY | DLQ
attempt
max_attempts
next_retry_at
deadline_at
created_at
updated_at
locked_by
locked_until
sent_at
done_at
last_error
last_error_at
dlq_reason
```

### 5.5 cloud_feedback

云端处理完成后由 server 写入，用于 router-controller 更新在线收益模型。第一轮只建表和 schema，不要求 router 完成在线学习闭环。

```text
id
event_id unique
request_id
idempotency_key
source
equipment
template_id
edge_pred
edge_confidence
route_action
route_score
route_threshold
cloud_model
cloud_pred
root_cause
recommendation
explanation
cloud_confidence
cloud_latency_ms
token_usage
cost
reward
reward_reason
corrected_edge
added_actionable_info
confirmed_high_risk
feedback_json
processed_by_router
processed_at
created_at
```

### 5.6 router_development_progress

开发进展以 Markdown 追加到本文档第 11 节，不单独建表。每轮编码必须记录：

- 日期；
- 完成内容；
- 修改文件；
- 验证命令；
- 未完成事项；
- 下一步计划。

## 6. 开发阶段

### 阶段 0：契约冻结和脚手架

目标：建立 router-controller 独立服务骨架，不实现完整 RAISE。

任务：

1. 新增 `router-controller` Go module；
2. 新增 `cmd/router-controller/main.go`；
3. 新增配置读取：
   - `REDIS_ADDR`
   - `INFERRED_STREAM`
   - `ROUTED_STREAM`
   - `CLOUD_REQ_STREAM`
   - `DLQ_STREAM`
   - `CONSUMER_GROUP`
   - `CONSUMER_NAME`
   - `MYSQL_DSN`
   - `POLICY_FILE`
   - `STATE_SNAPSHOT_INTERVAL`
4. 复制并改造 edge-infer worker 模式：
   - consumer group；
   - pending claim；
   - attempts；
   - idempotency key；
   - DLQ；
   - graceful shutdown。
5. 新增 `router-controller/README.md`；
6. 新增 Dockerfile 和 Makefile。

验收：

- `go test ./...` 通过；
- 可启动服务并创建 `router-g`；
- 读取一条 inferred event 后能写出最小 `routed` event；
- 重复 event 不产生重复决策。

### 阶段 1：数据库迁移和持久化

目标：把 router 决策、outbox 和反馈数据落到 MySQL。

任务：

1. 确定当前项目迁移方式。如果没有 migration 框架，先在 `server/internal/db` 或新目录中提供 SQL 初始化脚本；
2. 新增表：
   - `router_decisions`
   - `router_state_snapshots`
   - `cloud_outbox`
   - `cloud_feedback`
3. router-controller 新增 store 层；
4. 决策写入 `router_decisions`；
5. `DEFER` 写入 `cloud_outbox`；
6. 提供幂等写入逻辑，`event_id` 唯一冲突时返回已有结果。

验收：

- 同一 event 重放不会写出多条 decision；
- `DEFER` event 能进入 outbox；
- MySQL 不可用时 router 不 ACK 消息，等待重试或 DLQ；
- 单元测试覆盖 store 的 create/get/update。

### 阶段 2：RAISE 核心特征与打分

目标：实现最终算法主路径，不接云端反馈也能做完整决策。

任务：

1. 新增 `internal/raise`：
   - feature extractor；
   - calibrator；
   - conformal risk set；
   - cloud value；
   - expected gain；
   - expected cost；
   - dynamic threshold；
   - action selector。
2. 实现特征：
   - severity risk；
   - calibrated uncertainty；
   - conformal set size；
   - template novelty；
   - historical disagreement；
   - fleet impact；
   - path risk；
   - expected gain；
   - UCB bonus。
3. 实现成本：
   - cloud cost；
   - latency cost；
   - network cost；
   - privacy cost。
4. 实现动态阈值：
   - queue pressure；
   - budget pressure；
   - network pressure；
   - audit pressure。
5. 实现 policy hard constraint：
   - deny cloud；
   - force local；
   - force cloud。

验收：

- 对固定输入，分数和动作稳定可复现；
- 高严重度/高不确定事件在健康网络与预算充足时走 `CLOUD`；
- 低风险高置信事件走 `LOCAL`；
- cloud down 时高价值事件走 `DEFER`；
- deny cloud 永远不会输出 `CLOUD`；
- 单元测试覆盖每个公式模块。

### 阶段 3：在线状态管理

目标：实现 RAISE 所需的在线状态和重启恢复。

任务：

1. 新增 `internal/state`；
2. 实现滑动窗口：
   - template frequency；
   - affected devices；
   - abnormal growth；
   - recent cloud ratio；
3. 实现 template stats：
   - historical cloud count；
   - cloud correction count；
   - disagreement rate with Beta prior；
4. 实现 gain model：
   - `theta`；
   - `A`；
   - `b`；
   - UCB bonus；
5. 实现 calibrator/conformal 参数加载；
6. 实现状态周期性 snapshot 到 MySQL；
7. 实现启动时从最新 snapshot 恢复。

验收：

- 重启后 template stats 和 token bucket 不完全丢失；
- 在线反馈能改变 expected gain；
- 滑动窗口随时间淘汰过期样本；
- snapshot 失败不影响主流程，但必须记录错误指标。

### 阶段 4：预算、网络和队列监测

目标：让动态阈值依赖真实系统态。

任务：

1. 实现 token bucket：
   - per-minute；
   - per-hour；
   - per-day；
2. 实现 Redis queue lag watcher：
   - inferred group pending；
   - cloud.req length；
   - outbox pending count；
3. 实现 cloud health probe：
   - server/cloud endpoint health；
   - RTT；
   - fail rate；
4. 实现 sendable 判断：
   - cloud health；
   - budget remaining；
   - rate limit；
   - fail rate threshold。
5. 将系统态写入 `system_snapshot`。

验收：

- 预算耗尽后不再输出 `CLOUD`；
- cloud health down 后高价值事件进入 `DEFER`；
- 队列压力升高会提升阈值；
- health probe 抖动不会导致动作频繁震荡，需要平滑窗口。

### 阶段 5：Outbox replayer

目标：实现弱网自治与最终一致性补偿。

任务：

1. 在 router-controller 内启动 replayer goroutine；
2. 周期性扫描 `cloud_outbox` 中 `PENDING/RETRY` 且到期事件；
3. 按 `priority_score`、`deadline_at`、`created_at` 排序；
4. 使用状态机：
   - `PENDING -> SENDING -> SENT`
   - `SENDING -> RETRY`
   - `RETRY -> SENDING`
   - `RETRY -> DLQ`
5. 发布到 `cotriage.events.cloud.req`；
6. 避免重复补传：
   - DB 行锁或 compare-and-swap 状态更新；
   - cloud request idempotency key；
7. 补传成功后更新 `sent_at` 和状态。

验收：

- 网络恢复后 outbox 按优先级补传；
- 多 router 实例不会重复发送同一 outbox event；
- 超过最大重试进入 DLQ；
- replayer 不阻塞主消费流程。

### 阶段 6：Cloud feedback 闭环

目标：让云端结果反向更新 RAISE 在线收益模型。

任务：

1. server/cloud-gateway 在云端处理完成后写入 MySQL `cloud_feedback`；
2. router-controller 定期扫描 `cloud_feedback` 中 `processed_by_router=false` 的记录；
3. 实现 reward 计算：
   - 云端纠错：1.0；
   - 云端增加可执行根因/建议：0.7；
   - 云端确认高风险边缘决策：0.3；
   - 无新增信息：0.0；
4. 更新 gain model、template disagreement、audit stats；
5. 将 feedback update 写入状态快照。

验收：

- 同一 feedback 只处理一次；
- reward 能改变后续同类模板的 expected gain；
- cloud feedback 缺失时算法仍可运行；
- 单元测试覆盖 reward 分类。

### 阶段 7：server API 配套

目标：让前端和实验可以观察 RAISE。

任务：

1. server 配置补充：
   - `ROUTED_STREAM`
   - `CLOUD_REQ_STREAM`
   - `CLOUD_RESP_STREAM`
2. 新增 API：
   - `GET /api/v1/router/summary`
   - `GET /api/v1/router/decisions`
   - `GET /api/v1/router/decisions/:event_id`
   - `GET /api/v1/router/outbox`
   - `GET /api/v1/router/state`
   - `GET /api/v1/router/stream`
3. dashboard summary 增加：
   - routed total；
   - local/cloud/defer ratio；
   - outbox pending；
   - cloud budget remaining；
   - P95 route latency；
   - average score/threshold；
4. events 列表支持展示 routed action。

验收：

- API 能从 MySQL 和 Redis 返回一致数据；
- SSE 能推送 routed event；
- server 在 router 未启动时不崩溃，返回空数据或 degraded 状态。

### 阶段 8：web 可视化配套

目标：让 RAISE 决策可解释。

任务：

1. Dashboard 事件流增加：
   - route action；
   - score；
   - threshold；
   - reason；
2. 事件详情增加 RAISE 解释：
   - risk；
   - calibrated uncertainty；
   - novelty；
   - disagreement；
   - expected gain；
   - costs；
   - system snapshot；
3. 新增 router 状态面板：
   - LOCAL/CLOUD/DEFER 分布；
   - cloud budget；
   - outbox pending；
   - network health；
   - queue pressure；
4. 管理端预留路由策略入口：
   - force cloud/local/deny cloud；
   - SLA；
   - cloud budget；
   - audit ratio。

验收：

- 前端可看到每条事件为什么被路由；
- outbox 积压、网络 down、预算耗尽都能在页面上被解释；
- 不要求第一轮做复杂策略编辑器，但必须能展示当前策略。

### 阶段 9：Kubernetes 与本地运行

目标：可以本地和集群运行 router-controller。

任务：

1. 新增 `charts/router-controller`；
2. 配置环境变量：
   - Redis；
   - MySQL；
   - streams；
   - budget；
   - policy；
   - cloud health endpoint；
3. 更新 README/SOP：
   - 本地启动；
   - Redis 注入测试；
   - 查看 routed stream；
   - 查看 outbox；
4. 更新 `script/forward-datastores.sh` 如有必要；
5. 增加 `.vscode/launch.json` 调试配置。

验收：

- 本地可用 `go run ./cmd/router-controller` 启动；
- Kubernetes 可部署；
- Redis 中 raw -> inferred -> routed -> cloud.req 链路可观察。

### 阶段 10：实验与回归测试

目标：支撑论文实验和后续代码稳定性。

任务：

1. 单元测试：
   - feature extractor；
   - calibrator；
   - conformal；
   - gain model；
   - threshold；
   - policy；
   - token bucket；
   - outbox priority；
2. 集成测试：
   - Redis Streams 消费；
   - MySQL 决策落库；
   - cloud.req 发布；
   - outbox replay；
3. 离线 replay 工具：
   - 从 JSONL/CSV 读取 inferred event；
   - 执行 RAISE；
   - 输出 metrics；
4. baseline runner：
   - All-Local；
   - All-Cloud；
   - Confidence-Threshold；
   - Early-Exit-style；
   - Cascade-style；
   - Bandit-Routing；
   - Budget-Greedy；
   - RAISE ablations。

验收：

- `go test ./...` 通过；
- replay 工具能输出上云比例、Recall、成本、时延、DEFER 完成时间；
- RAISE 与 baseline 的实验结果可复现。

## 7. 开发顺序建议

建议按以下顺序开发：

1. 阶段 0：router-controller 脚手架；
2. 阶段 1：MySQL 表与持久化；
3. 阶段 2：RAISE 核心打分；
4. 阶段 4：预算、网络、队列状态；
5. 阶段 5：outbox replayer；
6. 阶段 3：在线状态 snapshot 和恢复；
7. 阶段 6：cloud feedback 闭环；
8. 阶段 7：server API；
9. 阶段 8：web 可视化；
10. 阶段 9：部署；
11. 阶段 10：实验工具和 baseline。

阶段 3 可在阶段 2 后部分并行，但不建议先做复杂 snapshot，再做主决策路径。先跑通 event -> decision -> routed/cloud.req/outbox 的骨架，后面再把在线状态做完整。

## 8. 第一轮开发边界

第一轮编码已确认只完成 router 主链路，并把进展持续追加到本文档第 11 节。

1. 新增 `router-controller` 独立服务；
2. 消费 `cotriage.events.inferred`；
3. 实现 RAISE 核心打分的配置参数化版本；
4. 输出 `cotriage.events.routed`；
5. `CLOUD` 输出 `cotriage.events.cloud.req`；
6. `DEFER` 写入 `cloud_outbox`；
7. 决策写入 `router_decisions`；
8. 提供单元测试和本地 Redis 注入测试文档。

第一轮配置参数化说明：

- 配置参数必须覆盖最终 RAISE 主公式中的权重和阈值，包括 value weights、cost weights、dynamic threshold alpha、path risk、budget、network threshold、policy version；
- 第一轮可使用固定配置初始化 `calibrator`、`conformal_set_size`、`expected_gain` 和 `ucb_bonus`，但输出字段必须完整；
- 第一轮不得退化为 `confidence < threshold` 的单阈值规则；
- 所有配置参数必须写入 routed event 和 `router_decisions` 的版本字段，保证后续实验可复现。

第一轮不做：

- web 页面改造；
- 完整 baseline runner；
- Kubernetes chart；
- cloud feedback 自动更新，但需要建表并固定 server 写 DB 的 schema；
- 多副本 leader election。

这些不进入第一轮，是为了尽快形成可运行的 router 主链路，便于后续逐层增强。

## 9. 主要风险与处理

1. 边缘输出字段不稳定。
   - 处理：router 输入解析必须兼容缺失字段，并在 routed event 中记录 fallback reason。

2. MySQL migration 机制不明确。
   - 处理：先提供 SQL 脚本和启动时可选 auto-migrate，后续再统一迁移框架。

3. 云端反馈标签不标准。
   - 处理：先定义 reward 规则和 cloud feedback schema，server 按 schema 写入。

4. 多副本并发更新在线状态。
   - 处理：第一轮单副本；后续按 template 分片或 leader election。

5. 算法参数过多导致难以调试。
   - 处理：所有中间特征、成本、score、threshold 必须落库并进入 routed event。

6. Redis Stream 堆积。
   - 处理：沿用 edge-infer 的 pending claim、DLQ 和 maxlen 裁剪策略。

7. MySQL 写入压力过大。
   - 处理：`router_decisions` 只保存展示字段，不保存完整 RAISE 决策字段；第一轮只建少量必要索引；论文实验数据通过 Redis routed stream、日志或离线 replay/export 文件获取。

## 10. Review 重点

以下 review 决策已经确认：

1. 新增独立 `router-controller` 服务，而不是合入 server；
2. 第一轮先完成主链路，web/API/baseline 在后续阶段；
3. MySQL 表不持久化完整决策字段，只保留必要展示字段；
4. `DEFER` 直接落 MySQL `cloud_outbox`；
5. cloud feedback 由 server 写 MySQL `cloud_feedback`；
6. 第一轮 RAISE 使用配置参数启动，但 routed stream payload 必须保留最终论文版字段和版本记录。

后续编码前仍需确认：

1. MySQL 初始化采用独立 SQL 脚本，还是做启动时 auto-migrate；
2. 第一轮 cloud.req 是否先由 Redis Stream 承载，不直接调用云端；
3. `router-controller` Go module 路径是否使用 `github.com/pangjian-pj/cotriage-dev/router-controller`；
4. 第一轮是否新增 `server` API 查询 `router_decisions`，还是留到阶段 7。

## 11. 开发进展记录

### 日期：2026年06月30日（开发计划 review 决策固化）

已确认：

1. `router-controller` 作为独立 Go service 开发；
2. 第一轮只做主链路：inferred -> RAISE decision -> routed/cloud.req/outbox/MySQL decision；
3. `DEFER` 直接落 MySQL `cloud_outbox`；
4. cloud feedback 由 server 写 MySQL `cloud_feedback`；
5. 第一轮 RAISE 使用配置参数启动，但保留最终论文版字段、版本号和完整决策解释；
6. MySQL 不持久化完整决策字段，只保留必要展示字段；论文复现数据通过 routed stream、日志和离线 replay/export 获取；
7. 后续每轮开发进展必须追加到本节。

下一步：

1. 创建 `router-controller` 目录和 Go module；
2. 实现配置、事件结构、Redis Streams worker 和最小 routed 输出；
3. 添加 MySQL 初始化 SQL；
4. 写入第一轮开发进展和验证命令。

### 日期：2026年06月30日（阶段 1：MySQL 表与持久化）

已完成：

1. 新增独立 `router-controller` Go module：
   - `router-controller/go.mod`
   - `router-controller/go.sum`
   - `router-controller/cmd/router-controller/main.go`
   - `router-controller/Makefile`
   - `router-controller/README.md`
2. 新增配置模块：
   - `router-controller/internal/config/config.go`
   - `router-controller/internal/config/env.go`
3. 新增 MySQL 初始化与 schema 入口：
   - `router-controller/internal/db/db.go`
   - `router-controller/migrations/001_router_tables.sql`
4. 新增轻量 MySQL 表模型：
   - `router_decisions`：只保留展示字段；
   - `router_state_snapshots`：保留在线状态快照；
   - `cloud_outbox`：保存 `DEFER` 补传状态；
   - `cloud_feedback`：由 server 写入云端反馈。
5. 新增 store 层：
   - `CreateDecisionIfAbsent`：按 `event_id` 幂等写入决策；
   - `CreateOutboxIfAbsent`：按 `event_id/request_id/idempotency_key` 幂等写入 outbox；
   - `CreateFeedbackIfAbsent`：按 `event_id` 幂等写入 feedback；
   - `SaveStateSnapshot`：保存 router 在线状态快照。
6. 新增基础测试：
   - `router-controller/internal/store/models_test.go`

验证命令：

```bash
cd router-controller
gofmt -w cmd internal
go mod tidy
GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/router-go-build-cache go test ./...
```

验证结果：

- `gofmt` 通过；
- `go mod tidy` 通过；
- `go test ./...` 当前被本机 Go 工具链版本不一致阻塞：

```text
go version go1.26.1 darwin/arm64
go tool compile -V -> compile version go1.25.3
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

该错误发生在标准库编译阶段，不是 `router-controller` 业务代码错误。需要修复本机 Go 安装，使 `go` 和 `go tool compile` 版本一致后重新运行测试。

阶段 1 当前边界：

- 已完成数据库模型、AutoMigrate、SQL 脚本和 store 幂等写入方法；
- 尚未实现 Redis Streams worker、RAISE 决策和 routed/cloud.req 发布，这些进入下一阶段主链路开发。

### 日期：2026年06月30日（阶段 2：RAISE 核心特征与打分）

已完成：

1. 新增 router 输入事件结构：
   - `router-controller/internal/event/event.go`
2. 新增 RAISE 核心算法包：
   - `router-controller/internal/raise/types.go`
   - `router-controller/internal/raise/config.go`
   - `router-controller/internal/raise/env_config.go`
   - `router-controller/internal/raise/router.go`
   - `router-controller/internal/raise/math.go`
3. 实现配置参数化最终公式：
   - value weights；
   - cost weights；
   - score weights；
   - dynamic threshold alpha；
   - path risk；
   - temperature calibration；
   - conformal set size；
   - Beta prior disagreement；
   - UCB bonus；
   - budget/network/sendable 判断。
4. 实现 policy hard constraint：
   - deny cloud；
   - force local；
   - force cloud；
   - force cloud template；
   - deny source。
5. 实现动作选择：
   - `LOCAL`；
   - `CLOUD`；
   - `DEFER`。
6. 新增 RAISE 单元测试场景：
   - 高风险高价值且 sendable -> `CLOUD`；
   - 低风险高置信 -> `LOCAL`；
   - 高价值但 cloud down -> `DEFER`；
   - deny cloud policy -> `LOCAL`；
   - force cloud 但不可发送 -> `DEFER`。
7. README 增加 RAISE 参数环境变量示例。

验证命令：

```bash
cd router-controller
gofmt -w internal/config internal/event internal/raise
GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/router-go-build-cache-fresh-2 go test ./internal/raise
```

验证结果：

- `gofmt` 通过；
- `go test ./internal/raise` 仍被本机 Go 工具链版本不一致阻塞：

```text
go version go1.26.1 darwin/arm64
go tool compile -V -> compile version go1.25.3
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 2 当前边界：

- 已完成 RAISE 公式级核心决策，不依赖 Redis/MySQL；
- 在线状态目前由调用方传入 `Stats/SystemState/Policy`；
- 尚未接入 Redis Streams worker、状态窗口、MySQL decision 写入和 routed/cloud.req 发布。

### 日期：2026年06月30日（阶段 3：在线状态管理）

已完成：

1. 新增在线状态包：
   - `router-controller/internal/state/window.go`
   - `router-controller/internal/state/template_stats.go`
   - `router-controller/internal/state/gain.go`
   - `router-controller/internal/state/manager.go`
2. 实现滑动窗口：
   - template frequency；
   - affected devices；
   - recent cloud ratio；
   - 过期 observation 淘汰。
3. 实现模板统计：
   - historical cloud count；
   - historical correction count；
   - 后续由 RAISE 使用 Beta prior 计算 historical disagreement。
4. 实现在线收益模型：
   - `theta`；
   - `A`；
   - `b`；
   - UCB bonus；
   - reward 更新。
   当前实现采用对角 LinUCB 近似，避免在主链路里引入重型矩阵依赖。
5. 实现状态管理器：
   - `Observe`：记录事件和路由动作；
   - `BuildStats`：生成 `raise.Stats`；
   - `ApplyFeedback`：更新模板分歧和收益模型；
   - `SnapshotJSON`：序列化状态；
   - `RestoreManager`：从 JSON 恢复状态。
6. store 层新增：
   - `LatestStateSnapshot`：按 `snapshot_type/snapshot_key` 获取最新状态快照。
7. 新增在线状态单元测试：
   - 窗口过期淘汰；
   - feedback 提升 expected gain；
   - snapshot round trip。

验证命令：

```bash
cd router-controller
gofmt -w internal/state internal/store
GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/router-go-build-cache-fresh-3 go test ./internal/state
```

验证结果：

- `gofmt` 通过；
- `go test ./internal/state` 仍被本机 Go 工具链版本不一致阻塞：

```text
go version go1.26.1 darwin/arm64
go tool compile -V -> compile version go1.25.3
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 3 当前边界：

- 已完成内存态在线状态和 JSON snapshot/restore；
- 尚未启动周期性 snapshot goroutine；
- 尚未接入真实 Redis worker；
- token bucket、queue lag、cloud health probe 属于阶段 4。

### 日期：2026年06月30日（阶段 4：预算、网络和队列监测）

已完成：

1. 新增运行态系统状态包：
   - `router-controller/internal/runtime/token_bucket.go`
   - `router-controller/internal/runtime/health.go`
   - `router-controller/internal/runtime/queue.go`
   - `router-controller/internal/runtime/builder.go`
   - `router-controller/internal/runtime/runtime_test.go`
2. 实现 token bucket：
   - per-minute budget；
   - per-hour budget；
   - per-day budget；
   - rate-limit bucket；
   - refill 与 allow 判断。
3. 实现 cloud health probe：
   - 支持可配置 health URL；
   - 支持 RTT 采样；
   - 支持 fail-rate 平滑窗口；
   - 当 health URL 未配置时默认视为 `UP`，便于第一轮本地联调。
4. 实现 Redis/MySQL queue lag watcher：
   - 读取 `cotriage.events.inferred` consumer group pending；
   - 读取 `cotriage.events.cloud.req` stream length；
   - 读取 MySQL `cloud_outbox` pending/retry/sending 数量。
5. store 层新增：
   - `CountPendingOutbox`，用于系统态中的 outbox backlog。
6. config 层新增运行态参数：
   - `CLOUD_HEALTH_URL`
   - `CLOUD_HEALTH_TIMEOUT`
   - `CLOUD_HEALTH_WINDOW`
   - `QUEUE_PRESSURE_MAX`
   - `CLOUD_BUDGET_MINUTE`
   - `CLOUD_BUDGET_HOUR`
   - `CLOUD_BUDGET_DAY`
   - `CLOUD_RATE_LIMIT`
   - `BUDGET_REFILL_INTERVAL`
   - `MIN_CLOUD_RATIO_FOR_AUDIT`
   - `ESTIMATED_CLOUD_LATENCY`
7. 实现 `runtime.Builder`：
   - 聚合 token bucket、cloud health、queue lag；
   - 生成 RAISE 所需 `raise.SystemState`；
   - 为后续 worker 主链路提供统一系统态入口；
   - 提供 `ConsumeCloudBudget`，后续用于 cloud/defer 动作落地前扣减预算。
8. 实现系统态 snapshot 保存入口：
   - snapshot type 为 `system_state`；
   - snapshot key 为 `default`；
   - payload 包含 `raise.SystemState` 与 token bucket 当前状态；
   - 复用阶段 1 的 `router_state_snapshots` 表，不新增额外表。
9. 新增运行态单元测试：
   - token bucket 消耗与 refill；
   - health probe 平滑失败率；
   - builder 生成系统态。

验证命令：

```bash
cd router-controller
gofmt -w internal/config internal/store internal/runtime
go version
go tool compile -V
GOCACHE=/private/tmp/cotriage-go-cache go test ./internal/runtime
```

验证结果：

- `gofmt` 通过；
- `go version` 输出 `go version go1.26.1 darwin/arm64`；
- `go tool compile -V` 输出 `compile version go1.25.3`；
- `go test ./internal/runtime` 被本机 Go 工具链版本不一致阻塞，失败发生在标准库和依赖包编译阶段：

```text
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 4 当前边界：

- 已完成 RAISE 所需动态系统态构建能力；
- sendable 约束已可通过 `raise.SystemState` 影响阶段 2 的 RAISE 决策；
- 尚未将 `runtime.Builder` 接入 Redis Streams worker 主循环；
- 已提供 `system_state` snapshot 保存入口，但尚未启动周期性 snapshot goroutine；
- 尚未启动后台 health probe goroutine，目前为决策前同步采样；
- outbox replayer 属于阶段 5。

### 日期：2026年06月30日（阶段 5：Outbox replayer）

已完成：

1. 新增 outbox replayer 包：
   - `router-controller/internal/replayer/replayer.go`
   - `router-controller/internal/replayer/replayer_test.go`
2. 实现 Redis cloud request publisher：
   - 发布目标为 `cotriage.events.cloud.req`；
   - payload 包含 `event_id`、`request_id`、`idempotency_key`、`payload_json`、`route_decision_json`、`priority_score`；
   - 下游可基于 `idempotency_key` 做云端幂等。
3. store 层新增 outbox 状态机方法：
   - `ListDueOutbox`：扫描 `PENDING/RETRY` 且到期的 outbox；
   - `TryLockOutbox`：compare-and-swap 抢占任务，状态转为 `SENDING`；
   - `RequeueExpiredSending`：回收锁超时的 `SENDING`，避免进程崩溃导致永久卡死；
   - `MarkOutboxSent`：发布成功后转为 `SENT`；
   - `MarkOutboxRetry`：发布失败后转为 `RETRY`；
   - `MarkOutboxDLQ`：超过最大重试后转为 `DLQ`。
4. 实现 replayer 主循环：
   - 每轮先回收过期 `SENDING`；
   - 按 `priority_score DESC, deadline_at ASC, created_at ASC` 取任务；
   - 抢占成功后发布 Redis；
   - 发布成功标记 `SENT`；
   - 发布失败按指数退避写回 `RETRY`；
   - 达到最大尝试次数写入 `DLQ`。
5. 接入 router-controller 进程入口：
   - `OUTBOX_REPLAYER_ENABLED=1` 默认启动；
   - replayer 以独立 goroutine 运行，不阻塞主进程；
   - 使用 `CONSUMER_NAME` 作为 `locked_by` worker 标识。
6. config/README 新增 replayer 参数：
   - `OUTBOX_REPLAYER_ENABLED`
   - `OUTBOX_REPLAYER_INTERVAL`
   - `OUTBOX_REPLAYER_BATCH`
   - `OUTBOX_LOCK_TTL`
   - `OUTBOX_RETRY_BASE_DELAY`
   - `OUTBOX_MAX_ATTEMPTS`
7. 新增轻量单元测试：
   - retry backoff；
   - worker fallback。

验证命令：

```bash
cd router-controller
gofmt -w cmd/router-controller internal/config internal/store internal/replayer
GOCACHE=/private/tmp/cotriage-go-cache go test ./internal/replayer
```

验证结果：

- `gofmt` 通过；
- `go test ./internal/replayer` 仍被本机 Go 工具链版本不一致阻塞，失败发生在标准库和依赖包编译阶段：

```text
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 5 当前边界：

- 已完成 outbox replayer 的后台补传主链路；
- 已实现多实例并发下的 CAS 抢占与超时锁回收；
- 尚未接入真实 Redis/MySQL 环境做集成验证；
- cloud request 的下游消费和 cloud response/feedback 闭环属于阶段 6；
- 若后续需要更严格的 exactly-once 语义，应在 cloud-gateway 侧使用 `idempotency_key` 做去重确认。

### 日期：2026年06月30日（阶段 6：Cloud feedback 闭环）

已完成：

1. 新增 feedback 处理包：
   - `router-controller/internal/feedback/reward.go`
   - `router-controller/internal/feedback/processor.go`
   - `router-controller/internal/feedback/reward_test.go`
2. 实现 reward 分类：
   - 云端纠错或 cloud pred 与 edge pred 不一致：`1.0`；
   - 云端新增 root cause 或 recommendation：`0.7`；
   - 云端确认高风险：`0.3`；
   - 无新增信息：`0.0`。
3. store 层新增 feedback 闭环方法：
   - `ListUnprocessedFeedback`：扫描未处理反馈；
   - `TryClaimFeedback`：CAS 认领反馈，避免多实例重复处理；
   - `FinalizeFeedbackReward`：写回 reward/reason/processed_at。
4. 实现 feedback processor 主循环：
   - 周期性扫描 `cloud_feedback`；
   - 认领单条 feedback；
   - 计算 reward；
   - 构造在线状态更新所需 inferred event/features；
   - 调用 `state.Manager.ApplyFeedback` 更新 template disagreement 和 gain model；
   - 保存 `router_state` snapshot。
5. 接入 router-controller 进程入口：
   - 默认开启 `FEEDBACK_PROCESSOR_ENABLED=1`；
   - 启动时尝试从最新 `router_state/default` snapshot 恢复在线状态；
   - processor 以独立 goroutine 运行，不阻塞 outbox replayer 或主进程退出。
6. config/README 新增 feedback 参数：
   - `FEEDBACK_PROCESSOR_ENABLED`
   - `FEEDBACK_PROCESSOR_INTERVAL`
   - `FEEDBACK_PROCESSOR_BATCH`
   - `STATE_WINDOW`
   - `GAIN_UCB_ALPHA`
7. 新增 reward 分类单元测试，覆盖四类 reward。

验证命令：

```bash
cd router-controller
gofmt -w cmd/router-controller internal/config internal/store internal/feedback
GOCACHE=/private/tmp/cotriage-go-cache go test ./internal/feedback
```

验证结果：

- `gofmt` 通过；
- `go test ./internal/feedback` 仍被本机 Go 工具链版本不一致阻塞，失败发生在标准库和依赖包编译阶段：

```text
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 6 当前边界：

- 已完成 router-controller 侧 feedback 消费、reward 计算和在线状态更新；
- 已实现同一 feedback 的 CAS 认领，降低多实例重复更新风险；
- 现有轻量 schema 没有 feedback processing lock/failed 状态，当前采用先 claim 再处理；若进程在 claim 后崩溃，该条 feedback 不会自动重放；
- cloud-gateway 写入 `cloud_feedback` 的 API/服务侧逻辑尚未实现，属于后续 server 配套；
- reward 特征目前来自 `cloud_feedback` 展示字段和路由分数字段，后续可在 server 阶段补充更丰富的 cloud evidence。

### 日期：2026年06月30日（阶段 7：server API 配套）

已完成：

1. server 配置补充：
   - `ROUTED_STREAM`
   - `CLOUD_REQ_STREAM`
   - `CLOUD_RESP_STREAM`
2. 新增 server 侧 router store：
   - `server/internal/store/router_model.go`
   - `server/internal/store/router.go`
   - 独立声明 router 展示模型，不跨模块 import `router-controller`；
   - 查询 `router_decisions`、`cloud_outbox`、`router_state_snapshots`。
3. 新增 Router API：
   - `GET /api/v1/router/summary`
   - `GET /api/v1/router/decisions`
   - `GET /api/v1/router/decisions/:event_id`
   - `GET /api/v1/router/outbox`
   - `GET /api/v1/router/state`
   - `GET /api/v1/router/stream`
4. summary 返回：
   - routed total；
   - LOCAL/CLOUD/DEFER total 与 ratio；
   - outbox pending/retry/dlq；
   - P95 route latency；
   - average score/threshold；
   - latest action；
   - latest state snapshot version/time；
   - Redis routed/cloud.req/cloud.resp 的降级补充信息。
5. decisions/outbox 支持分页与过滤：
   - decision filter: `action/template_id/source/limit/offset`；
   - outbox filter: `status/limit/offset`。
6. state API 支持读取最新 snapshot：
   - 可按 `type=router_state` 或 `type=system_state` 查询；
   - response 同时返回 snapshot metadata 与 JSON payload。
7. SSE stream：
   - `GET /api/v1/router/stream` 监听 `ROUTED_STREAM`；
   - 兼容 Redis message 中 `payload` 字段或原始 values；
   - Redis stream 暂无数据时持续 keepalive。
8. degraded-safe 行为：
   - 当 router-controller 未启动、router 表未创建或 MySQL store 不可用时，summary/list/state 返回空数据或 degraded 标记；
   - server 不因 router 缺失而崩溃。
9. README 补充 Router API 与新增 stream 环境变量。

验证命令：

```bash
cd server
gofmt -w cmd/server internal/config internal/api internal/store
GOCACHE=/private/tmp/cotriage-go-cache go test ./internal/store ./internal/api
```

验证结果：

- `gofmt` 通过；
- `go test ./internal/store ./internal/api` 仍被本机 Go 工具链版本不一致阻塞，失败发生在标准库和依赖包编译阶段；该问题此前在 router-controller 与 server Go 测试中均复现：

```text
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

阶段 7 当前边界：

- 已完成 server 后端 API 数据面；
- 尚未实现 cloud-gateway 写入 `cloud_feedback` 的专用 POST API；
- 尚未改造 web 页面展示 router summary/decision/outbox/state；
- SSE 已有 router stream，但前端订阅属于阶段 8；
- 未做真实 Redis/MySQL 集成验证，待本地 Go 工具链修复后补跑。

### 日期：2026年06月30日（阶段 8：web 可视化配套）

已完成：

1. web API 封装新增 router 数据接口：
   - `getRouterSummary`
   - `listRouterDecisions`
   - `listRouterOutbox`
   - `getRouterState`
   - `subscribeRouterEvents`
2. Dashboard 指标区新增 RAISE 核心指标：
   - RAISE 路由总量；
   - Cloud 路由比例；
   - Defer 队列；
   - P95 路由时延。
3. Dashboard 新增 RAISE 路由状态面板：
   - LOCAL/CLOUD/DEFER 分布；
   - average score；
   - average threshold；
   - latest action；
   - latest state snapshot version；
   - system/router state snapshot 预览。
4. Dashboard 新增 Outbox 补偿队列面板：
   - status；
   - event_id；
   - priority_score；
   - attempt/max_attempts；
   - defer_reason 或 last_error。
5. 事件表新增路由列：
   - 每条实时事件展示 route action；
   - 根据 `LOCAL/CLOUD/DEFER` 使用不同颜色；
   - 事件展开详情展示 score、threshold、reason、cloud health。
6. 接入 router SSE：
   - 订阅 `/api/v1/router/stream`；
   - routed event 到达后更新本地 decision map；
   - 不影响原有 inferred event stream。
7. 保持降级可用：
   - 当 server 返回 degraded 或 router 数据为空时，页面展示空状态和 degraded 标记；
   - outbox 无积压时展示空状态；
   - router state 缺失时展示 `暂无 router state snapshot`。

验证命令：

```bash
cd web
npm run build
```

验证结果：

- `vue-tsc -b` 通过；
- `vite build` 通过；
- 构建过程中仍有 AvatarKit WASM 动态 URL 与大 chunk size 警告，属于当前前端依赖体积/wasm 加载方式导致，不影响本次 router 可视化代码构建。

阶段 8 当前边界：

- 已完成 Dashboard 层 router 可观测展示；
- 尚未新增独立 router 管理页面；
- 尚未实现策略编辑器；
- router state 当前只做 JSON 预览，后续可拆解为 budget/network/queue/feedback 等结构化子面板；
- 前端已订阅 router SSE，但真实联调仍依赖 server、Redis、MySQL 与 router-controller 同时运行。

### 日期：2026年06月30日（阶段 9：Kubernetes 与本地运行）

已完成：

1. 新增 router-controller 镜像构建入口：
   - `router-controller/Dockerfile`
   - `router-controller/Makefile` 增加 `build` 与 `docker-build`
2. 新增 Kubernetes 部署清单：
   - `charts/router-controller/deployment.yaml`
   - 采用单副本 Deployment；
   - 注入 Redis/MySQL/stream/budget/replayer/feedback 参数；
   - 使用 `metadata.name` 作为 `CONSUMER_NAME` 和 outbox lock worker；
   - 使用 `mysql-secret` 中的 `MYSQL_PASSWORD` 组装 `MYSQL_DSN`。
3. 更新本地 port-forward 脚本：
   - `script/forward-datastores.sh` 新增 server 端口转发；
   - 默认 `SERVER_LOCAL_PORT=8080`；
   - 保持 MySQL、Redis、onnx-classifier 原有转发逻辑。
4. 更新 VSCode 调试配置：
   - 新增 `router-controller` debug target；
   - server debug target 补充 routed/cloud stream 环境变量。
5. 更新运行文档：
   - `router-controller/README.md` 增加本地启动、Redis 注入、stream 查看、MySQL 表查看、Kubernetes 部署命令；
   - `sop/router-controller运行SOP.md` 增加完整本地/集群运行与排查步骤；
   - 根 `README.md` 增加 router-controller 文档入口。

验证命令：

```bash
node -e "JSON.parse(require('fs').readFileSync('.vscode/launch.json','utf8')); console.log('launch.json ok')"
ruby -e "require 'yaml'; YAML.load_file('charts/router-controller/deployment.yaml'); puts 'router deployment yaml ok'"
zsh -n script/forward-datastores.sh
kubectl apply --dry-run=client --validate=false -f charts/router-controller/deployment.yaml
```

验证结果：

- `launch.json` JSON 解析通过；
- `charts/router-controller/deployment.yaml` YAML 解析通过；
- `script/forward-datastores.sh` shell 语法检查通过；
- `kubectl dry-run` 被当前 sandbox 网络权限阻塞，kubectl 尝试访问 `https://127.0.0.1:60083/api` 获取 apiserver discovery 信息：

```text
dial tcp 127.0.0.1:60083: connect: operation not permitted
```

阶段 9 当前边界：

- 已完成 router-controller 的本地运行、调试与 Kubernetes 部署文件；
- 尚未实际 build/push router-controller 镜像；
- 尚未在 Kubernetes 集群中执行 `kubectl apply` 和 rollout 验证；
- 若后续启用多副本，需要增加 leader election 或按 template/source 分片，否则在线状态会存在多副本竞争更新风险。

### 日期：2026年07月01日（主链路 worker 补齐）

背景：

- Dashboard 中 “RAISE 路由总量” 一直为 0；
- 排查后确认 router-controller 仅启动了 outbox replayer 与 feedback processor，尚未启动 `inferred -> RAISE -> routed` 主消费链路。

已完成：

1. 新增主链路 worker：
   - `router-controller/internal/worker/worker.go`
2. worker 启动时创建 Redis consumer group：
   - stream: `cotriage.events.inferred`
   - group: `router-g`
   - 初次创建从 `0` 开始，便于消费已有 inferred 历史消息。
3. 实现 inferred event 消费：
   - `XREADGROUP` 读取 `cotriage.events.inferred`；
   - 支持 `payload` JSON 解码；
   - 解码失败写入 `DLQ` 并 ACK，避免坏消息卡住主链路。
4. 实现 RAISE 决策主流程：
   - `state.Manager.BuildStats` 构造在线统计；
   - `runtime.Builder.Build` 构造系统态；
   - `raise.Engine.Decide` 输出 `LOCAL/CLOUD/DEFER`；
   - `state.Manager.Observe` 更新滑动窗口与 cloud ratio。
5. 实现决策落库：
   - 写入 `router_decisions`；
   - 保存 action、reason、score、threshold、cloud health、raise version 等展示字段。
6. 实现 Redis 输出：
   - 所有动作写入 `cotriage.events.routed`；
   - `CLOUD` 写入 `cotriage.events.cloud.req`；
   - `DEFER` 写入 `cloud_outbox`；
   - 成功完成后 ACK inferred message。
7. 接入 router-controller main：
   - 启动时创建 state manager、runtime builder、RAISE engine；
   - 以 goroutine 启动主 worker；
   - outbox replayer 与 feedback processor 复用同一个 state manager。
8. 新增 worker 配置：
   - `WORKER_BLOCK_TIMEOUT`
   - `WORKER_BATCH`
   - `ROUTER_STREAM_MAXLEN`
   - `.vscode/launch.json` 与 README 已同步。

验证命令：

```bash
cd router-controller
gofmt -w internal/worker cmd/router-controller internal/config
GOCACHE=/private/tmp/cotriage-go-cache go test ./internal/worker
node -e "JSON.parse(require('fs').readFileSync('.vscode/launch.json','utf8')); console.log('launch.json ok')"
```

验证结果：

- `gofmt` 通过；
- `launch.json` JSON 解析通过；
- `go test ./internal/worker` 仍被本机 Go 工具链版本不一致阻塞，失败发生在标准库和依赖包编译阶段：

```text
compile: version "go1.25.3" does not match go tool version "go1.26.1"
```

当前边界：

- 主链路已接入，重新启动 router-controller 后应开始写 `router_decisions` 和 `cotriage.events.routed`；
- 如果 consumer group 之前已经存在且 last delivered ID 在尾部，历史消息不会自动重放；需要新注入 inferred event，或删除/重建 consumer group；
- 当前先实现单副本 worker，后续多副本需补 pending claim 与在线状态分片/leader 机制。

### 日期：2026年07月01日（Dashboard router 可解释性修正）

背景：

- Dashboard 原先直接展示 `average_score`、`average_threshold`、`latest_action`、`latest_state_snapshot_version` 和完整 system state JSON；
- 这些字段更像工程调试输出，缺少面向用户的含义解释，容易让操作者不知道“应该看什么、如何判断是否异常”。

已完成：

1. 将 RAISE 状态面板字段改为业务化命名：
   - `average_score` 展示为“上云压力”；
   - `average_threshold` 展示为“路由门槛”；
   - `latest_action` 展示为“最近去向”；
   - `latest_state_snapshot_version` 展示为“运行版本”。
2. 为核心指标增加 hover 解释：
   - 解释上云压力与路由门槛的关系；
   - 解释 `LOCAL/CLOUD/DEFER` 的动作含义；
   - 解释状态快照版本的用途。
3. 移除页面上的原始 system state JSON 预览，改为结构化状态卡片：
   - 云端健康；
   - 失败率；
   - 队列压力；
   - Outbox；
   - 分钟预算；
   - 日预算。
4. 保留底层诊断价值，但避免把论文/算法内部字段直接暴露给普通运维视角。

当前边界：

- 本次先修正 Dashboard 首页的 router 面板；
- 后续若新增独立 router 管理页，可继续补充更细的 budget/network/feedback 子面板和字段级解释。
