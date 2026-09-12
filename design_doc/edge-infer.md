# edge-infer 设计文档

## 1. 目标与定位

`edge-infer` 是 Cotriage 的边缘实时推理组件。它从 Redis Streams 消费原始日志事件，在边缘集群内完成日志预处理、模板提取、快速分类、不确定性估计和证据提取，并将结构化结果写入下游 Stream，供 `router-controller` 决定 `LOCAL/CLOUD/DEFER`。

核心目标：

- 在弱网或断网环境下仍能完成基本日志分类和紧急告警判断；
- 在200ms内的完成边缘分诊；
- 通过模板缓存和轻量模型减少大模型调用、云端负载与数据上传量；
- 输出可校准的不确定性和结构化证据，为云边动态路由提供依据；
- 记录完整性能指标，支撑比赛中的实时性、准确性和资源效率评测。

非目标：

- 不负责云端级别的复杂根因定位和长链路推理；
- 不负责决定事件最终在本地还是云端处理；
- 不直接操作设备或执行高风险运维动作；
- 不负责跨节点冲突检测和全局一致性仲裁。

## 2. 设计原则

### 2.1 快速路径优先

不能让每条日志都经过生成式大模型。edge-infer 采用四级分层推理：

1. 规则路径：处理证据明确的常见故障；
2. 模板缓存路径：复用近期相同日志模板的结果；
3. 轻量分类模型路径：处理新模板和一般疑难日志；
4. 边缘小 LLM 路径：仅处理分类模型仍不确定的少量事件。

降级路径不属于第五级推理。当任一级依赖不可用或超时时，统一输出 `FALLBACK/UNKNOWN` 并交由路由器处理。

### 2.2 推理与路由解耦

edge-infer 负责回答“这条日志可能是什么、严重程度如何、判断有多可靠”；`router-controller` 负责结合网络、队列、预算和业务策略回答“应该在本地处理还是发送到云端”。

### 2.3 可解释、可复现、可评测

每次推理必须记录处理路径、模型版本、规则版本、置信度、证据和各阶段耗时。相同输入在相同版本下应尽量得到稳定结果。

## 3. 总体处理流程

```text
cotriage.events.raw
        |
        v
输入校验与幂等检查
        |
        v
日志规范化与模板提取
        |
        +----> 强规则命中 ------------------------+
        |                                         |
        +----> 模板缓存命中 ----------------------+--> 结果校验
        |                                         |
        +----> 轻量分类模型 ----------------------+      |
        |         | 低置信度                             |
        |         v                                      |
        +----> 边缘小 LLM -------------------------+      |
        |                                                v
        +----> 超时/异常降级为 UNKNOWN            cotriage.events.inferred
```

推荐时延预算：

| 阶段 | 目标耗时 |
| --- | ---: |
| 输入解析与规范化 | <= 5ms |
| 规则或模板缓存命中 | <= 10ms |
| 轻量分类模型 | <= 100ms |
| 边缘小 LLM | <= 150ms，严格超时 |
| edge-infer 总耗时 P95 | <= 180ms |

上述为研发目标，最终以真实硬件实测结果为准。

## 4. 输入数据契约

### 4.1 Redis Stream

- Stream：`cotriage.events.raw`
- Consumer Group：`edge-infer-g`
- Consumer：每个 Pod 使用唯一名称，例如 `${POD_NAME}`
- Producer：`log-collector`

Stream entry 包含：

```text
event_id: string
payload: JSON string
```

`payload` 示例：

```json
{
  "event_id": "01J...",
  "ts": "2026-06-22T03:00:00Z",
  "equipment": "LinuxServer",
  "source": "host-5-worker",
  "log": {
    "raw": "[error] connect to 10.0.0.2:3306 failed: connection refused"
  }
}
```

兼容要求：`event_id` 同时出现在 Stream 字段和 payload 时，两者必须一致，否则进入 DLQ。

## 5. 输出数据契约

### 5.1 Redis Stream

- Stream：`cotriage.events.inferred`
- Producer：`edge-infer`
- Consumer Group：`router-g`

Stream entry 同样使用：

```text
event_id: string
payload: JSON string
```

`payload` 示例：

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
  "module": "mysql-client",
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

字段约束：

- `edge_pred`：`NORMAL | ABNORMAL | APP_ERROR | NET_ERROR | DISK_ERROR | INFRA_ERROR | SECURITY | UNKNOWN`；
- `severity`：整数 `[0,100]`，表示业务影响程度；
- `confidence`：校准后的预测可信度 `[0,1]`；
- `uncertainty`：预测不确定性 `[0,1]`，MVP 可取 `1-confidence`；
- `evidence`：最多 5 条短证据，不包含完整敏感日志；
- `inference_path`：`RULE | CACHE | MODEL | EDGE_LLM | FALLBACK`；
- `latency_ms`：从开始处理到成功写入输出 Stream 的耗时；
- `model_version` 和 `rule_version`：必须可追溯，未使用时允许为空。

## 6. 分层推理设计

### 6.1 输入校验与预处理

首先校验 `event_id`、时间戳和 `log.raw`。空日志、超长日志或格式错误的事件不得触发模型调用。

预处理包括：

- 限制输入长度，保留日志头部和异常尾部；
- 去除时间戳、PID、线程号等不稳定前缀；
- 对 IP、端口、UUID、路径、十六进制值和数字进行占位符归一化；
- 提取模块名、错误码和关键异常短语；
- 对密码、Token、密钥等敏感信息进行脱敏。

输出 `template_text`，再计算：

```text
template_id = xxhash64(template_text)
```

同一版本的规范化算法必须产生稳定的 `template_id`。

### 6.2 规则路径

规则只覆盖证据明确、误判风险低的场景，例如：

- `out of memory`、`oom killed`；
- `no space left on device`；
- `connection refused`、`network unreachable`；
- 明确的认证失败或攻击特征；
- 健康检查和已知正常噪声。

规则输出分类、严重度区间、证据和 `advice_code`。规则需要版本化并有单元测试。模糊关键词不得直接产生高置信度结论。

规则配置来源：

- MVP 阶段支持本地 `RULES_FILE`；
- 后续由 server 管理端发布 active 规则集；
- edge-infer 通过 `GET /api/v1/edge-config` 拉取规则集；
- 拉取成功后先校验规则语法、枚举、严重度、置信度和优先级；
- 校验通过后原子替换内存规则引擎；
- 校验失败时继续使用上一份可用规则集；
- inferred event 必须记录 `rule_version`，用于追踪命中的规则发布版本。

规则执行顺序：

1. 仅加载 active 规则集；
2. 过滤 disabled 规则；
3. 按 priority 从小到大匹配；
4. 第一条命中即返回结果；
5. 未命中再进入模板路径。

规则匹配目标字段：

- 默认匹配 `template_text`；
- 必要时可匹配 `raw_log`；
- 匹配 `raw_log` 的规则需要更谨慎，避免动态字段导致规则不稳定。

### 6.3 模板缓存路径

缓存 Key：

```text
cotriage:edge-infer:template:{template_id}:{model_version}:{rule_version}
```

缓存内容为最近一次可靠推理结果，默认 TTL 为 5 分钟。只有满足以下条件的结果才能缓存：

- `confidence >= 0.85`；
- 分类不为 `UNKNOWN`；
- 模型和规则版本未变化。

缓存命中时仍要更新 `event_id`、时间戳、来源和耗时，并将 `inference_path` 设为 `CACHE`。

模板路径分为两类：

1. 管理端模板知识库：人工确认、可发布、可审计的稳定模板；
2. Redis 临时模板缓存：运行时为了减少重复推理而写入的短 TTL 缓存。

执行顺序：

1. 优先查 active 模板知识库；
2. 未命中再查 Redis 临时模板缓存；
3. 两者都未命中再进入轻量分类模型路径。

管理端模板知识库：

- 由 server 管理端维护；
- 通过 active 模板集发布；
- edge-infer 通过 `GET /api/v1/edge-config` 拉取；
- 拉取成功后校验 template_id、template_text、edge_pred、severity、confidence、evidence 和 advice；
- 校验通过后原子替换内存模板索引；
- 校验失败时继续使用上一份可用模板集；
- inferred event 需要记录模板集版本，后续可新增 `template_set_version` 字段。

Redis 临时模板缓存属于性能优化，不是可靠配置来源。Redis 缓存读写失败时不得阻断主推理链路；失败信息可写入 evidence 供审计。

### 6.4 轻量分类模型路径

主路径使用编码器分类模型，而不是生成式模型。当前 MVP 先接入 BGL 训练得到的 DistilBERT 二分类模型：

- 输入：`template_text`；
- 输出：`NON_ALERT/ALERT`、`alert_probability`、验证集选择的分类阈值；
- 部署：训练模块将 DistilBERT 导出为 ONNX，`serve_onnx.py` 以 HTTP `/predict` 形式提供本地推理服务；
- edge-infer 接入：配置 `CLASSIFIER_URL` 后，在规则和模板缓存均未命中时调用 HTTP classifier；
- 类别映射：`NON_ALERT -> NORMAL`，`ALERT -> ABNORMAL`；
- 置信度：`ALERT` 使用 `alert_probability`，`NON_ALERT` 使用 `1-alert_probability`；
- 不确定性：MVP 使用 `1-confidence`。

当前二分类模型只负责正常/异常初筛，不直接承担细粒度根因诊断。细粒度类别和建议由规则、模板、边缘小 LLM 或云端模型继续完成。

后续版本可将训练目标升级为七分类 logits：

- 输入：`template_text` 和可选模块特征；
- 输出：七分类 logits；
- 部署：ONNX Runtime，优先使用 INT8 量化；
- 置信度：softmax 后经过温度缩放校准；
- 不确定性：可使用预测熵或校准后的 top-k 分布。

轻量分类模型具有时延稳定、内存占用可控、输出格式可靠的优点，适合作为 200ms 约束下的主要推理路径。

### 6.5 边缘小 LLM 路径

小 LLM 是补充路径，不是默认路径。当前 MVP 通过 Linux 服务器上的 llama.cpp `llama-server` 提供 OpenAI 兼容接口：

```text
LLM_URL=http://100.85.92.31:18081/v1/chat/completions
```

仅在以下条件下调用：

- 分类模型输出 `ABNORMAL`；
- 新模板且规则、模板缓存无法判断；
- 需要生成简短证据或建议；
- 当前队列、CPU、内存和超时预算允许。

MVP 阶段通过环境变量配置 `LLM_URL` 和 `LLM_TIMEOUT`。后续可改为通过 server 发布的 `GET /api/v1/edge-config` 获取配置。edge-infer 使用 ETag 轮询，配置变化后先健康检查，再原子切换；新配置不可用时继续使用最近一次可用配置。

小 LLM 必须设置严格的请求超时、最大输出 Token、确定性采样参数和 JSON Schema 校验。模型自报的 `confidence` 不能直接视为可靠概率；MVP 可将其作为辅助特征，最终置信度由离线验证集校准。

建议输出：

```json
{
  "edge_pred": "NET_ERROR",
  "severity": 78,
  "confidence": 0.72,
  "evidence": ["connection refused"],
  "advice_code": "CHECK_NETWORK_AND_SERVICE",
  "advice": "检查目标服务状态、端口监听和网络策略"
}
```

类别映射：

```text
0=NORMAL
1=ABNORMAL
2=APP_ERROR
3=NET_ERROR
4=DISK_ERROR
5=INFRA_ERROR
6=SECURITY
7=UNKNOWN
```

### 6.6 降级路径

发生以下情况时输出 `UNKNOWN`，不得阻塞整个消费链路：

- 当前没有已发布且健康的模型；
- 模型请求超时或返回非预期格式；
- 输入无法解析；
- 资源保护机制拒绝模型调用。

降级结果使用 `inference_path=FALLBACK`、较高 `uncertainty`，并在内部错误字段或指标中记录原因。业务错误与系统错误必须区分，避免把模型暂时不可用误认为日志正常。

## 7. 严重度、置信度与证据

### 7.1 严重度

`severity` 表示潜在业务影响，不等于模型置信度。建议区间：

| 区间 | 含义 |
| --- | --- |
| 0-19 | 正常或可忽略噪声 |
| 20-39 | 轻微异常 |
| 40-69 | 需要关注 |
| 70-89 | 高风险，需要及时处理 |
| 90-100 | 宕机、数据丢失、安全入侵等紧急事件 |

高严重度必须由明确证据或经过验证的模型结论支持。

### 7.2 置信度校准

分类模型应使用独立验证集进行温度缩放，并报告 ECE 或可靠性曲线。路由器使用的是校准后的 `confidence`，而不是未经处理的 logits 或 LLM 自评值。

### 7.3 证据输出

`evidence` 用于云端复核和人工解释，应来自日志中的关键短语、错误码或结构化特征。禁止编造原日志中不存在的事实。

## 8. Redis 消费可靠性

edge-infer 启动不应强依赖 Redis 已经可用。进程启动后先完成本地规则、模板文件、分类器和 LLM 客户端初始化；Redis 暂时不可用时，worker 按 `REDIS_RETRY_DELAY` 持续等待 `PING` 和 Consumer Group 创建成功，而不是直接退出。

处理顺序：

1. 使用 `XREADGROUP` 获取消息；
2. 校验消息并检查 `event_id` 是否已处理；
3. 完成推理；
4. 使用 `XADD` 写入 `cotriage.events.inferred`；
5. 输出成功后才对原消息执行 `XACK`。

幂等要求：

- `event_id` 是全链路幂等键；
- 重复消费不得产生语义不同的结果；
- 下游存储对 `event_id` 建立唯一约束；
- 后续可用 Lua 或数据库状态表进一步保证“输出成功与处理状态”一致。

失败处理：

- 短暂 Redis 或模型错误使用退避重试；
- `XREADGROUP` 读取失败时记录日志并继续重试，避免 Redis 连接短暂抖动导致进程退出；
- 定期通过 `XAUTOCLAIM` 接管超时 Pending 消息；
- 超过最大尝试次数后写入 `cotriage.events.dlq`；
- 解析错误、模型错误、Redis 错误分别计数。

容量保护：

- raw、inferred 和 DLQ Stream 都必须有最大保留长度；
- producer 写入时使用 `XADD MAXLEN ~` 做第一层裁剪；
- edge-infer 每 `REDIS_CLEANUP_EVERY` 周期性执行 `XTRIM MAXLEN ~`，作为统一兜底；
- 默认保留 `RAW_STREAM_MAXLEN=100000`、`INFERRED_STREAM_MAXLEN=100000`、`DLQ_STREAM_MAXLEN=10000`。

模板缓存属于性能优化，不应成为主链路强依赖。Redis 模板缓存读取失败时跳过缓存路径继续推理；写入失败时在 evidence 中记录错误，但不阻断 inferred event 发布。

## 9. 配置拉取与热更新

四级分层路径最终都应由 server 管理端统一配置：

1. 规则路径：active 规则集；
2. 模板路径：active 模板知识库；
3. 轻量分类模型路径：classifier endpoint、timeout、model_version；
4. 边缘小 LLM 路径：active provider。

edge-infer 使用轮询方式拉取：

```text
GET /api/v1/edge-config
If-None-Match: <last-etag>
```

运行时配置项：

- `EDGE_CONFIG_URL`：server 统一配置发布物地址，例如 `http://127.0.0.1:8080/api/v1/edge-config`；
- `EDGE_CONFIG_REFRESH_INTERVAL`：配置轮询间隔，默认 `30s`；
- `EDGE_CONFIG_TIMEOUT`：单次配置拉取超时时间，默认 `3s`。

server 返回：

- `304 Not Modified`：配置未变化，继续使用本地内存配置；
- `200 OK`：返回新的统一配置发布物，包括 rules、templates、classifier 和 active_provider；
- 请求失败：继续使用上一份可用配置，并记录日志。

当前实现：

- `rules` 更新后会原子替换内存规则引擎；
- `templates` 更新后会原子替换内存模板索引；
- `classifier` 更新后会重建 HTTP classifier 客户端，接管 `MODEL` 路径；
- `active_provider` 更新后会重建 OpenAI 兼容 LLM 客户端，接管 `EDGE_LLM` 路径；
- 如果 server 未配置 active classifier 或 active provider，对应路径会被关闭，日志继续走后续降级逻辑。

热更新要求：

- 新配置必须完整校验后才能替换；
- 替换必须是原子操作，避免半更新状态；
- 正在处理的单条日志使用进入处理时看到的配置快照；
- 配置切换后新日志使用新快照；
- 输出事件记录 `rule_version`、`model_version`，后续可扩展 `template_set_version` 和 `edge_config_version`；
- 本地文件配置可作为 bootstrap fallback，当 server 不可用时仍可启动基础能力。

校验失败处理：

- 保留上一份可用配置；
- 写日志并上报配置错误指标；
- 不影响 Redis 消费主循环；
- Web 管理端应能在发布校验阶段尽早发现大部分错误。

## 10. 模板统计与持久化

主路径不能对每条日志同步执行 MySQL 写入。模板结果优先保存在 Redis，统计信息通过内存缓冲区异步批量写入 MySQL。

建议表 `edge_log_templates`：

```text
template_id           PK
template_text
first_seen_at
last_seen_at
seen_count_total
last_infer_at
last_edge_pred
last_severity
last_confidence
last_uncertainty
last_advice
model_version
rule_version
cache_expires_at
```

MySQL 暂时不可用时不得阻塞实时推理，可保留有限内存缓冲并记录丢弃计数。

## 11. 资源保护与部署

edge-infer 以 Kubernetes Deployment 部署，可通过 Consumer Group 横向扩展。轻量模型可以：

- 直接嵌入 edge-infer 进程；或
- 作为同 Pod sidecar，通过 localhost 调用。

边缘小 LLM 建议作为独立 Deployment，以便单独分配资源和替换模型。

资源保护机制：

- 限制最大并发推理数；
- 设置输入队列上限和模型超时；
- 队列积压严重时优先走规则、缓存或降级路径；
- 定期采集 CPU、内存和队列长度；
- 通过 readiness probe 阻止未加载模型的实例接收流量；
- Pod 退出时停止读取新消息，并等待在途任务完成或超时。

比赛目标要求单次边缘推理内存占用不超过 1.5GB，因此模型、运行时和进程峰值 RSS 都必须纳入实测。

## 12. 可观测性与比赛指标

### 12.1 运行指标

- 消费速率和成功率；
- Stream lag 和 Pending 数量；
- 各推理路径命中率；
- P50、P95、P99 推理耗时；
- 模型超时率、格式错误率和降级率；
- CPU、峰值内存和模型加载时间；
- 输入字节数、输出字节数和预计减少的上云字节数。

### 12.2 效果指标

- Accuracy、Precision、Recall、Macro-F1；
- 各故障类别的混淆矩阵；
- 边缘模型相对云端全量模型的能力保持率；
- 置信度校准误差；
- 弱网期间本地基本业务功能保持率；
- 规则、缓存、轻量模型和小 LLM 的消融实验结果。

至少比较以下基线：

1. 全部事件发送云端；
2. 全部事件仅在边缘处理；
3. 固定置信度阈值路由；
4. 基于网络、负载和不确定性的动态路由。

## 13. 配置项

建议环境变量：

```text
REDIS_ADDR=redis:6379
RAW_STREAM=cotriage.events.raw
INFERRED_STREAM=cotriage.events.inferred
DLQ_STREAM=cotriage.events.dlq
CONSUMER_GROUP=edge-infer-g
CONSUMER_NAME=${POD_NAME}
RULES_FILE=
TEMPLATES_FILE=
SERVER_ADDR=http://server:8080
CONFIG_REFRESH_INTERVAL=10s
TEMPLATE_CACHE_TTL=5m
BLOCK_TIMEOUT=2s
MODEL_TIMEOUT=150ms
MAX_INFER_CONCURRENCY=4
MAX_ATTEMPTS=3
CLAIM_IDLE_TIME=30s
LOG_MAX_BYTES=8192
```

阈值和规则应由版本化配置管理，避免散落在代码中。

## 14. MVP 开发顺序

### 阶段一：打通可靠链路

- [x] 消费 `cotriage.events.raw`；
- [x] 完成输入校验、日志规范化和模板 ID；
- [x] 实现基础规则、外部规则文件与模板缓存；
- [x] 输出统一的 inferred payload；
- [x] 实现 ACK、重试、Pending 接管和 DLQ；
- [x] 添加时延与路径埋点；
- [ ] 与真实 Redis 和 log-collector 完成端到端联调；
- [ ] 将规则和模板来源从本地 JSON 切换为 server 发布的版本化快照。

### 阶段二：接入轻量分类模型

- 训练七分类编码器模型；
- 导出 ONNX 并进行 INT8 量化；
- 加入置信度校准和资源监控；
- 完成准确率、F1、时延和内存基准测试。

### 阶段三：接入边缘小 LLM

- 对接 `/api/v1/edge-config`；
- 实现健康检查、配置热更新和原子切换；
- 仅让低置信度样本进入小 LLM；
- 校验 JSON 输出并实现严格超时降级。

### 阶段四：支撑云边协同实验

- 与 router-controller 联调动态路由；
- 记录网络波动、队列积压和上云比例；
- 完成纯云、纯边、静态路由和动态路由对照实验；
- 验证 200ms 时延、1.5GB 内存和弱网业务保持率目标。

## 15. 当前实现说明

当前代码位于 `edge-infer/`，已完成阶段一的本地可测试 MVP：

- 使用 Redis Consumer Group 消费原始事件；
- 支持内置规则以及 `RULES_FILE` 指定的 JSON 规则；
- 支持 `TEMPLATES_FILE` 指定的人工模板知识库；
- 可靠规则结果自动写入带 TTL 的 Redis 模板缓存；
- 未命中规则和模板时输出 `FALLBACK/UNKNOWN`，暂不伪造模型判断；
- 输出成功后才 ACK，失败消息保留在 Pending，达到最大次数后进入 DLQ；
- 使用 `event_id` 短期幂等键减少重复输出；
- 已提供规范化与规则引擎单元测试、Dockerfile 和本地运行说明。
- 已通过隔离 Redis Stream 完成单条规则事件的 `raw -> inferred` 联调。

当前尚未实现：

- server 端规则、模板和统一配置快照管理；
- edge-infer 通过 ETag 拉取配置并热更新；
- 轻量分类模型推理；
- 边缘小 LLM 推理；
- Prometheus 指标接口和完整端到端性能测试。

阶段一使用本地 JSON 是为了先稳定数据契约和实时链路。server 后续应作为配置控制面，edge-infer 拉取发布后的不可变快照并在本地执行，server 不进入逐条日志的实时处理路径。
