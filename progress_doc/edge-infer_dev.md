# edge-infer 组件开发记录

## 日期：2026年06月22日

### 内容

1. 明确 edge-infer 第一阶段 MVP 范围：先实现规则路径、模板缓存路径和统一降级，不在模型尚未接入时伪造分类结果。
2. 创建独立 Go Module 和 `cmd/edge-infer` 启动入口，支持环境变量配置和 SIGINT/SIGTERM 优雅退出。
3. 定义 raw event 与 inferred event 数据结构，补齐模板、证据、推理路径、版本和阶段耗时字段。
4. 实现日志规范化：去除不稳定前缀，对 UUID、IP、十六进制值、路径和数字进行占位符归一化，并生成稳定模板 ID。
5. 实现规则引擎：支持优先级、启用状态、正则校验、分类、严重度、置信度、证据和处理建议；提供 OOM、磁盘满、网络失败和认证失败内置规则。
6. 实现用户配置入口：通过 `RULES_FILE` 加载 JSON 规则，通过 `TEMPLATES_FILE` 加载人工模板及其结果，并提供示例文件。
7. 实现 Redis 模板缓存：高置信度且非 UNKNOWN 的结果按模板 ID、模型版本和规则版本缓存，默认 TTL 为 5 分钟。
8. 实现 Redis Streams 消费链路：自动创建 Consumer Group，使用 `XREADGROUP` 消费，输出成功后执行 `XACK`。
9. 实现可靠性机制：短期幂等键、Pending 消息 `XAUTOCLAIM`、失败次数记录、最大重试和 DLQ。
10. 添加 Dockerfile、Makefile、README 和本地运行配置说明。
11. 添加规范化、模板 ID、规则优先级、非法规则和默认磁盘规则测试。

### 验证结果

- `go test ./...`：通过。
- `go vet ./...`：通过。
- 本机默认 `GOROOT` 存在 Go 1.26.1 可执行文件与 Go 1.25.3 标准库错配，验证时显式使用 `/opt/homebrew/Cellar/go/1.26.1/libexec`。
- 已通过本机 Redis port-forward 完成隔离 Stream 联调：向 `cotriage.test.edge-infer.raw` 写入磁盘满事件，组件成功在 inferred Stream 输出 `DISK_ERROR`、严重度 `92`、置信度 `0.99` 和 `RULE` 路径结果。
- 尚未与持续运行的 log-collector 和业务 Stream 做长时间联调。

### 下一步

1. 与持续运行的 log-collector 做长时间联调，并注入失败验证 Pending 接管和 DLQ。
2. 在 server 中设计规则、模板和统一配置发布快照。
3. edge-infer 增加 ETag 配置拉取、校验、原子切换和回滚。
4. 接入轻量分类模型，并建立准确率、F1、时延和内存基准。

## 日期：2026年06月22日（实时大盘与容量优化）

### 内容

1. 在 `.vscode/launch.json` 中增加 edge-infer 调试配置，直接运行 `edge-infer/cmd/edge-infer` 并连接本机 Redis port-forward。
2. 调试配置补齐 raw、inferred、DLQ、Consumer Group、Consumer Name、规则文件、模板文件、缓存 TTL、Pending 接管和最大重试参数。
3. 为 inferred Stream 增加 `INFERRED_STREAM_MAXLEN`，默认近似上限为 `100000`。
4. 为 DLQ Stream 增加 `DLQ_STREAM_MAXLEN`，默认近似上限为 `10000`。
5. edge-infer 对 inferred 和 DLQ 执行 `XADD` 时设置近似 `MAXLEN`，避免长期运行导致 Redis 内存无限增长。
6. 更新 edge-infer README，记录两个新增环境变量及默认值。
7. 配合实时大盘联调，server 可通过 inferred Stream 展示分类、严重度、推理路径、置信度和时延。

### 验证结果

- `go test ./...`：通过。
- Go 代码已通过 `gofmt`。
- Redis 实测 inferred Stream 约 33240 条，尚未达到新设置的 100000 条上限。

### 后续事项

1. 重启 edge-infer，使新的 Stream 长度限制生效。
2. 与持续运行的 log-collector 联调，观察 inferred 增长、SSE 推送和前端批量渲染。
3. 根据比赛实验需要确定 inferred event 的合理保留时长，并评估是否落库保存长期结果。

## 日期：2026年06月23日（接入 DistilBERT ONNX 分类器）

### 内容

1. 新增 HTTP classifier 抽象，edge-infer 通过 `CLASSIFIER_URL` 调用本地模型服务。
2. 新增 `CLASSIFIER_TIMEOUT` 配置，默认 `1.5s`，避免模型服务异常拖慢 Redis Stream 消费。
3. 推理顺序更新为：规则路径、模板缓存路径、HTTP 模型路径、降级路径。
4. 模型路径请求体包含 `event_id`、`template_id` 和 `template_text`。
5. 模型路径响应体支持 `label`、`label_id`、`alert_probability`、`threshold`、`probabilities` 和 `model_version`。
6. 当前 DistilBERT 二分类模型只判断 `NON_ALERT/ALERT`：`NON_ALERT` 映射为 `NORMAL`，`ALERT` 映射为 `ABNORMAL`。
7. 模型路径会写入 `inference_path=MODEL`、`model_version`、`stage_latency_ms.model`、置信度和证据字段。
8. 高置信且非 UNKNOWN 的模型结果继续进入 Redis 模板缓存，后续相同模板优先走缓存路径。
9. edge-infer README 增加 ONNX classifier 启动方式、`CLASSIFIER_URL` 配置和推理顺序说明。

### 验证结果

- 已新增 HTTP classifier 单元测试，覆盖请求编码、响应解析和 `ALERT/NON_ALERT` 映射逻辑。
- `go test ./...` 通过；由于本机 Go 默认安装存在版本错配，验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

### 后续事项

1. 启动 `dataset/model-training/serve_onnx.py`。
2. 以 `CLASSIFIER_URL=http://127.0.0.1:18080/predict` 启动 edge-infer。
3. 注入一条规则和模板均不命中的日志，确认 inferred Stream 输出 `inference_path=MODEL`。

## 日期：2026年06月23日（调试配置与异常类别修正）

### 内容

1. `.vscode/launch.json` 新增 `onnx-classifier` Python 调试配置，用于启动 `dataset/model-training/serve_onnx.py`。
2. `edge-infer` 调试配置新增 `CLASSIFIER_URL=http://127.0.0.1:18080/predict` 和 `CLASSIFIER_TIMEOUT=1500ms`。
3. 修正 DistilBERT 二分类映射：`ALERT` 不再映射为具体根因 `INFRA_ERROR`，而是映射为通用异常 `ABNORMAL`。
4. 新增 `PredAbnormal=ABNORMAL`，并加入规则/模板合法预测集合。
5. 更新 edge-infer 设计文档、README 和测试用例中的类别映射说明。

### 验证结果

- 已完成 Go 代码 gofmt。
- `.vscode/launch.json` JSON 格式校验通过。
- Python 训练/导出/服务脚本静态语法检查通过。
- `go test ./...` 通过；验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

### 后续事项

1. 使用 VS Code 先启动 `onnx-classifier`，再启动 `edge-infer`。
2. 注入未命中规则和模板的异常日志，确认输出 `edge_pred=ABNORMAL` 且 `inference_path=MODEL`。

## 日期：2026年06月23日（输出原始日志摘要）

### 内容

1. inferred event 新增 `raw_log` 字段，用于承载原始日志摘要。
2. `raw_log` 在 edge-infer 输出阶段从 `raw.log.raw` 填充，并限制为最多 2048 个字符，避免超长日志拖慢 Redis Stream、SSE 和前端表格。
3. `template_text` 继续保留，用于模型推理、模板缓存和排查。
4. edge-infer 设计文档的输出 payload 示例增加 `raw_log`。

### 验证结果

- `go test ./...` 通过；验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

### 后续事项

1. 重启 edge-infer，使新输出字段进入 inferred Stream。
2. 前端实时事件流展示 `raw_log`，并在悬浮提示中保留 `template_text`。

## 日期：2026年06月23日（接入远程边缘小 LLM）

### 内容

1. 用户确认 Linux 服务器上的 llama.cpp 服务健康检查可访问：`http://100.85.92.31:18081/health` 返回 `{"status":"ok"}`。
2. 新增 OpenAI Chat Completions 兼容小模型客户端，支持调用 `LLM_URL=/v1/chat/completions`。
3. 新增 `LLM_URL` 和 `LLM_TIMEOUT` 配置，默认不启用小模型，超时时间默认 `3s`。
4. 推理链路更新为：`RULE -> CACHE -> MODEL -> EDGE_LLM -> FALLBACK`。
5. 当 DistilBERT classifier 输出 `ABNORMAL` 且配置了 `LLM_URL` 时，edge-infer 调用远程小模型做细粒度诊断。
6. 小模型输出只允许 `APP_ERROR/NET_ERROR/DISK_ERROR/INFRA_ERROR/SECURITY/UNKNOWN`，不允许继续输出泛化的 `ABNORMAL`。
7. 小模型输出 JSON 会被校验 `edge_pred`、`severity`、`confidence`、`evidence`、`advice_code` 和 `advice`。
8. 小模型调用失败、超时或输出不合法时，保留 classifier 的 `ABNORMAL/MODEL` 结果，并在 evidence 中追加 `edge_llm_error`。
9. `.vscode/launch.json` 的 edge-infer 调试配置新增 `LLM_URL=http://100.85.92.31:18081/v1/chat/completions` 和 `LLM_TIMEOUT=3s`。

### 验证结果

- 已新增小模型 JSON 解析和结果映射单元测试。
- `.vscode/launch.json` JSON 格式校验通过。
- `go test ./...` 通过；验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

### 后续事项

1. 重启 edge-infer，使 `LLM_URL` 生效。
2. 注入一条 classifier 输出 `ABNORMAL` 的日志，确认 inferred Stream 输出 `inference_path=EDGE_LLM` 和细分类。

## 日期：2026年06月23日（小模型半截 JSON 修复）

### 内容

1. 调试发现小模型输出在 `json.Unmarshal` 处报 `unexpected end of JSON input`，说明返回内容包含 JSON 开头但未完整闭合。
2. 优化 LLM system prompt，要求输出单行紧凑 JSON，缩短 evidence 和 advice 长度，减少小模型输出被截断概率。
3. LLM 请求显式设置 `stream=false`，避免服务端流式输出被误解析。
4. `parseLLMJSON` 先严格解析完整 JSON；严格解析失败后，尝试从半截输出中提取 `edge_pred`、`severity`、`confidence`、`evidence`、`advice_code` 和 `advice`。
5. 半截 JSON 修复成功时会在 evidence 中追加 `llm_json_repaired` 标记，便于后续审计。
6. 修复失败时错误信息会包含 `content_preview`，后续无需进入调试器也能看到小模型原始输出片段。

### 验证结果

- 新增半截 JSON 修复单元测试。
- `go test ./...` 通过；验证时使用 `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec` 和项目内 `GOCACHE`。

### 后续事项

1. 重启 edge-infer，观察 `edge_llm_error=decode edge llm JSON` 是否消失。
2. 若仍频繁出现，继续调低 llama.cpp 输出长度或改用 grammar/json-schema 约束。

## 日期：2026年06月23日（ONNX classifier 切换为 MiniLM）

### 内容

1. VS Code `onnx-classifier` 调试配置中的 `--onnx-dir` 从 `outputs/bgl-distilbert/onnx` 切换为 `outputs/bgl-minilm/onnx`。
2. `edge-infer/README.md` 中的轻量分类模型说明从 DistilBERT 更新为 MiniLM。
3. edge-infer 的 `CLASSIFIER_URL` 不需要变化，仍然调用本地 `http://127.0.0.1:18080/predict`。
4. MiniLM classifier 仍只输出 `NON_ALERT/ALERT`，edge-infer 继续将 `ALERT` 映射为 `ABNORMAL`，再交给边缘小 LLM 做细粒度诊断。

### 验证结果

- MiniLM ONNX 单条推理校验通过：`KERNEL FATAL rts panic! - stopping execution` 输出 `ALERT`，`alert_probability=0.989299`。

### 后续事项

1. 使用 VS Code 重新启动 `onnx-classifier`。
2. 重启 edge-infer，使新 classifier 服务进入推理链路。

## 日期：2026年06月24日（Redis 自动裁剪与启动解耦）

### 内容

1. 移除 edge-infer 启动阶段对 Redis `PING` 的强依赖，Redis 不可用时不再 `fatal` 退出。
2. worker 主循环新增 Redis 就绪等待逻辑：`PING` 或 Consumer Group 创建失败时按 `REDIS_RETRY_DELAY` 退避重试。
3. `XREADGROUP` 读取失败时不再直接退出进程，而是记录日志后退避重试。
4. 新增 Redis 自动裁剪机制，按 `REDIS_CLEANUP_EVERY` 周期执行 raw、inferred、DLQ 三个 Stream 的 `XTRIM MAXLEN ~`。
5. 新增配置项：`RAW_STREAM_MAXLEN`、`REDIS_RETRY_DELAY`、`REDIS_CLEANUP_EVERY`。
6. 模板缓存读写改为尽力而为：缓存读失败会跳过 Redis 缓存路径，缓存写失败会在 evidence 中记录错误，不再阻断主推理结果发布。
7. 更新 `.vscode/launch.json`、`edge-infer/README.md` 和 `sop/redis-sop.md`，补充自动裁剪、重试和人工清理流程。

### 验证结果

- `gofmt` 已执行。
- `.vscode/launch.json` JSON 格式校验通过。
- `GOROOT=/opt/homebrew/Cellar/go/1.26.1/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/.cache/go-build-126-edge go test ./...`：通过。

### 后续事项

1. 在 Redis port-forward 未启动时启动 edge-infer，确认进程持续等待而非退出。
2. Redis 恢复后观察 edge-infer 是否自动创建 Consumer Group 并开始消费。
3. 观察 Redis Stream 长度是否稳定在配置上限附近。

## 日期：2026年06月24日（规则与模板运行时配置设计）

### 内容

1. 更新 `design_doc/edge-infer.md`，补充规则路径从本地 `RULES_FILE` 迁移到 server active 规则集的设计。
2. 补充规则加载、优先级匹配、校验失败保留上一份可用规则集、输出记录 `rule_version` 等设计。
3. 补充模板路径拆分：管理端模板知识库优先，Redis 临时模板缓存作为性能优化兜底。
4. 补充模板知识库通过 server active 模板集发布，edge-infer 拉取后原子替换内存模板索引的设计。
5. 新增“配置拉取与热更新”章节，说明 `GET /api/v1/edge-config`、ETag、304、失败保留旧配置、原子替换和配置版本追踪。
6. 修正后续章节编号。

### 验证结果

- 设计文档补充，无代码变更。

## 日期：2026年06月24日（接入 server 发布的规则与模板配置）

### 内容

1. edge-infer 新增 `EDGE_CONFIG_URL`、`EDGE_CONFIG_REFRESH_INTERVAL`、`EDGE_CONFIG_TIMEOUT` 配置项。
2. 新增远程配置轮询客户端，使用 `If-None-Match` 拉取 server 的 `/api/v1/edge-config`。
3. 支持将 server 发布的规则转换为 edge-infer 运行时规则引擎。
4. 支持将 server 发布的模板知识库转换为 edge-infer 运行时模板索引。
5. `Engine` 新增规则和模板的热更新能力，更新时原子替换运行时配置，不要求重启 edge-infer。
6. `.vscode/launch.json` 的 edge-infer debug 配置新增 `EDGE_CONFIG_URL`，本地调试默认从 `http://127.0.0.1:8080/api/v1/edge-config` 拉取活动配置。
7. server 侧模板 ID 生成算法与 edge-infer 对齐，避免模板知识库发布后因 ID 不一致无法命中。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。

## 日期：2026年06月24日（接入 server 发布的模型配置）

### 内容

1. edge-infer 远程配置解析新增 `classifier` 和 `active_provider`。
2. active classifier 会重建 HTTP classifier 客户端，接管 `MODEL` 路径。
3. active provider 会重建 OpenAI 兼容 LLM 客户端，接管 `EDGE_LLM` 路径。
4. `OpenAIChatLLM` 支持 server 下发的 `model`、`timeout_ms`、`max_tokens`、`temperature`、`top_p` 和 `seed`。
5. `Engine` 新增模型客户端热更新能力，处理中的日志使用进入处理时的模型快照，新日志使用新配置。
6. 如果 server 未配置 active classifier 或 active provider，对应模型路径会关闭，系统继续执行后续路径或降级。
7. 更新 `design_doc/edge-infer.md`，补充模型配置热更新当前实现。

### 验证结果

- `GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/Users/pangjian/researchProjects/cotriage-dev/tmp/go-build-cache go test ./...`：通过。
