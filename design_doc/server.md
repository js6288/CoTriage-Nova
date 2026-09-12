server设计文档

1. 设备管理
在cotriage系统设计中，端设备会以worker节点的形式加入到边缘集群中。cotriage本身不负责端设备的添加与移除，只支持设备信息的查看。

设备信息：
- node_name: 节点名
- equipment: 设备名称，从节点的cotriage.io/equipment label中获取
- site: 车间名称，标识设备属于哪个车间，从节点的cotriage.io/site label中获取
- status: 节点状态，标识该设备是否在运行
- collector_status: 该 node 上 log-collector Pod 是否 Running
- last_event_time：该节点最近一次事件时间
- ingest_rate_1m：最近一分钟事件上报速率

API设计：
获取设备列表：GET /api/v1/devices
用途：查询边缘集群中所有 worker 节点（设备）的基础信息与接入健康度。

Query Parameters（可选）
site：按车间过滤（精确匹配），如 ?site=workshop-a
equipment：按设备名称过滤（精确匹配）
status：Ready | NotReady | Unknown
limit：默认 200，最大 1000（MVP 可先不实现分页，只保留字段）
offset：默认 0（同上）

Response示例：
{
  "items": [
    {
      "node_name": "kind-worker",
      "equipment": "press-01",
      "site": "workshop-a",
      "status": "Ready",
      "collector_status": "Running",
      "last_event_time": "2026-05-27T08:12:31Z",
      "ingest_rate_1m": 42
    },
    {
      "node_name": "kind-worker2",
      "equipment": "press-02",
      "site": "workshop-a",
      "status": "NotReady",
      "collector_status": "Unknown",
      "last_event_time": null,
      "ingest_rate_1m": 0
    }
  ],
  "total": 2,
  "generated_at": "2026-05-27T08:12:35Z"
}
字段说明：
node_name：Kubernetes Node.name
equipment：Node label cotriage.io/equipment（不存在可返回空字符串或 unknown）
site：Node label cotriage.io/site（不存在可返回空字符串或 unknown）
status：Node Condition Ready 的归一化状态（Ready/NotReady/Unknown）
collector_status：该 node 上 log-collector Pod 状态（Running/Pending/CrashLoopBackOff/NotFound/Unknown 等）
last_event_time：该 node 最近一次 event 的时间戳（从redis streams中获取）
ingest_rate_1m：过去 60 秒该 node 产生的 event 数量（从redis streams中获取）

2. 边缘模型管理
在cotriage系统设计中，支持管理员对边缘侧大模型进行接入，修改，查看，删除，发布等功能。
边缘侧大模型的配置存储在数据库表中。

当前模型管理拆为两类：
- 轻量分类模型：对应 edge-infer 的 `MODEL` 路径，管理 MiniLM/DistilBERT ONNX HTTP classifier；
- 边缘小 LLM：对应 edge-infer 的 `EDGE_LLM` 路径，复用 `edge_provider` 管理 OpenAI 兼容模型服务。

classifier_configs 表：
- id (PK)
- name：配置名称
- endpoint：HTTP classifier `/predict` 地址
- model：模型版本或部署包名称，例如 `bgl-minilm`
- timeout_ms：调用超时
- threshold：告警阈值，主要用于展示和审计；实际阈值也会由 classifier 服务响应返回
- status：draft | published | disabled | deleted
- is_active：当前生效的分类模型配置，全局最多一个
- config_version：启用版本号
- etag：由配置内容生成
- created_at, updated_at, deleted_at

轻量分类模型 API：
```text
GET    /api/v1/edge-classifiers
POST   /api/v1/edge-classifiers
PATCH  /api/v1/edge-classifiers/{id}
DELETE /api/v1/edge-classifiers/{id}
POST   /api/v1/edge-classifiers/{id}/activate
```

启用语义：
- 将目标分类模型置为 `published + is_active=true`；
- 将旧 active 分类模型置为 `is_active=false`；
- 生成新的 `config_version` 和 `etag`；
- 下一次 `GET /api/v1/edge-config` 会携带 active classifier，edge-infer 拉取后热更新 `MODEL` 路径。

edge_provider表：
id (PK)
name (unique，便于人读)
provider (enum)
endpoint (string)
model (string)
timeout_ms (int)
max_tokens (int)
temperature (float)
top_p (float)
seed (int, nullable)
healthcheck_path (string, nullable) 例如 /v1/models or /health
response_path (string) 例如 choices[0].message.content
status 
is_active is_active（运行时激活位）true：当前系统实际在用的那一个 provider（全局最多一个）false：其它所有 provider
config_version (bigint，自增，用于 edge-config 版本)
etag (string) 可由 config_version + hash 生成
created_at, updated_at
created_by, updated_by
deleted_at（软删）
last_publish_at

API设计：
Base path：/api/v1
provider 枚举：llama_cpp | openai_compat | vllm | custom_http
模型状态：
draft：草稿/未发布。可以编辑；不能被 edge-infer 使用。
published：已发布、可被启用（可作为候选版本）。通常不随便编辑（编辑要生成新 draft 或新版本）。
disabled：已发布但被禁用（例如安全/故障原因临时下线）；不能被启用。
deleted：软删除；不可用，仅用于审计/历史。

唯一性
任意时刻：sum(is_active=true) <= 1
激活必须是发布且未禁用
is_active=true ⇒ status == "published"
禁用会导致无法激活
status in ("disabled","deleted","draft") ⇒ is_active=false
发布不等于启用
status="published" 可以 is_active=false
（表示“已经发布、可回滚备用、但当前没在用”）

边缘模型接入：
POST /api/v1/edge-providers
{
  "name": "edge-qwen-1.8b-local",
  "provider": "llama_cpp",
  "endpoint": "http://127.0.0.1:8080/v1/chat/completions",
  "model": "default",
  "timeout_ms": 500,
  "max_tokens": 80,
  "temperature": 0,
  "top_p": 1,
  "seed": 42,
  "healthcheck_path": "/v1/models",
  "response_path": "choices[0].message.content"
}

边缘模型修改：
PARCH /api/v1/edge-providers/{id}（部分更新）
{
  "timeout_ms": 800,
  "max_tokens": 100
}

查看边缘模型：
详情页：
GET /api/v1/edge-providers/{id}
列表页：
GET /api/v1/edge-providers
支持查询参数：
status=draft|published|disabled
active=true|false
q=<name contains>
page page_size
响应 200
JSON
{
  "items": [ { ... }, { ... } ],
  "page": 1,
  "page_size": 20,
  "total": 57
}


删除边缘模型：
DELETE /api/v1/edge-providers/{id}
如果是 active：返回 409，要求先切换 active 到别的 provider 或 disable
默认做软删：status=deleted, deleted_at=...

发布边缘模型：
server 提供一个只读 endpoint：
GET /edge-config 返回当前生效配置 + ETag/version
edge-infer 每隔 N 秒拉一次：
带 If-None-Match: <etag>，没变就 304
变了就下载、校验、切换
同一时刻内，只能有一个边缘模型处于启用状态。

POST /api/v1/edge-providers/{id}/activate
语义：
将该 provider 标记为 published 且 is_active=true
并将当前 active 的 provider（如果存在）置为 is_active=false（可选保留 published 状态）
config_version 全局自增（每次 activate 增加一次）
生成新 etag

一致性要求：在 edge_provider 表上用事务：先 UPDATE edge_provider SET is_active=false WHERE is_active=true; 再 UPDATE edge_provider SET is_active=true,... WHERE id=?;

edge-infer 拉取生效配置（只读）
GET /api/v1/edge-config

响应头：
ETag: W/"104-9ab122"
Cache-Control: no-store（防止中间缓存错乱）

请求头（edge-infer）：
If-None-Match: W/"104-9ab122"

200 响应体（配置发布物）
{
  "config_version": 104,
  "etag": "W/\"104-9ab122\"",
  "active_provider": {
    "id": 12,
    "provider": "llama_cpp",
    "endpoint": "http://127.0.0.1:8080/v1/chat/completions",
    "model": "default",
    "timeout_ms": 500,
    "max_tokens": 80,
    "temperature": 0,
    "top_p": 1,
    "seed": 42,
    "healthcheck_path": "/v1/models",
    "response_path": "choices[0].message.content"
  }
}

304 Not Modified
无 body，edge-infer 继续用本地配置。

3. 实时大盘与异常态势分析

server 负责为 Web 实时大盘提供 Redis Streams 聚合接口。前端不直接连接 Redis，所有统计快照、事件列表和 SSE 实时流均通过 server 暴露。

数据来源：
- raw Stream：`cotriage.events.raw`
- inferred Stream：`cotriage.events.inferred`
- DLQ Stream：`cotriage.events.dlq`
- edge-infer Consumer Group：默认 `edge-infer-g`

API 设计：

1. 大盘汇总

GET /api/v1/dashboard/summary

用途：返回实时大盘顶部指标卡、分类分布和推理路径分布所需数据。

主要字段：
- raw_total：raw Stream 当前长度
- inferred_total：inferred Stream 当前长度
- pending：edge-infer Consumer Group 中已读取但尚未 ACK 的消息数
- dlq_total：DLQ Stream 当前长度
- ingest_rate_1m：raw Stream 最近一分钟新增消息数
- infer_rate_1m：inferred Stream 最近一分钟新增消息数
- p95_latency_ms：最近样本的 P95 推理时延
- abnormal_rate：最近样本中非 `NORMAL` 的比例
- fallback_rate：最近样本中 `FALLBACK` 路径的比例
- classification：按 `edge_pred` 聚合的分类分布
- paths：按 `inference_path` 聚合的推理路径分布

2. 处理趋势

GET /api/v1/dashboard/timeseries

用途：返回最近 10 分钟每分钟完成推理数量和异常事件数量。

Response 示例：
{
  "items": [
    {
      "time": "2026-06-24T10:01:00Z",
      "inferred": 128,
      "abnormal": 12
    }
  ]
}

3. 最近事件列表

GET /api/v1/events?limit=50

用途：返回最近 inferred event，用于实时事件表初始化和断线兜底。

限制：
- limit 默认 50
- limit 最大 200
- 历史消息没有 `raw_log` 时，前端回退展示 `template_text`

4. SSE 实时事件流

GET /api/v1/events/stream

用途：向前端推送连接建立后的新 inferred event。

协议：
- Content-Type：`text/event-stream`
- event：`inferred`
- 每 15 秒无新消息时发送 keepalive
- 每个浏览器连接对应一个 Redis 阻塞读取，不在 server 内维护全量事件缓存

5. 异常态势分析

GET /api/v1/dashboard/abnormal-analysis

用途：返回实时大盘中的“异常态势分析”数据，包括风险等级、异常聚合 Top 和优先处理队列。

统计窗口：
- 只统计最近 10 分钟内的非 `NORMAL` inferred event
- 超过 10 分钟的异常不再进入风险等级、异常聚合 Top 和优先处理队列
- 最近 1 分钟和最近 5 分钟异常数作为短时风险指标
- 增长倍数使用“最近 5 分钟异常数 / 5-10 分钟前异常数”

Response 示例：
{
  "risk_level": "MEDIUM",
  "abnormal_1m": 12,
  "abnormal_5m": 36,
  "growth_ratio": 1.8,
  "top_types": {
    "ABNORMAL": 28,
    "INFRA_ERROR": 8
  },
  "top_clusters": [
    {
      "cluster_id": "tpl_xxx:ABNORMAL",
      "edge_pred": "ABNORMAL",
      "template_id": "tpl_xxx",
      "template_text": "KERNEL FATAL rts panic stopping execution",
      "sample_raw_log": "KERNEL FATAL rts panic! - stopping execution",
      "count": 18,
      "affected_equipments": 3,
      "equipments": ["press-01", "press-02", "press-03"],
      "max_severity": 80,
      "avg_confidence": 0.91,
      "first_seen": "2026-06-24T10:00:10Z",
      "last_seen": "2026-06-24T10:04:12Z",
      "advice": "检查节点系统日志和相关服务状态",
      "priority_score": 118.2
    }
  ],
  "priority_queue": [
    {
      "cluster_id": "tpl_xxx:ABNORMAL",
      "edge_pred": "ABNORMAL",
      "count": 18,
      "affected_equipments": 3,
      "max_severity": 80,
      "priority_score": 118.2
    }
  ],
  "generated_at": "2026-06-24T10:05:00Z"
}

风险等级判定：

HIGH 高风险，满足任一条件：
- 最近 1 分钟异常数 `abnormal_1m >= 50`
- 5 分钟增长倍数 `growth_ratio >= 3`
- 最近 10 分钟内存在最高严重度 `max_severity >= 90` 的异常簇

MEDIUM 中风险，未触发 HIGH 且满足任一条件：
- 最近 1 分钟异常数 `abnormal_1m >= 10`
- 最近 5 分钟异常数 `abnormal_5m >= 30`
- 5 分钟增长倍数 `growth_ratio >= 1.5`
- 最近 10 分钟内存在最高严重度 `max_severity >= 70` 的异常簇

LOW 低风险：
- 未触发 HIGH 或 MEDIUM 条件

优先处理队列：

异常簇按 `priority_score` 排序。当前分数是经验公式：

```text
priority_score =
  max_severity * 0.5
  + count * 2
  + affected_equipments * 8
  + avg_confidence * 20
```

该公式含义：
- 严重度越高越优先；
- 出现次数越多越优先；
- 影响设备越多越优先；
- 模型置信度越高越优先。

当前风险阈值和优先级公式属于 MVP 经验规则，适合演示和第一版运行观察。后续应将这些参数配置化，支持通过管理端调整不同场景下的风险阈值。

4. 规则管理

规则管理用于把 edge-infer 的 `RULE` 路径从本地 JSON 文件升级为 server 端可配置、可发布、可审计的运行时配置。规则只处理证据明确、误判风险低的常见故障或正常噪声，不承担模糊诊断。

设计目标：
- 支持用户在管理端新增、编辑、禁用、删除规则；
- 支持草稿、发布、启用和回滚；
- 支持规则优先级排序；
- 支持规则命中后的分类、严重度、置信度、证据和建议输出；
- edge-infer 通过只读配置接口拉取当前启用规则集；
- 所有发布物带版本号和 ETag，便于 edge-infer 增量拉取和审计。

规则状态：
- draft：草稿，可编辑，不会被 edge-infer 使用；
- published：已发布，可被规则集引用；
- disabled：已发布但禁用，不参与推理；
- deleted：软删除，仅保留审计记录。

规则匹配类型：
- contains：日志模板或原始日志包含指定字符串；
- regex：正则表达式匹配；
- all_keywords：全部关键词命中；
- any_keywords：任意关键词命中；
- future：后续可扩展为结构化字段匹配，如设备、站点、模块、日志级别。

建议规则只匹配 `template_text`，必要时可选择匹配 `raw_log`。匹配 `raw_log` 时必须注意敏感信息和动态字段，避免规则不稳定。

edge_rule 表：
- id (PK)
- name：规则名称，便于人读
- description：规则说明
- match_type：contains | regex | all_keywords | any_keywords
- pattern：主匹配表达式，contains/regex 使用
- keywords_json：关键词数组，all_keywords/any_keywords 使用
- target_field：template_text | raw_log，默认 template_text
- edge_pred：NORMAL | ABNORMAL | APP_ERROR | NET_ERROR | DISK_ERROR | INFRA_ERROR | SECURITY | UNKNOWN
- severity：0-100
- confidence：0-1
- evidence_json：命中后写入 inferred event 的证据模板，最多 5 条
- advice_code：建议编码
- advice：中文处理建议
- priority：优先级，数值越大越先匹配
- enabled：是否启用
- status：draft | published | disabled | deleted
- rule_version：规则自身版本号
- created_at, updated_at
- created_by, updated_by
- deleted_at

edge_rule_set 表：
- id (PK)
- name：规则集名称
- description：规则集说明
- status：draft | published | active | archived
- config_version：发布版本号，全局自增
- etag：由 config_version + 内容 hash 生成
- created_at, updated_at
- published_at, activated_at
- created_by, updated_by

edge_rule_set_item 表：
- id (PK)
- rule_set_id
- rule_id
- priority_snapshot：发布时的优先级快照
- rule_snapshot_json：发布时规则内容快照

说明：
- 运行时以 rule_set 发布快照为准，而不是直接读取可编辑规则表；
- 用户编辑已发布规则时，应生成新的 draft 或新版本，避免正在运行的规则集被隐式修改；
- 任意时刻最多一个 active 规则集。

内置规则：
- server 启动并完成表结构迁移后，会幂等预置一批常见规则；
- 内置规则覆盖 OOM/内存不足、磁盘空间不足、磁盘 I/O、网络连接失败、DNS 解析失败、请求超时、认证鉴权失败、证书/TLS 异常、Kubernetes 镜像拉取失败、CrashLoopBackOff、调度失败、MySQL 连接/权限异常、Redis 连接异常、HTTP 5xx、Hadoop/YARN ResourceManager 异常和健康检查正常噪声；
- 内置规则默认状态为 `published` 且 `enabled=true`；
- 如果系统尚不存在 active 规则集，server 会自动创建并启用“内置常用规则集”；
- 如果已经存在 active 规则集，server 只补齐缺失的内置规则，不覆盖用户当前启用的规则集；
- seed 逻辑以规则名称做幂等判断，用户修改或禁用已有内置规则时不会被启动流程强制覆盖。

API 设计：

规则 CRUD：
```text
GET    /api/v1/edge-rules
POST   /api/v1/edge-rules
GET    /api/v1/edge-rules/{id}
PATCH  /api/v1/edge-rules/{id}
DELETE /api/v1/edge-rules/{id}
```

查询参数：
- status=draft|published|disabled
- enabled=true|false
- edge_pred=NET_ERROR
- q=<name or pattern contains>
- page, page_size

创建规则示例：
```json
{
  "name": "磁盘空间不足",
  "description": "明确的 no space left on device 日志",
  "match_type": "contains",
  "target_field": "template_text",
  "pattern": "no space left on device",
  "edge_pred": "DISK_ERROR",
  "severity": 85,
  "confidence": 0.96,
  "evidence": ["no space left on device"],
  "advice_code": "CHECK_DISK_SPACE",
  "advice": "检查磁盘使用率、日志目录和临时文件清理策略",
  "priority": 100,
  "enabled": true
}
```

规则校验：
- regex 必须能成功编译；
- edge_pred 必须在合法枚举内；
- severity 必须在 0-100；
- confidence 必须在 0-1；
- evidence 最多 5 条；
- UNKNOWN 规则不允许 confidence 大于 0.8；
- NORMAL 规则必须谨慎使用，建议只用于明确健康检查或噪声日志。

规则测试：
```text
POST /api/v1/edge-rules/test
```

请求：
```json
{
  "rule": { "...": "待测试规则，可选" },
  "rule_id": 12,
  "log": "KERNEL FATAL rts panic! - stopping execution",
  "template_text": "kernel fatal rts panic stopping execution"
}
```

响应：
```json
{
  "matched": true,
  "edge_pred": "INFRA_ERROR",
  "severity": 90,
  "confidence": 0.95,
  "evidence": ["fatal", "panic"],
  "advice_code": "CHECK_KERNEL_AND_NODE",
  "advice": "检查节点内核日志、硬件状态和相关服务"
}
```

规则集发布：
```text
POST /api/v1/edge-rule-sets
POST /api/v1/edge-rule-sets/{id}/publish
POST /api/v1/edge-rule-sets/{id}/activate
GET  /api/v1/edge-rule-sets
GET  /api/v1/edge-rule-sets/{id}
```

activate 语义：
- 将目标规则集置为 active；
- 将旧 active 规则集置为 archived 或 published；
- 全局 config_version 自增；
- 生成新 ETag；
- 写入发布审计记录。

5. 模板知识库管理

模板知识库用于把 edge-infer 的 `CACHE` 路径从“运行时 Redis 临时缓存”升级为可人工维护的稳定模板结果库。它适合沉淀已经确认过的常见日志模板和处理建议，使相同模板后续无需再经过分类模型或小 LLM。

模板来源：
- 人工新增；
- 从大盘异常聚合 Top 中一键沉淀；
- 从 inferred event 详情中一键加入模板库；
- 后续可从离线标注数据集批量导入。

模板状态：
- draft：草稿，可编辑，不参与推理；
- published：已发布，可被模板集引用；
- disabled：禁用，不参与推理；
- deleted：软删除，仅保留审计记录。

edge_template 表：
- id (PK)
- template_id：由 normalize 后的 template_text 计算得到，建议与 edge-infer 保持同一算法
- template_text：规范化日志模板
- raw_example：样例原始日志
- source：manual | inferred_event | abnormal_cluster | import
- edge_pred：NORMAL | ABNORMAL | APP_ERROR | NET_ERROR | DISK_ERROR | INFRA_ERROR | SECURITY | UNKNOWN
- severity：0-100
- confidence：0-1
- evidence_json：证据数组
- advice_code：建议编码
- advice：中文处理建议
- model_version：模板结论来源模型版本，可为空
- rule_version：模板结论来源规则版本，可为空
- hit_count：模板命中次数，可异步统计
- last_hit_at：最近命中时间
- status：draft | published | disabled | deleted
- created_at, updated_at
- created_by, updated_by
- deleted_at

唯一性建议：
- 对 `template_id + status not deleted` 建唯一约束，避免同一模板重复维护；
- 如果后续支持不同规则/模型版本并存，可扩展为 `template_id + knowledge_base_version`。

edge_template_set 表：
- id (PK)
- name：模板集名称
- description：模板集说明
- status：draft | published | active | archived
- config_version：发布版本号，全局自增
- etag：由 config_version + 内容 hash 生成
- created_at, updated_at
- published_at, activated_at
- created_by, updated_by

edge_template_set_item 表：
- id (PK)
- template_set_id
- template_id
- template_snapshot_json：发布时模板内容快照

说明：
- 运行时以 template_set 发布快照为准；
- 用户编辑已发布模板时，应生成新版本或要求重新发布模板集；
- 任意时刻最多一个 active 模板集。

API 设计：

模板 CRUD：
```text
GET    /api/v1/edge-templates
POST   /api/v1/edge-templates
GET    /api/v1/edge-templates/{id}
PATCH  /api/v1/edge-templates/{id}
DELETE /api/v1/edge-templates/{id}
```

查询参数：
- status=draft|published|disabled
- edge_pred=DISK_ERROR
- template_id=<id>
- q=<template_text contains>
- source=manual|inferred_event|abnormal_cluster|import
- page, page_size

创建模板示例：
```json
{
  "template_text": "error in contacting rm",
  "raw_example": "2015-10-18 18:06:09,966 ERROR ... ERROR IN CONTACTING RM.",
  "edge_pred": "INFRA_ERROR",
  "severity": 75,
  "confidence": 0.9,
  "evidence": ["ERROR IN CONTACTING RM"],
  "advice_code": "CHECK_RESOURCE_MANAGER",
  "advice": "检查 ResourceManager 状态、网络连通性和相关服务日志",
  "source": "manual"
}
```

模板校验：
- template_text 不能为空；
- template_id 由 server 端按统一算法生成，不允许前端随意填写；
- edge_pred、severity、confidence 必须满足合法范围；
- confidence 建议不低于 0.85，否则不应发布为稳定模板；
- evidence 最多 5 条；
- advice_code 建议使用枚举或统一命名规范。

模板测试：
```text
POST /api/v1/edge-templates/test
```

用途：输入一条 raw log，server 使用与 edge-infer 对齐的 normalize 算法生成 template_text/template_id，并检查当前模板库是否可命中。

响应：
```json
{
  "template_id": "a81c70b9e4af0c21",
  "template_text": "error in contacting rm",
  "matched": true,
  "template": {
    "edge_pred": "INFRA_ERROR",
    "severity": 75,
    "confidence": 0.9,
    "advice_code": "CHECK_RESOURCE_MANAGER"
  }
}
```

从事件沉淀模板：
```text
POST /api/v1/edge-templates/from-event
POST /api/v1/edge-templates/from-cluster
```

用途：
- `from-event` 从单条 inferred event 生成模板草稿；
- `from-cluster` 从异常态势分析中的异常簇生成模板草稿；
- 默认状态为 draft，需要用户确认后发布。

当前实现：
- 已实现 `POST /api/v1/edge-templates/from-cluster`；
- 请求从异常聚合 Top 或优先处理队列带入 `template_text`、`sample_raw_log`、`edge_pred`、`max_severity`、`avg_confidence`、`evidence`、`advice_code` 和 `advice`；
- 后端按 `template_id` 做幂等判断，模板已存在时返回已有模板，不重复插入；
- 新模板默认 `source=abnormal_cluster`、`status=draft`；
- `from-event` 保留为后续从单条实时事件详情沉淀模板的扩展点。

模板集发布：
```text
POST /api/v1/edge-template-sets
POST /api/v1/edge-template-sets/{id}/publish
POST /api/v1/edge-template-sets/{id}/activate
GET  /api/v1/edge-template-sets
GET  /api/v1/edge-template-sets/{id}
```

6. edge-infer 配置发布聚合

当前已存在 `GET /api/v1/edge-config` 用于发布边缘小 LLM 配置。后续应扩展为统一发布物，包含四级分层路径的运行时配置。

GET /api/v1/edge-config

响应头：
```text
ETag: W/"config-108-abc123"
Cache-Control: no-store
```

响应体建议：
```json
{
  "config_version": 108,
  "etag": "W/\"config-108-abc123\"",
  "rules": {
    "version": 31,
    "etag": "W/\"rules-31-aa01\"",
    "items": [
      {
        "id": 12,
        "name": "磁盘空间不足",
        "match_type": "contains",
        "target_field": "template_text",
        "pattern": "no space left on device",
        "edge_pred": "DISK_ERROR",
        "severity": 85,
        "confidence": 0.96,
        "evidence": ["no space left on device"],
        "advice_code": "CHECK_DISK_SPACE",
        "advice": "检查磁盘使用率、日志目录和临时文件清理策略",
        "priority": 100
      }
    ]
  },
  "templates": {
    "version": 18,
    "etag": "W/\"templates-18-bb02\"",
    "items": [
      {
        "template_id": "a81c70b9e4af0c21",
        "template_text": "error in contacting rm",
        "edge_pred": "INFRA_ERROR",
        "severity": 75,
        "confidence": 0.9,
        "evidence": ["ERROR IN CONTACTING RM"],
        "advice_code": "CHECK_RESOURCE_MANAGER",
        "advice": "检查 ResourceManager 状态、网络连通性和相关服务日志"
      }
    ]
  },
  "classifier": {
    "enabled": true,
    "endpoint": "http://127.0.0.1:18080/predict",
    "timeout_ms": 1500,
    "model_version": "bgl-minilm"
  },
  "active_provider": {
    "id": 12,
    "provider": "llama_cpp",
    "endpoint": "http://127.0.0.1:8080/v1/chat/completions",
    "model": "default",
    "timeout_ms": 500,
    "max_tokens": 80,
    "temperature": 0,
    "top_p": 1
  }
}
```

配置发布原则：
- server 端只发布 active 规则集和 active 模板集；
- edge-infer 使用 ETag 轮询，未变化返回 304；
- 新配置拉取成功后先校验，再原子替换内存规则和模板；
- 新配置不合法时继续使用上一份可用配置；
- edge-infer 输出中必须记录 rule_version 和 template_set_version，便于排查命中来源；
- 发布、启用、回滚都必须写入审计日志。
