# Web 管理端设计文档

## 1. 设计目标

Cotriage Web 管理端用于展示云边协同日志分诊系统的实时运行状态，并提供设备、规则、模板和模型的管理入口。

界面统一使用简体中文，采用淡蓝色科技风格，技术栈为 Vue 3、TypeScript 和 Ant Design Vue。管理页面使用左侧导航栏与右侧内容区布局。

主要目标：

- 让用户直观看到日志正在持续接入和处理；
- 展示边缘推理的吞吐、时延、分类和路径分布；
- 快速发现积压、降级、死信和高严重度事件；
- 支持从总体指标下钻到单条推理事件；
- 为比赛展示提供可量化、可持续刷新的运行证据。

## 2. 页面结构

首页提供两个独立入口：

1. 进入管理页面：跳转 `/manage`；
2. 查看实时大盘：跳转 `/dashboard`。

管理端包含以下一级模块：

1. 设备管理；
2. 规则管理；
3. 模板知识库；
4. 模型管理。

访问 `/manage` 时默认进入设备管理。实时大盘不使用管理台左侧导航和 Header，而是作为独立全屏页面展示；大盘右上角提供“进入管理页面”按钮。

## 3. 实时日志分诊大盘

### 3.1 顶部状态

页面标题右侧显示 SSE 实时流连接状态：

- 绿色：实时流已连接；
- 橙色：连接中或发生断线重连。

EventSource 具备浏览器原生自动重连能力。SSE 断开不影响 REST 快照每 5 秒刷新。

### 3.2 指标卡

大盘顶部显示 8 个指标：

| 指标 | 含义 |
| --- | --- |
| 近一分钟接入 | raw Stream 最近一分钟新增消息数 |
| 近一分钟推理 | inferred Stream 最近一分钟完成消息数 |
| 待确认消息 | `edge-infer-g` Consumer Group Pending 数量 |
| P95 推理时延 | 最近最多 1000 条推理结果的 P95 `latency_ms` |
| 异常日志比例 | 非 `NORMAL` 结果占最近样本的比例 |
| 降级比例 | `FALLBACK` 路径占最近样本的比例 |
| 累计完成推理 | inferred Stream 当前长度 |
| 死信消息 | DLQ Stream 当前长度 |

Pending、降级率和 DLQ 超过正常范围时，卡片使用警示颜色。

### 3.3 处理趋势

展示最近 10 分钟：

- 每分钟完成推理数量；
- 每分钟异常事件数量。

当前使用 ECharts 双折线面积图展示，支持 Tooltip、图例和自适应 resize。完成推理使用蓝色渐变面积，异常事件使用橙色渐变面积，便于比赛演示时直观看到吞吐与异常走势。

### 3.4 分类与路径分布

分类分布展示：

```text
NORMAL
ABNORMAL
APP_ERROR
NET_ERROR
DISK_ERROR
INFRA_ERROR
SECURITY
UNKNOWN
```

推理路径分布展示：

```text
RULE
CACHE
MODEL
EDGE_LLM
FALLBACK
```

这两个分布用于直观说明日志类型以及四级推理链路中各路径的实际使用比例。

当前使用 ECharts：

- 推理路径分布：横向柱状图；
- 日志分类分布：横向柱状图；
- 空数据时展示“等待数据”占位；
- 图表组件统一封装为 `EChartPanel`，负责初始化、数据更新和容器 resize。

当前视觉调整：

- 推理路径分布改为横向柱状图，便于比较各路径数量，避免饼图标签拥挤；
- 日志分类分布改为横向柱状图，直接展示各分类数量；
- 大盘顶部提供“返回首页”和“进入管理页面”入口；
- 管理台 Header 提供“分诊大盘”和“返回首页”入口。

### 3.5 异常态势分析

在全量实时事件表上方增加“异常态势分析”，用于替代单纯刷屏式异常日志列表。该区域只统计最近 10 分钟 inferred event 中的非 `NORMAL` 事件，让用户优先理解“现在风险多大、异常集中在哪里、先处理什么”。超过 10 分钟的异常不再进入异常聚合 Top、优先处理队列和风险等级判断。

异常态势分析包含：

- 当前风险等级：低、中、高；
- 1 分钟异常数；
- 5 分钟异常数；
- 相比上一个 5 分钟窗口，也就是 5-10 分钟前窗口的增长倍数；
- 异常聚合 Top：按最近 10 分钟内的 `template_id + edge_pred` 聚合，展示异常类型、次数、影响设备、主要推理路径、最高严重度、平均置信度、最近出现时间、样例原始日志和诊断建议；
- 优先处理队列：基于最近 10 分钟内异常簇的严重度、出现次数、影响设备数和置信度计算优先分数，展示最值得先看的异常簇。

风险等级规则：

- 高：近 1 分钟异常数达到 50，或 5 分钟增长倍数达到 3，或出现严重度 90 以上异常；
- 中：近 1 分钟异常数达到 10，或近 5 分钟异常数达到 30，或 5 分钟增长倍数达到 1.5，或出现严重度 70 以上异常；
- 低：未触发上述条件。

该设计的目标不是替代单条日志排查，而是在日志持续进入时先帮助用户完成异常归并和优先级判断。

### 3.6 实时事件流

实时事件表最多保留最近 50 条，字段包括：

- 推理完成时间；
- 设备名称；
- 原始日志摘要；
- 分类结果；
- 严重度；
- 置信度；
- 推理路径；
- 诊断结果摘要；
- 推理耗时。

表格单元格优先展示 `raw_log`，历史事件没有该字段时回退展示 `template_text`。悬浮提示同时展示原始日志和规范化日志模板，兼顾演示直观性与排查需要。

事件行支持展开，展开后展示原始日志、日志模板、证据、诊断建议、建议编码、模型版本和各阶段耗时。`EDGE_LLM` 路径的事件可以直接看到小模型细粒度诊断结果。

颜色规则：

- `NORMAL` 使用绿色；
- 普通异常使用橙色；
- 磁盘和安全异常使用红色；
- 严重度 70 以上使用橙色，90 以上使用红色。

后续可将展开区升级为事件详情抽屉，支持复制 JSON、查看规则版本和跨事件关联结果。

## 4. 数据链路

前端禁止直接连接 Redis。实时大盘的数据链路为：

```text
edge-infer
    -> cotriage.events.inferred
    -> server REST 聚合与 SSE 订阅
    -> Vue 实时大盘
```

REST 用于页面首次加载、统计快照和断线兜底；SSE 用于向浏览器单向推送新推理事件。

### 4.1 REST API

```text
GET /api/v1/dashboard/summary
GET /api/v1/dashboard/timeseries
GET /api/v1/dashboard/abnormal-analysis
GET /api/v1/events?limit=50
```

### 4.2 SSE API

```text
GET /api/v1/events/stream
Content-Type: text/event-stream
event: inferred
```

server 从 inferred Stream 的最新位置开始阻塞读取，只将连接建立后的新事件推送给当前浏览器。连接期间每 15 秒无新消息时发送 keepalive。

### 4.3 刷新策略

- 页面初始化：并行加载 summary、timeseries、abnormal-analysis 和最近事件；
- 每 10 秒：刷新 summary、timeseries 与 abnormal-analysis；
- 实时事件：通过 SSE 到达后进入前端缓冲区，每 500ms 合并更新一次表格，避免高吞吐时每条事件都触发页面重绘；
- 实时事件表支持“暂停刷新”。暂停期间新事件进入前端暂存队列，不再立即插入表格；用户可以展开查看当前事件，再点击“载入新事件”或“继续刷新”合并新数据；
- 页面卸载：关闭 EventSource 并清除定时器。

为避免 Redis 内存随运行时间无限增长，raw 和 inferred Stream 使用近似 `MAXLEN=100000`，DLQ 使用近似 `MAXLEN=10000`。该限制由生产者在 `XADD` 时执行，不依赖大盘请求触发清理。

## 5. 设备管理

设备管理展示边缘集群中的设备列表，列表字段包括：

- 设备 ID `node_name`；
- 设备名称 `equipment`；
- 设备状态 `status`。

点击设备进入详情页，展示 collector 状态、最近事件时间和一分钟接入速率等完整信息。

## 6. 规则管理

规则管理用于配置 edge-infer 四级分层推理中的 `RULE` 路径。页面目标是让用户把明确、低误判风险的日志模式沉淀为可发布规则。

### 6.1 页面结构

规则管理页面包含：

- 规则列表；
- 新建/编辑规则抽屉；
- 规则测试面板；
- 规则集发布与启用区域；
- 发布历史与回滚入口。

规则列表字段：

- 规则名称；
- 匹配类型；
- 匹配目标字段；
- 匹配表达式或关键词摘要；
- 分类结果 `edge_pred`；
- 严重度；
- 置信度；
- 优先级；
- 状态；
- 更新时间；
- 操作：编辑、测试、禁用、删除、加入规则集。

筛选条件：

- 状态：draft / published / disabled；
- 启用状态；
- 分类结果；
- 关键词搜索；
- 优先级范围。

### 6.2 新建与编辑

表单字段：

- 规则名称；
- 规则描述；
- 匹配类型：contains / regex / all_keywords / any_keywords；
- 匹配字段：template_text / raw_log；
- pattern 或 keywords；
- 输出分类；
- 严重度；
- 置信度；
- evidence；
- advice_code；
- advice；
- priority；
- enabled。

前端校验：

- regex 类型必须先通过正则语法校验；
- severity 限制 0-100；
- confidence 限制 0-1；
- evidence 最多 5 条；
- UNKNOWN 不建议设置高置信度；
- NORMAL 规则需要二次确认，避免误把异常日志吞掉。

### 6.3 规则测试

编辑规则时右侧提供测试输入框：

- 输入 raw log；
- 可选输入 template_text；
- 点击“测试规则”；
- 页面展示是否命中、输出分类、严重度、置信度、证据和建议。

测试接口：

```text
POST /api/v1/edge-rules/test
```

测试不会保存规则，也不会影响 edge-infer 当前运行配置。

### 6.4 规则集发布

规则编辑完成后不会立即影响 edge-infer。用户需要：

1. 将规则加入规则集草稿；
2. 点击“发布规则集”；
3. 校验通过后生成新版本；
4. 点击“启用”后成为 active 规则集；
5. edge-infer 下一次拉取 `edge-config` 后生效。

页面应展示：

- 当前 active 规则集版本；
- 待发布草稿版本；
- 规则数量；
- 最近发布时间；
- 最近启用时间；
- 发布人；
- 回滚按钮。

## 7. 模板知识库管理

模板知识库用于配置 edge-infer 四级分层推理中的 `CACHE`/模板路径。它不是 Redis 临时缓存，而是人工确认后的稳定模板结果库。

### 7.1 页面结构

模板知识库页面包含：

- 模板列表；
- 新建/编辑模板抽屉；
- 模板测试面板；
- 从事件或异常聚合沉淀模板入口；
- 模板集发布与启用区域；
- 发布历史与回滚入口。

模板列表字段：

- template_id；
- template_text；
- 样例原始日志；
- 分类结果；
- 严重度；
- 置信度；
- evidence 摘要；
- advice_code；
- 来源：manual / inferred_event / abnormal_cluster / import；
- 状态；
- 最近命中时间；
- 操作：编辑、测试、禁用、删除、加入模板集。

筛选条件：

- 状态；
- 分类结果；
- 来源；
- template_id；
- 关键词搜索；
- 最近命中时间。

### 7.2 新建与编辑

表单字段：

- template_text；
- raw_example；
- edge_pred；
- severity；
- confidence；
- evidence；
- advice_code；
- advice；
- source。

设计约束：

- template_id 由 server 根据统一 normalize/template hash 算法生成；
- 前端不允许手动填写 template_id；
- confidence 低于 0.85 时可以保存草稿，但发布时需要二次确认或禁止发布；
- 模板结果必须是确定性结论，模糊诊断应交给分类模型或边缘小 LLM。

### 7.3 模板测试

用户输入 raw log 后，server 返回：

- normalize 后的 template_text；
- template_id；
- 当前 active 模板集是否命中；
- 命中的模板结果。

测试接口：

```text
POST /api/v1/edge-templates/test
```

### 7.4 从大盘沉淀模板

在实时事件详情和异常态势分析中增加“加入模板库”入口：

- 从单条 inferred event 生成模板草稿；
- 从异常聚合 Top 生成模板草稿；
- 从优先处理队列生成模板草稿；
- 自动带入 template_id、template_text、raw_example、edge_pred、severity、confidence、evidence 和 advice；
- 用户确认后保存为 draft；
- draft 发布并启用后，后续相同模板走模板路径，不再调用分类模型或小 LLM。

当前实现：

- 异常聚合 Top 卡片提供“沉淀为模板”按钮；
- 优先处理队列条目提供“沉淀为模板”按钮；
- 点击后调用 `POST /api/v1/edge-templates/from-cluster`；
- 后端发现模板已存在时不重复创建，前端提示“模板知识库中已存在该模板”；
- 新沉淀模板默认保存为 draft，需要进入模板知识库审核后发布。

接口：

```text
POST /api/v1/edge-templates/from-event
POST /api/v1/edge-templates/from-cluster
```

### 7.5 模板集发布

模板编辑完成后不会立即影响 edge-infer。用户需要：

1. 将模板加入模板集草稿；
2. 点击“发布模板集”；
3. 校验通过后生成新版本；
4. 点击“启用”后成为 active 模板集；
5. edge-infer 下一次拉取 `edge-config` 后生效。

## 8. 模型管理

模型管理用于配置 edge-infer 四级分层推理中的 `MODEL` 和 `EDGE_LLM` 路径。

页面使用 Tab 拆分：

- 轻量分类模型：管理 MiniLM / DistilBERT ONNX HTTP classifier；
- 边缘小 LLM：管理 llama.cpp / vLLM / OpenAI 兼容模型服务。

### 8.1 轻量分类模型

列表字段：

- 名称；
- Endpoint，例如 `http://127.0.0.1:18080/predict`；
- 模型版本；
- 告警阈值；
- 状态；
- 是否生效；
- 操作：编辑、启用、删除。

表单字段：

- 名称；
- Endpoint；
- 模型版本；
- 超时 ms；
- 告警阈值；
- 状态。

启用后，server 会将 active classifier 写入 `GET /api/v1/edge-config`，edge-infer 下一次轮询后热更新 `MODEL` 路径。

### 8.2 边缘小 LLM

列表字段：

- 名称；
- Provider 类型：llama_cpp / openai_compat / vllm / custom_http；
- Endpoint；
- 模型名；
- 状态；
- 是否生效；
- 操作：编辑、启用、删除。

表单字段：

- 名称；
- Provider；
- Endpoint；
- 模型名；
- 超时 ms；
- max_tokens；
- temperature；
- top_p；
- response_path；
- healthcheck_path；
- 状态。

启用后，server 会将 active provider 写入 `GET /api/v1/edge-config`，edge-infer 下一次轮询后热更新 `EDGE_LLM` 路径。

页面应展示：

- 当前 active 模板集版本；
- 模板数量；
- 最近发布时间；
- 最近启用时间；
- 发布人；
- 回滚按钮。

## 8. 配置生效提示

规则管理和模板知识库页面都需要明确提示：

- 保存草稿不会立即生效；
- 发布后仍需启用；
- edge-infer 通过 `GET /api/v1/edge-config` 轮询拉取配置；
- 配置变化存在短暂生效延迟；
- 若新配置校验失败，edge-infer 会继续使用上一份可用配置。

## 9. 当前实现状态

已完成：

- 首页管理台与实时大盘双入口；
- `/dashboard` 独立全屏大盘路由；
- 大盘右上角管理页面跳转按钮；
- 8 个实时指标卡；
- 最近 10 分钟 ECharts 处理趋势；
- ECharts 分类与推理路径分布；
- 异常态势分析，包括风险等级、异常聚合 Top 和优先处理队列；
- 最近事件 REST 查询；
- inferred Stream SSE 实时推送；
- SSE 连接状态和自动重连显示；
- server 端 Redis 聚合接口；
- 前后端生产构建验证。
- Vite 将 `echarts` 和 `ant-design-vue` 拆分为独立 vendor chunk，降低主业务包体积并便于浏览器缓存。

待完成：

- 单条事件详情 API 和详情抽屉；
- 原始日志与 inferred event 的关联展示；
- 规则管理页面与规则集发布；
- 模板知识库页面与模板集发布；
- 两类模型的管理页面；
- Prometheus 长周期指标存储；
- 时间范围选择、设备过滤和指标导出；
- 大盘接口单元测试和高并发 SSE 压力测试。

## 10. 本地验证

1. 执行 `./script/forward-datastores.sh`；
2. 启动 VS Code 中的 `server`、`edge-infer` 和 `log-collector`；
3. 在 `web` 目录运行 `npm run dev`；
4. 访问 `http://127.0.0.1:5173/dashboard`；
5. 确认接入量、推理量和累计完成数增长；
6. 确认实时事件表持续出现新数据；
7. 确认浏览器显示“实时流已连接”。
