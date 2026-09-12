# Server 组件开发记录

## 日期：2026年06月22日

### 内容

1. 新增大盘汇总接口 `GET /api/v1/dashboard/summary`，从 Redis Streams 聚合累计数量、近一分钟速率、Pending、DLQ、P95 时延、异常比例、降级比例、分类和路径分布。
2. 新增趋势接口 `GET /api/v1/dashboard/timeseries`，返回最近 10 分钟完成推理量和异常事件量。
3. 新增事件列表接口 `GET /api/v1/events?limit=50`，默认返回最近 50 条 inferred event，最大允许 200 条。
4. 新增 SSE 接口 `GET /api/v1/events/stream`，使用 Redis `XREAD BLOCK` 从 inferred Stream 最新位置读取新事件，每 15 秒发送 keepalive。
5. 在 server 配置中新增 `INFERRED_STREAM` 和 `DLQ_STREAM`，默认分别为 `cotriage.events.inferred` 和 `cotriage.events.dlq`。
6. 针对大盘卡顿降低统计开销：summary 最近样本由 1000 条降至 500 条，timeseries 由 2000 条降至 1000 条。
7. raw Stream 近一分钟速率查询上限由 10000 条降低到 2000 条，避免单次请求反序列化过多消息；超过 2000 条/分钟时当前指标显示为封顶值。
8. SSE 连接保持轻量设计：每个浏览器连接对应一个阻塞 Redis 读取，不在 server 内持续缓存历史事件。
9. 新增 `server/.gitignore` 中的本地 Go build cache 忽略规则。

### 验证结果

- `go test ./...`：通过。
- 直接请求旧 server 进程时新接口曾返回 404，确认需要在代码变更后重新启动 VS Code server 调试进程。
- 当前 Redis 实测占用约 135MB；raw Stream 约 294488 条，inferred Stream 约 33240 条，DLQ 为 0 条。

### 后续事项

1. 为大盘聚合增加短时服务端缓存，避免多个浏览器重复扫描相同样本。
2. 将高吞吐速率统计改为 collector 写入分钟计数器，避免依赖 Stream 范围查询及 2000 条上限。
3. 增加大盘 handler 单元测试和多 SSE 连接压力测试。
4. 后续将长周期指标写入 Prometheus 或时序存储，不使用 Redis Stream 承担长期统计。

## 日期：2026年06月23日（事件流透传原始日志）

### 内容

1. Dashboard inferred event DTO 新增 `raw_log` 字段。
2. `GET /api/v1/events` 和 `GET /api/v1/events/stream` 会透传 edge-infer 输出的 `raw_log`。
3. 兼容旧 inferred event：历史消息没有 `raw_log` 时仍可正常解码。

### 验证结果

- `go test ./...` 通过；验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

## 日期：2026年06月23日（异常态势分析接口）

### 内容

1. 新增 `GET /api/v1/dashboard/abnormal-analysis` 接口。
2. 接口从 inferred Stream 最近 2000 条事件中筛选非 `NORMAL` 异常事件。
3. 新增当前风险等级 `risk_level`，基于 1 分钟异常数、5 分钟异常数、增长倍数和最高严重度计算。
4. 新增异常类型统计 `top_types`。
5. 新增异常聚合 `top_clusters`，按 `template_id + edge_pred` 聚合异常，返回次数、影响设备数、最高严重度、平均置信度、首次/最近出现时间、样例原始日志、证据和诊断建议。
6. 新增优先处理队列 `priority_queue`，基于严重度、次数、影响设备数和置信度计算 `priority_score`。
7. 更新 `design_doc/web_design.md`，补充异常态势分析的数据口径和 REST API。

### 验证结果

- `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/.cache/go-build-126-server go test ./...`：通过。

## 日期：2026年06月23日（异常态势分析 10 分钟窗口）

### 内容

1. 调整 `GET /api/v1/dashboard/abnormal-analysis` 的统计口径。
2. 异常类型统计、异常聚合 Top、优先处理队列和风险等级只使用最近 10 分钟内的非 `NORMAL` inferred event。
3. 超过 10 分钟的异常事件不再进入异常态势分析，避免旧异常在页面长期挂起。
4. 1 分钟异常数和 5 分钟异常数保持原有时间窗口。
5. 增长倍数改为使用最近 5 分钟窗口与 5-10 分钟前窗口对比。
6. 更新 `design_doc/web_design.md`，明确异常态势分析只反映最近 10 分钟当前态势。

### 验证结果

- `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/.cache/go-build-126-server go test ./...`：通过。

## 日期：2026年06月24日（补充实时大盘设计文档）

### 内容

1. 更新 `design_doc/server.md`，新增“实时大盘与异常态势分析”章节。
2. 补充大盘汇总接口 `GET /api/v1/dashboard/summary` 的字段口径。
3. 补充处理趋势接口 `GET /api/v1/dashboard/timeseries`。
4. 补充最近事件列表 `GET /api/v1/events` 和 SSE 实时事件流 `GET /api/v1/events/stream`。
5. 补充异常态势分析接口 `GET /api/v1/dashboard/abnormal-analysis` 的统计窗口、响应示例和字段含义。
6. 明确 LOW/MEDIUM/HIGH 风险等级阈值。
7. 补充优先处理队列 `priority_score` 的经验公式，并说明后续应配置化。

### 验证结果

- 文档补充，无代码变更。

## 日期：2026年06月24日（规则管理与模板管理设计）

### 内容

1. 更新 `design_doc/server.md`，新增规则管理设计。
2. 规则管理设计包含 `edge_rule`、`edge_rule_set`、`edge_rule_set_item` 表结构。
3. 规则管理设计补充规则状态、匹配类型、校验规则、测试接口、规则集发布和启用流程。
4. 更新 `design_doc/server.md`，新增模板知识库管理设计。
5. 模板管理设计包含 `edge_template`、`edge_template_set`、`edge_template_set_item` 表结构。
6. 模板管理设计补充模板来源、模板测试、从事件/异常聚合沉淀模板、模板集发布和启用流程。
7. 补充统一 `GET /api/v1/edge-config` 发布物设计，将 rules、templates、classifier 和 active_provider 聚合为 edge-infer 可拉取的运行时配置。

### 验证结果

- 设计文档补充，无代码变更。

## 日期：2026年06月24日（规则管理与模板管理后端实现）

### 内容

1. 新增规则管理数据模型、GORM 表模型和 store 层，实现规则创建、编辑、列表、软删除。
2. 新增模板知识库数据模型、GORM 表模型和 store 层，实现模板创建、编辑、列表、软删除。
3. 新增规则集和模板集发布/启用能力，启用时生成 `config_version` 与 `etag`。
4. 新增 REST API：`/api/v1/edge-rules`、`/api/v1/edge-rule-sets`、`/api/v1/edge-templates`、`/api/v1/edge-template-sets`。
5. `GET /api/v1/edge-config` 聚合 active provider、active rules、active templates，并生成组合 ETag，避免规则/模板变更时 edge-infer 误拿 304。
6. `EnsureSchema` 自动迁移新增规则、模板、规则集、模板集表。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（预置常用规则）

### 内容

1. 新增 server 启动阶段的内置规则 seed 逻辑。
2. 内置规则覆盖 OOM、磁盘空间不足、磁盘 I/O、网络连接失败、DNS 失败、请求超时、认证鉴权失败、证书异常、Kubernetes 镜像拉取失败、CrashLoopBackOff、调度失败、MySQL、Redis、HTTP 5xx、Hadoop/YARN 和健康检查正常噪声。
3. 内置规则默认 `published + enabled`，降低首次使用门槛。
4. seed 逻辑幂等执行：规则名称已存在时不覆盖用户已有配置。
5. 如果系统还没有 active 规则集，自动创建并启用“内置常用规则集”；如果已有 active 规则集，只补齐缺失内置规则，不改变当前生效配置。
6. 更新 `design_doc/server.md`，补充内置规则机制，并修正规则优先级说明为“数值越大越先匹配”。
7. 新增集成测试覆盖内置规则 seed 的幂等性和 active 规则集创建逻辑。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（修复内置规则 seed 启动超时）

### 问题

VS Code 调试启动 server 时出现：

```text
seed builtin rules: context deadline exceeded
```

原因是 server 启动阶段将 `EnsureSchema` 和 `SeedBuiltinRules` 放在同一个 8 秒 context 中执行。MySQL 首次迁移或连接较慢时，schema 迁移消耗了大部分时间，seed 阶段拿到的 context 已接近超时，导致启动失败。

### 内容

1. 将 schema 迁移和内置规则 seed 拆成两个独立 context。
2. schema 迁移超时时间调整为 15 秒。
3. 内置规则 seed 单独使用 30 秒超时，避免被迁移阶段消耗时间。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（异常聚合沉淀模板接口）

### 内容

1. 新增 `CreateTemplateFromClusterRequest` 请求模型。
2. 新增 `POST /api/v1/edge-templates/from-cluster` 接口，用于从异常态势分析中的异常聚合生成模板草稿。
3. 接口自动带入 `template_text`、`sample_raw_log`、`edge_pred`、`max_severity`、`avg_confidence`、`evidence`、`advice_code` 和 `advice`。
4. 新模板默认 `source=abnormal_cluster`、`status=draft`。
5. 后端按 `template_id` 幂等判断，已存在模板时返回已有模板，避免重复沉淀污染模板知识库。
6. 更新 `design_doc/server.md`，补充当前已实现的 `from-cluster` 沉淀流程。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（模型管理后端实现）

### 内容

1. 复用已有 `edge_providers` 作为边缘小 LLM Provider 管理后端。
2. 新增 `classifier_configs` 表，用于管理轻量分类模型 HTTP classifier。
3. 新增 `ClassifierConfigsStore`，支持分类模型创建、编辑、列表、删除和启用。
4. 新增 REST API：`/api/v1/edge-classifiers`、`/api/v1/edge-classifiers/{id}/activate`。
5. `EnsureSchema` 新增 `ClassifierConfigModel` 自动迁移。
6. `GET /api/v1/edge-config` 新增 active classifier，并将 classifier ETag 纳入组合 ETag。
7. 更新 `design_doc/server.md`，补充轻量分类模型管理表结构、API 和启用语义。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（异常聚合补充推理路径）

### 内容

1. 异常态势分析中的异常聚合新增 `inference_paths` 字段。
2. 异常态势分析中的异常聚合新增 `primary_inference_path` 字段。
3. 聚合时按推理路径出现次数降序排序，出现次数最多的路径作为主路径。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。
