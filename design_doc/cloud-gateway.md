# cloud-gateway 云端推理设计

## 1. 设计目标

`cloud-gateway` 是 CoTriage 云边协同链路中的云端推理组件，位于 `router-controller` 之后。`edge-infer` 负责快速边缘分类，`router-controller` 负责判断事件是否需要上云，而 `cloud-gateway` 负责对高价值事件执行更强的云端诊断，并把诊断结果反馈给 router 的在线收益模型。

本模块不是简单的 “调用一次 LLM”。它需要回答四个问题：

1. 云端应该如何消费 `cotriage.events.cloud.req`，保证幂等、可恢复、可观测；
2. 云端 LLM 应该输出什么结构化诊断，才能真正补充边缘结果；
3. 云端结果如何进入 `cloud_feedback`，为 RAISE 的在线学习提供奖励信号；
4. 云端异常、限流、超时、弱网和 JSON 解析失败时如何降级，不破坏主链路。

因此，cloud-gateway 的设计目标是：

- 对 router 发来的 `CLOUD` 和 outbox replay 请求进行可靠消费；
- 基于云端大模型生成结构化根因、处置建议、解释证据和置信度；
- 对比边缘推理与云端推理，计算可解释 reward；
- 写入 `cloud_feedback`，供 `router-controller` feedback processor 消费；
- 发布 `cotriage.events.cloud.resp`，用于后续页面展示、审计和实验复现；
- 建立独立 `cloud_provider` 模型管理体系，不把 API key 写死到代码中。

## 2. 系统位置

整体链路如下：

```text
log-collector
  -> cotriage.events.raw
  -> edge-infer
  -> cotriage.events.inferred
  -> router-controller / RAISE
      -> LOCAL  -> cotriage.events.routed
      -> DEFER  -> cloud_outbox -> outbox replayer
      -> CLOUD  -> cotriage.events.cloud.req
  -> cloud-gateway
      -> cotriage.events.cloud.resp
      -> cloud_feedback
  -> router-controller feedback processor
  -> RAISE online gain update
```

边界划分：

- `router-controller` 只决定是否上云，不调用云端模型；
- `cloud-gateway` 只处理已被 router 选择的上云请求，不重新做路由决策；
- `server` 负责 API、云端模型配置管理和 cloud-gateway 配置下发；
- `cloud-gateway` 是独立 service，通过 server 的 cloud config API 拉取模型配置；
- `web` 后续展示云端诊断结果、云边分歧、token 成本和 reward。

cloud-gateway 必须拆成独立服务，不放入 server 进程。原因是：

- 云端推理会引入长耗时、外部 API 限流、超时和重试，不应阻塞 server API；
- cloud-gateway 需要独立扩缩容、限流和熔断；
- 云端调用可能依赖不同网络出口、密钥和资源配额，部署边界应和 server 分离；
- server 只作为配置控制面，cloud-gateway 作为数据面 worker；
- 该边界与 `edge-infer` 保持一致：模型配置在 server 管理，worker 通过拉取配置热更新。

配置关系：

```text
server
  -> MySQL cloud_providers
  -> GET /api/v1/cloud-config
  -> cloud-gateway polling / hot reload
```

cloud-gateway 写入 `cloud_feedback` 仍然属于“server 负责云端反馈”的系统语义：即 cloud-gateway 作为云端推理执行服务，按 server 下发的配置和 schema 写入 server 管理的 MySQL 表。

## 3. 输入契约

### 3.1 Redis Stream

cloud-gateway 消费：

```text
stream: cotriage.events.cloud.req
group: cloud-gateway-g
consumer: cloud-gateway-{instance}
```

Redis message fields：

```json
{
  "event_id": "01J...",
  "request_id": "cloud-01J...",
  "idempotency_key": "router-cloud-01J...",
  "payload_json": "{...}",
  "source": "router_controller"
}
```

其中 `payload_json` 为 router 输出的 `RoutedEvent`：

```json
{
  "input": {
    "event_id": "01J...",
    "source": "host-5-worker",
    "equipment": "LinuxServer",
    "event_ts": "2026-07-01T08:00:00Z",
    "raw_log": "[error] connect to 10.0.0.2:3306 failed: connection refused",
    "template_id": "tpl-net-001",
    "template_text": "connect to <IP>:<NUM> failed: connection refused",
    "edge_pred": "NET_ERROR",
    "severity": 90,
    "confidence": 0.42,
    "uncertainty": 0.58,
    "evidence": ["connection refused", "3306"],
    "advice_code": "CHECK_NETWORK_AND_SERVICE",
    "advice": "检查目标服务状态、端口监听和网络策略",
    "inference_path": "MODEL",
    "model_version": "edge-log-classifier-v1"
  },
  "decision": {
    "action": "CLOUD",
    "reason": "HIGH_SCORE_SENDABLE",
    "score": 0.83,
    "threshold": 0.61,
    "policy_ref": "raise-v1",
    "raise_version": "raise-v1"
  },
  "system_state": {
    "CloudHealth": "UP",
    "RTTMS": 35,
    "FailRate": 0.01,
    "CloudQueueLag": 4,
    "OutboxLag": 0,
    "MinuteBudgetRemaining": 58
  },
  "routed_at": "2026-07-01T08:00:00.123Z",
  "router": "router-controller-1",
  "schema_version": "router.routed.v1"
}
```

### 3.2 幂等键

幂等语义以 `idempotency_key` 为准：

```text
idempotency_key = router-cloud-{event_id}
```

处理规则：

- 同一 `event_id` 最多写入一条 `cloud_feedback`；
- 重复收到同一 `idempotency_key` 时，如果反馈已存在且 cloud response 已发布，可以直接 ACK；
- 如果上一次处理失败但未写反馈，允许重新调用云端；
- 如果调用云端成功但写 DB 失败，不 ACK Redis message，等待重试；
- 如果写 DB 成功但发布 `cloud.resp` 失败，优先保证 DB 反馈闭环，再在后续补偿机制中补发响应事件。

## 4. 输出契约

### 4.1 cloud response stream

cloud-gateway 发布：

```text
stream: cotriage.events.cloud.resp
```

payload：

```json
{
  "schema_version": "cloud.response.v1",
  "event_id": "01J...",
  "request_id": "cloud-01J...",
  "idempotency_key": "router-cloud-01J...",
  "status": "OK",
  "edge": {
    "pred": "NET_ERROR",
    "confidence": 0.42,
    "severity": 90,
    "advice_code": "CHECK_NETWORK_AND_SERVICE"
  },
  "cloud": {
    "model": "deepseek-chat",
    "pred": "NET_ERROR",
    "root_cause": "目标 MySQL 服务端口未监听或 Service/Pod 已重启，导致连接被拒绝。",
    "recommendation": "先检查目标 Pod/Service 状态，再确认 3306 端口监听、网络策略和最近重启事件。",
    "explanation": "日志中出现 connection refused 和 3306，说明 TCP 已到达目标地址但服务未接受连接。",
    "confidence": 0.86,
    "evidence": ["connection refused", "3306", "mysql"]
  },
  "feedback": {
    "corrected_edge": false,
    "added_actionable_info": true,
    "confirmed_high_risk": true,
    "reward": 0.62,
    "reward_reason": "ADDED_ACTIONABLE_INFO"
  },
  "usage": {
    "prompt_tokens": 620,
    "completion_tokens": 180,
    "total_tokens": 800,
    "cost": 0.0012,
    "latency_ms": 1240
  },
  "processed_at": "2026-07-01T08:00:01.500Z"
}
```

失败响应：

```json
{
  "schema_version": "cloud.response.v1",
  "event_id": "01J...",
  "request_id": "cloud-01J...",
  "status": "FAILED",
  "error_code": "LLM_TIMEOUT",
  "error_message": "cloud model request timed out",
  "retryable": true,
  "processed_at": "2026-07-01T08:00:01.500Z"
}
```

### 4.2 MySQL feedback

cloud-gateway 写入现有 `cloud_feedback` 表。字段映射如下：

```text
event_id              <- input.event_id
request_id            <- cloud request request_id
idempotency_key       <- cloud request idempotency_key
source                <- input.source
equipment             <- input.equipment
template_id           <- input.template_id
edge_pred             <- input.edge_pred
edge_confidence       <- input.confidence
route_action          <- decision.action
route_score           <- decision.score
route_threshold       <- decision.threshold
cloud_model           <- provider.model
cloud_pred            <- cloud.pred
root_cause            <- cloud.root_cause
recommendation        <- cloud.recommendation
explanation           <- cloud.explanation
cloud_confidence      <- cloud.confidence
cloud_latency_ms      <- usage.latency_ms
token_usage           <- usage.total_tokens
cost                  <- usage.cost
reward                <- feedback.reward
reward_reason         <- feedback.reward_reason
corrected_edge        <- feedback.corrected_edge
added_actionable_info <- feedback.added_actionable_info
confirmed_high_risk   <- feedback.confirmed_high_risk
feedback_json         <- full cloud response payload
processed_by_router   <- false
created_at            <- now
```

注意：`processed_by_router` 必须由 cloud-gateway 写为 `false`，后续由 router-controller feedback processor claim 并置为 `true`。

## 5. 云端推理算法

云端推理算法命名为：

```text
CLOUD-DIAG: Contextual LLM-based Operational Diagnosis with Utility-grounded Feedback
```

中文称为“上下文增强的云端运维诊断与效用反馈算法”。

它的目标不是替代 edge-infer，而是为 router 选择出的高价值事件补充更强诊断能力。CLOUD-DIAG 由四步组成：

```text
parse -> context build -> constrained generation -> utility feedback
```

### 5.1 事件解析

对云端请求 `q_t`，解析得到：

```text
e_t = q_t.input
d_t = q_t.decision
s_t = q_t.system_state
```

其中 `e_t` 是触发上云的中心事件。云端根因推理不应只依赖这一条日志，而应围绕中心事件构造一个上下文日志包：

```text
G_t = {e_t} union N_time(e_t) union N_template(e_t) union N_source(e_t) union N_equipment(e_t)
```

其中：

- `N_time(e_t)`：同一时间窗口内的近邻日志；
- `N_template(e_t)`：同模板或相似模板日志；
- `N_source(e_t)`：同 source / pod / node 的相关日志；
- `N_equipment(e_t)`：同设备或同车间的相关日志。

第一轮实现中，cloud-gateway 可从 Redis `cotriage.events.inferred` 或 `cotriage.events.routed` 读取最近日志，按时间窗口和字段过滤形成 `G_t`。后续如果 MySQL 中沉淀事件表，可切换为 DB 查询以获得更稳定的历史上下文。

边缘输出记为：

```text
y_e = (pred_e, conf_e, severity_e, advice_e, evidence_e)
```

云端模型输出记为：

```text
y_c = (pred_c, root_cause_c, recommendation_c, explanation_c, conf_c, evidence_c)
```

### 5.2 上下文日志聚合

云端模型光靠一两条日志通常只能做文本解释，难以做根因推理。因此 CLOUD-DIAG 在调用模型前先构造上下文日志包 `G_t`，再做摘要压缩。

候选日志打分：

```text
rel(e_i, e_t) =
  eta_time * exp(- |ts_i - ts_t| / tau)
  + eta_template * I(template_i = template_t)
  + eta_source * I(source_i = source_t)
  + eta_equipment * I(equipment_i = equipment_t)
  + eta_pred * I(edge_pred_i = edge_pred_t)
  + eta_sev * severity_i
```

默认权重：

```text
eta_time = 0.25
eta_template = 0.20
eta_source = 0.20
eta_equipment = 0.15
eta_pred = 0.10
eta_sev = 0.10
```

聚合约束：

```text
time_window = CLOUD_CONTEXT_WINDOW
max_events = CLOUD_CONTEXT_MAX_EVENTS
max_tokens = CLOUD_CONTEXT_MAX_TOKENS
```

第一轮建议默认：

```text
CLOUD_CONTEXT_WINDOW=5m
CLOUD_CONTEXT_MAX_EVENTS=20
CLOUD_CONTEXT_MAX_TOKENS=3000
```

上下文日志包应包含：

```json
{
  "center_event": {},
  "related_events": [
    {
      "event_id": "01J...",
      "time_delta_ms": -1200,
      "source": "host-5-worker",
      "equipment": "LinuxServer",
      "template_text": "connect to <IP>:<NUM> failed: connection refused",
      "edge_pred": "NET_ERROR",
      "severity": 82,
      "confidence": 0.71,
      "raw_log": "..."
    }
  ],
  "aggregate": {
    "event_count": 12,
    "top_templates": ["tpl-net-001", "tpl-service-004"],
    "top_sources": ["host-5-worker"],
    "top_preds": ["NET_ERROR", "SERVICE_ERROR"],
    "max_severity": 94,
    "time_span_ms": 286000
  }
}
```

上下文日志包不是越大越好。超过 token 预算时按以下顺序保留：

1. 中心事件；
2. 同 source 且时间最近的异常日志；
3. 同 template 的高严重度日志；
4. 同 equipment 的异常增长日志；
5. 其余日志只保留聚合统计。

### 5.3 Prompt 上下文构造

云端 prompt 不直接塞完整系统状态，而是构造紧凑诊断上下文：

```text
ctx_t = [
  center raw_log,
  center template_text,
  related event summary,
  top related logs,
  aggregate statistics,
  equipment,
  source,
  edge_pred,
  severity,
  edge_confidence,
  edge_evidence,
  edge_advice,
  route_reason,
  route_score,
  cloud_health,
  recent system hints
]
```

上下文构造原则：

- 原始日志最多保留 `CLOUD_RAW_LOG_MAX_CHARS` 字符，避免 token 爆炸；
- 相关日志最多保留 `CLOUD_CONTEXT_MAX_EVENTS` 条；
- 如果相关日志过多，先聚合为模板、source、分类和严重度统计；
- 敏感字段进入云端前必须脱敏；
- 只传与诊断有关的 router state，不传完整内部 JSON；
- prompt 中明确要求输出 JSON，禁止输出 Markdown 长文；
- 对 high severity 事件要求给出可执行处置步骤；
- 对低置信度事件要求说明不确定性来源。

### 5.4 结构化输出约束

云端模型必须输出 JSON：

```json
{
  "pred": "NET_ERROR",
  "root_cause": "目标服务端口未监听或服务实例异常重启。",
  "recommendation": "检查目标 Pod/Service、端口监听、网络策略和最近重启事件。",
  "explanation": "connection refused 表示 TCP 连接到达目标但没有进程接受连接。",
  "confidence": 0.86,
  "evidence": ["connection refused", "3306"]
}
```

允许的 `pred` 枚举与 edge-infer 对齐：

```text
NORMAL
NET_ERROR
DISK_ERROR
CPU_HIGH
MEMORY_HIGH
SERVICE_ERROR
CONFIG_ERROR
SECURITY_RISK
UNKNOWN
```

解析失败时进入修复策略：

1. 尝试提取第一个 JSON object；
2. 尝试用轻量 JSON repair；
3. 如果仍失败，返回 `cloud_pred=UNKNOWN`，把原始文本放入 `explanation`，`cloud_confidence=0.3`；
4. 记录 `reward_reason=UNSTRUCTURED_RESPONSE`，reward 不超过 0.1。

### 5.5 置信度计算

云端置信度优先使用模型结构化输出中的 `confidence`，但需要裁剪：

```text
conf_c = clip(conf_model, 0, 1)
```

如果模型未输出置信度，则根据输出质量估计：

```text
conf_c = 0.4
       + 0.2 * I(root_cause not empty)
       + 0.2 * I(recommendation not empty)
       + 0.1 * I(evidence not empty)
       + 0.1 * I(pred_c != UNKNOWN)
```

最终裁剪到 `[0, 0.95]`，避免云端模型无校准时过度自信。

### 5.6 云边分歧

定义云边分类分歧：

```text
disagree_t = I(pred_c != pred_e and pred_c != UNKNOWN)
```

定义云端是否补充可执行信息：

```text
actionable_t = I(root_cause_c not empty) * I(recommendation_c not empty)
```

定义是否确认高风险：

```text
highrisk_t = I(severity_e >= theta_sev) * I(conf_c >= theta_conf)
```

其中第一轮默认：

```text
theta_sev = 75
theta_conf = 0.65
```

### 5.7 Reward 计算

cloud feedback 的 reward 是 router 在线收益学习的核心输入。定义：

```text
R_t = alpha_d * disagree_t
    + alpha_a * actionable_t
    + alpha_h * highrisk_t
    + alpha_c * confidence_gain_t
    - beta_l  * latency_penalty_t
    - beta_k  * token_cost_t
```

其中：

```text
confidence_gain_t = max(0, conf_c - conf_e)
latency_penalty_t = min(1, latency_ms / CLOUD_LATENCY_SLA_MS)
token_cost_t      = min(1, total_tokens / CLOUD_TOKEN_BUDGET_PER_REQ)
```

默认权重：

```text
alpha_d = 0.35
alpha_a = 0.30
alpha_h = 0.20
alpha_c = 0.15
beta_l  = 0.10
beta_k  = 0.05
```

最终：

```text
reward = clip(R_t, -1, 1)
```

reward reason 取主导原因：

```text
if disagree_t == 1:
  reward_reason = "CORRECTED_EDGE"
else if actionable_t == 1:
  reward_reason = "ADDED_ACTIONABLE_INFO"
else if highrisk_t == 1:
  reward_reason = "CONFIRMED_HIGH_RISK"
else if reward < 0:
  reward_reason = "LOW_UTILITY_OR_HIGH_COST"
else:
  reward_reason = "LOW_INCREMENTAL_GAIN"
```

这个 reward 会被 router-controller 消费，用于更新 template-level gain 和 UCB bonus，使后续类似事件的上云概率自适应变化。

## 6. Prompt 设计

### 6.1 System prompt

```text
You are a senior AIOps diagnosis assistant for industrial edge-cloud log triage.
You must return only valid JSON.
Do not invent facts not supported by the log or provided context.
If the evidence is insufficient, set pred to UNKNOWN and explain the uncertainty.
```

### 6.2 User prompt 模板

```text
Diagnose the following operational log event.

Raw log:
{{raw_log}}

Template:
{{template_text}}

Related log context:
{{related_event_summary}}

Top related logs:
{{top_related_logs}}

Equipment:
{{equipment}}

Source:
{{source}}

Edge inference:
- pred: {{edge_pred}}
- severity: {{severity}}
- confidence: {{confidence}}
- uncertainty: {{uncertainty}}
- evidence: {{evidence}}
- advice: {{advice}}

Router decision:
- reason: {{route_reason}}
- score: {{route_score}}
- threshold: {{route_threshold}}

Return JSON with this schema:
{
  "pred": "NORMAL|NET_ERROR|DISK_ERROR|CPU_HIGH|MEMORY_HIGH|SERVICE_ERROR|CONFIG_ERROR|SECURITY_RISK|UNKNOWN",
  "root_cause": "one concise root cause hypothesis",
  "recommendation": "concrete next action for an operator",
  "explanation": "why this diagnosis follows from the evidence",
  "confidence": 0.0,
  "evidence": ["short evidence phrase"]
}
```

### 6.3 脱敏规则

云端调用前应执行最小脱敏：

```text
IPv4           -> <IP>
email          -> <EMAIL>
token/key      -> <SECRET>
long hex/base64 -> <HASH>
password-like  -> <SECRET>
```

注意：端口号、错误码、服务名通常对诊断有价值，第一轮不默认脱敏。

## 7. 可靠性设计

### 7.1 ACK 时机

cloud-gateway 对 `cloud.req` 的 ACK 必须满足：

```text
LLM call completed
AND cloud_feedback written successfully
AND cloud.resp published successfully
```

如果 `cloud.resp` 发布失败但 DB 已写成功，可选择：

- 第一轮：不 ACK，依靠幂等 DB 写入重试，直到 resp 发布成功；
- 后续优化：新增 `cloud_response_outbox`，DB 与 resp 发布解耦。

第一轮建议采用“不 ACK 重试 + 幂等写 DB”的简单可靠策略。

### 7.2 重试与 DLQ

错误分类：

```text
RETRYABLE:
  LLM_TIMEOUT
  RATE_LIMIT
  PROVIDER_5XX
  REDIS_TEMPORARY_ERROR
  MYSQL_TEMPORARY_ERROR

NON_RETRYABLE:
  BAD_PAYLOAD
  JSON_SCHEMA_INVALID
  PROVIDER_CONFIG_MISSING
  PRIVACY_POLICY_DENIED
```

重试策略：

```text
retry_delay = min(max_delay, base_delay * 2^attempt + jitter)
```

第一轮如果直接使用 Redis consumer group pending 重试，则不新增表；达到最大失败次数后写入 `cotriage.events.dlq`：

```json
{
  "source_stream": "cotriage.events.cloud.req",
  "message_id": "...",
  "event_id": "01J...",
  "request_id": "cloud-01J...",
  "error_code": "BAD_PAYLOAD",
  "error": "payload_json is invalid"
}
```

### 7.3 Pending claim

为支持 cloud-gateway 重启和多实例，cloud-gateway worker 后续应支持：

```text
XAUTOCLAIM cloud.req group cloud-gateway-g min-idle-time
```

第一轮可以先实现主消费链路；如果部署多实例或需要强可靠，再补 pending claim。

### 7.4 限流

cloud-gateway 必须有本地限流：

```text
CLOUD_WORKER_CONCURRENCY
CLOUD_PROVIDER_QPS
CLOUD_PROVIDER_TIMEOUT
CLOUD_MAX_TOKENS
```

当 provider 返回 rate limit：

- 不写成功 feedback；
- 不 ACK；
- 记录失败日志；
- 依靠重试或 pending reclaim 继续处理。

## 8. Provider 接入

cloud-gateway 必须使用独立的 `cloud_providers` 表，不能复用或混用 `edge_providers`。原因是：

- edge provider 面向边缘小模型，cloud provider 面向云端强模型，两者 SLA、成本、限流、密钥和部署边界不同；
- router 论文实验需要单独统计 cloud model 的 token、cost、latency 和 reward；
- 后续 web 管理页也应把边缘模型和云端模型分开展示，避免操作人员误配；
- cloud-gateway 需要像 edge-infer 一样拉取配置，但配置接口必须独立，不能混在 `/api/v1/edge-config` 中。

`cloud_providers` 字段：

```text
id
name
provider              openai_compat | deepseek | qwen | custom_http
endpoint
model
api_key_env           只保存环境变量名，不保存明文 key
timeout_ms
max_tokens
temperature
top_p
seed
health_path
response_path
status                draft | published | disabled | deleted
is_active
config_version
etag
created_at
updated_at
deleted_at
```

API key 规则：

- 数据库只保存 `api_key_env`，例如 `DEEPSEEK_API_KEY`；
- cloud-gateway 收到配置后，按 `api_key_env` 从自身环境变量读取；
- 不把 key 写入代码、配置提交或前端响应。

配置拉取语义：

```text
cloud-gateway -> server GET /api/v1/cloud-config
```

server 返回 active cloud provider 的非密钥配置和版本号，cloud-gateway 本地缓存并按 `etag` 或 `config_version` 热更新。

## 9. Server API 设计

server 是 cloud-gateway 的控制面，至少需要提供独立 cloud config API。该接口与 `/api/v1/edge-config` 分离，避免边缘推理配置和云端推理配置混杂。

### 9.1 云端配置拉取

```text
GET /api/v1/cloud-config
```

用途：

- cloud-gateway 启动时拉取 active cloud provider；
- cloud-gateway 周期性轮询该接口，根据 `etag` 或 `config_version` 判断是否热更新；
- 不返回 API key 明文，只返回 `api_key_env`。

响应：

```json
{
  "enabled": true,
  "config_version": 12,
  "etag": "W/\"cloud-config-12-a8f31c\"",
  "provider": {
    "id": 3,
    "name": "deepseek-prod",
    "provider": "openai_compat",
    "endpoint": "https://api.deepseek.com/v1/chat/completions",
    "model": "deepseek-chat",
    "api_key_env": "DEEPSEEK_API_KEY",
    "timeout_ms": 20000,
    "max_tokens": 512,
    "temperature": 0.1,
    "top_p": 0.9,
    "response_path": "choices[0].message.content"
  },
  "prompt": {
    "version": "cloud-diag-prompt-v1",
    "raw_log_max_chars": 2000,
    "context_window": "5m",
    "context_max_events": 20,
    "context_max_tokens": 3000
  },
  "reward": {
    "latency_sla_ms": 5000,
    "token_budget_per_req": 2000
  },
  "generated_at": "2026-07-01T08:01:00Z"
}
```

无 active provider 时：

```json
{
  "enabled": false,
  "reason": "NO_ACTIVE_CLOUD_PROVIDER",
  "generated_at": "2026-07-01T08:01:00Z"
}
```

### 9.2 云端 Provider 管理

```text
GET    /api/v1/cloud-providers
POST   /api/v1/cloud-providers
PATCH  /api/v1/cloud-providers/{id}
DELETE /api/v1/cloud-providers/{id}
POST   /api/v1/cloud-providers/{id}/activate
```

语义与边缘模型管理类似，但落表为 `cloud_providers`，且不与 `edge_providers` 共表。

### 9.3 云端推理状态

```text
GET /api/v1/cloud-gateway/status
```

响应：

```json
{
  "enabled": true,
  "provider": "deepseek",
  "model": "deepseek-chat",
  "cloud_req_lag": 12,
  "cloud_resp_total": 1024,
  "last_success_at": "2026-07-01T08:00:01Z",
  "last_error": "",
  "generated_at": "2026-07-01T08:01:00Z"
}
```

该状态由 cloud-gateway 定期 heartbeat 写入 Redis 或 MySQL，server API 只负责读取展示。

### 9.4 云端反馈列表

```text
GET /api/v1/cloud-feedback?event_id=&template_id=&limit=50&offset=0
```

用于 Dashboard 和事件详情展示。

### 9.5 单事件云端诊断

```text
GET /api/v1/cloud-feedback/{event_id}
```

用于事件展开详情：

- 云端分类；
- 根因；
- 建议；
- 云边是否分歧；
- reward；
- token 成本；
- 云端延迟。

## 10. 配置项

server 需新增控制面配置：

```text
CLOUD_REQ_STREAM=cotriage.events.cloud.req
CLOUD_RESP_STREAM=cotriage.events.cloud.resp
CLOUD_CONFIG_DEFAULT_ENABLED=true
CLOUD_CONFIG_CACHE_MAX_AGE=30s
```

cloud-gateway service 需新增运行配置：

```text
CLOUD_GATEWAY_ENABLED=true
SERVER_BASE_URL=http://server:8080
CLOUD_CONFIG_ENDPOINT=/api/v1/cloud-config
CLOUD_CONFIG_POLL_INTERVAL=10s
REDIS_ADDR=redis:6379
CLOUD_REQ_STREAM=cotriage.events.cloud.req
CLOUD_RESP_STREAM=cotriage.events.cloud.resp
DLQ_STREAM=cotriage.events.dlq
CLOUD_GATEWAY_GROUP=cloud-gateway-g
CLOUD_GATEWAY_CONSUMER=cloud-gateway-1
CLOUD_GATEWAY_BLOCK_TIMEOUT=2s
CLOUD_GATEWAY_BATCH=10
CLOUD_GATEWAY_CONCURRENCY=2
CLOUD_CONTEXT_SOURCE_STREAM=cotriage.events.inferred
CLOUD_CONTEXT_WINDOW=5m
CLOUD_CONTEXT_MAX_EVENTS=20
CLOUD_CONTEXT_MAX_TOKENS=3000
CLOUD_RAW_LOG_MAX_CHARS=2000
CLOUD_LATENCY_SLA_MS=5000
CLOUD_TOKEN_BUDGET_PER_REQ=2000
CLOUD_STREAM_MAXLEN=100000
```

其中真实 API key 通过环境变量提供：

```text
DEEPSEEK_API_KEY=...
```

## 11. 数据库设计

第一轮需要两类表：

1. `cloud_providers`：server 管理云端模型配置；
2. `cloud_feedback`：cloud-gateway 写入云端诊断结果和 reward。

### 11.1 cloud_providers

必须新增 `cloud_providers` 表，不与 `edge_providers` 混用：

```text
cloud_providers
  id
  name unique
  provider
  endpoint
  model
  api_key_env
  timeout_ms
  max_tokens
  temperature
  top_p
  seed
  health_path
  response_path
  status
  is_active
  config_version
  etag
  created_at
  updated_at
  created_by
  updated_by
  deleted_at
  last_publish_at
```

约束：

```text
sum(is_active=true and deleted_at is null) <= 1
is_active=true -> status=published
status in (draft, disabled, deleted) -> is_active=false
api_key_env not empty
```

### 11.2 cloud_feedback

当前 `cloud_feedback` 表已满足第一轮主链路闭环，不需要新增大量字段。原因：

- 云端推理结果本身是低频、高价值事件，不应把所有 prompt、raw completion、解析过程都展开为热表列；
- 展示和 router feedback 只需要核心字段；
- 完整复现实验数据可以通过 `cloud.resp` stream、server 日志和 `feedback_json` 获得。

第一轮沿用现有 `cloud_feedback` 字段：

```text
cloud_feedback
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

### 11.3 上下文日志存储原则

云端推理时会聚合上下文日志，但第一轮不新增 `cloud_context_events` 表。理由：

- 相关日志可从 Redis stream 近窗口读取；
- `cloud.resp.feedback_json` 可保存本次模型实际使用的摘要上下文；
- 如果每次云端推理都把上下文日志展开写表，会引入明显写放大；
- 论文复现可通过 stream export 和 feedback_json 重建。

如果后续需要云端诊断历史和失败审计，可新增：

```text
cloud_invocations
  id
  event_id
  request_id
  provider
  model
  status
  prompt_hash
  response_hash
  latency_ms
  token_usage
  error_code
  error_message
  created_at
```

第一轮不建议新增该表，避免数据库压力和开发面过大。

## 12. Web 展示设计

Dashboard 后续应新增云端推理区：

```text
云端推理总量
云边分歧比例
平均云端延迟
平均 token
平均 reward
最近云端状态
```

事件详情中增加：

```text
云端诊断
  - 云端分类
  - 根因判断
  - 处置建议
  - 解释证据
  - 云边分歧
  - reward 原因
  - token / latency
```

产品原则：

- 不展示原始 prompt；
- 不把完整 raw LLM response 直接扔到页面；
- 面向用户展示“根因、建议、分歧、成本、收益”；
- 面向论文复现的数据留在 stream/log/export 中。

## 13. 开发阶段建议

### 阶段 1：文档与契约冻结

完成：

- 固定 `cloud.req` 输入格式；
- 固定 `cloud.resp` 输出格式；
- 固定上下文日志聚合策略；
- 固定 `cloud_feedback` 写入字段；
- 固定 `cloud_providers` 表结构；
- 固定 `/api/v1/cloud-config` 配置下发接口；
- 固定 reward 公式和默认参数。

### 阶段 2：server cloud provider 控制面

完成：

- `cloud_providers` 表；
- cloud provider CRUD；
- cloud provider activate；
- `/api/v1/cloud-config`；
- 不返回 API key 明文；
- config version / etag。

### 阶段 3：独立 cloud-gateway service

完成：

- 新增独立 Go service；
- 拉取 `/api/v1/cloud-config`；
- 配置本地缓存与热更新；
- Redis consumer group；
- request decode；
- 幂等检查；
- LLM client interface；
- cloud response publish；
- cloud feedback 写 DB。

### 阶段 4：OpenAI-compatible provider

完成：

- DeepSeek/OpenAI-compatible chat completions；
- API key env 读取；
- 超时、限流和错误分类；
- JSON 输出解析与 repair。

### 阶段 5：上下文日志聚合

完成：

- 从 Redis 近窗口读取 inferred/routed event；
- 按 source/equipment/template/time/severity 聚合相关日志；
- 构造 `related_event_summary` 和 `top_related_logs`；
- 按 token 预算裁剪上下文；
- 在 `feedback_json` 中保存本次使用的摘要上下文。

### 阶段 6：reward 与 router feedback 联调

完成：

- reward 计算；
- `cloud_feedback.processed_by_router=false` 写入；
- router feedback processor 消费；
- Dashboard 中 router state 观察 cloud gain 变化。

### 阶段 7：Web 云端诊断展示

完成：

- cloud feedback API；
- Dashboard 云端推理指标；
- 事件详情展示云端根因与建议；
- 展示云边分歧和 reward。

### 阶段 8：可靠性增强

完成：

- pending claim；
- DLQ；
- provider health probe；
- 并发 worker；
- 可选 cloud invocation 审计表。

## 14. 第一轮开发边界

第一轮应完成：

- 新增独立 `cloud-gateway` service；
- server 新增 `cloud_providers` 表和配置 API；
- cloud-gateway 通过 `/api/v1/cloud-config` 拉取云端模型配置；
- 消费 `cotriage.events.cloud.req`；
- 聚合中心事件附近的相关日志；
- 调用 OpenAI-compatible provider；
- 解析结构化 JSON；
- 发布 `cotriage.events.cloud.resp`；
- 写入 `cloud_feedback`；
- 支持幂等；
- 支持失败日志和 DLQ；
- 不把 API key 写死到代码中。

第一轮暂不做：

- provider 管理页面；
- 多 provider 自动切换；
- cloud invocation 审计表；
- 复杂 prompt 版本管理；
- 人工标注校验闭环。

## 15. 验收标准

本地联调成功标准：

1. router-controller 输出一条 `CLOUD` 决策到 `cotriage.events.cloud.req`；
2. cloud-gateway 消费该请求；
3. 云端模型返回结构化诊断；
4. Redis 中出现 `cotriage.events.cloud.resp`；
5. MySQL `cloud_feedback` 新增对应 `event_id`；
6. `processed_by_router=false`；
7. router-controller feedback processor 处理后变为 `true`；
8. router state snapshot 中能看到 template gain 或 feedback 统计更新；
9. 重复注入同一 `event_id` 不产生重复 feedback；
10. provider 超时或 key 缺失时不会导致 server 崩溃。

## 16. 论文价值

cloud-gateway 在论文中承担两个价值：

1. 提供高质量 oracle-like cloud diagnosis，用于修正边缘模型的不确定与误判；
2. 将云端诊断转化为在线 reward，驱动 RAISE 自适应路由。

因此实验中可以报告：

- 云端调用比例；
- 云边分歧率；
- 云端纠错率；
- 每 token 带来的诊断收益；
- reward 驱动前后 RAISE 的 cloud precision；
- 弱网/预算限制下的高风险事件覆盖率；
- 与固定阈值上云、全量上云、随机抽样上云的对比。

这使 cloud-gateway 不只是工程调用层，而是 RAISE 闭环学习的重要组成部分。
