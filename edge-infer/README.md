# edge-infer

Cotriage 边缘实时推理组件。当前 MVP 实现：

- 消费 `cotriage.events.raw`；
- 日志规范化和稳定模板 ID；
- 内置规则与可配置 JSON 规则；
- 可配置模板知识库和 Redis 模板缓存；
- 可选 HTTP 轻量分类模型路径；
- 未配置模型或模型未命中时输出 `FALLBACK/UNKNOWN`；
- Consumer Group、Pending 接管、重试、幂等检查和 DLQ。
- Redis 不可用时自动等待重连，不再启动即退出；
- 周期性裁剪 raw、inferred 和 DLQ Stream，避免长时间运行写爆 Redis。

当前轻量分类模型使用 `dataset/model-training` 导出的 MiniLM ONNX 部署包，由本地 HTTP classifier 服务承载。

## 本地运行

先启动 Redis port-forward，然后在仓库根目录执行：

```bash
cd edge-infer
REDIS_ADDR=127.0.0.1:6379 go run ./cmd/edge-infer
```

使用示例配置：

```bash
RULES_FILE=./config/rules.example.json \
TEMPLATES_FILE=./config/templates.example.json \
REDIS_ADDR=127.0.0.1:6379 \
go run ./cmd/edge-infer
```

启用 MiniLM ONNX 分类器：

```bash
# 终端 1：在仓库根目录启动 ONNX classifier
python dataset/model-training/serve_onnx.py \
  --onnx-dir dataset/model-training/outputs/bgl-minilm/onnx \
  --host 127.0.0.1 \
  --port 18080

# 终端 2：启动 edge-infer
cd edge-infer
CLASSIFIER_URL=http://127.0.0.1:18080/predict \
REDIS_ADDR=127.0.0.1:6379 \
go run ./cmd/edge-infer
```

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `REDIS_ADDR` | `redis:6379` | Redis 地址 |
| `RAW_STREAM` | `cotriage.events.raw` | 输入 Stream |
| `INFERRED_STREAM` | `cotriage.events.inferred` | 输出 Stream |
| `DLQ_STREAM` | `cotriage.events.dlq` | 死信 Stream |
| `CONSUMER_GROUP` | `edge-infer-g` | 消费者组 |
| `CONSUMER_NAME` | 主机名 | 当前实例名称 |
| `RULES_FILE` | 空 | JSON 规则文件路径；为空时使用内置规则 |
| `TEMPLATES_FILE` | 空 | JSON 模板文件路径 |
| `CLASSIFIER_URL` | 空 | HTTP 分类器 `/predict` 地址；为空时禁用模型路径 |
| `CLASSIFIER_TIMEOUT` | `1.5s` | 调用分类器超时时间 |
| `LLM_URL` | 空 | OpenAI 兼容小模型 `/v1/chat/completions` 地址；为空时禁用边缘 LLM 路径 |
| `LLM_TIMEOUT` | `3s` | 调用边缘 LLM 超时时间 |
| `TEMPLATE_CACHE_TTL` | `5m` | 模板缓存时间 |
| `BLOCK_TIMEOUT` | `2s` | Stream 阻塞读取时间 |
| `CLAIM_IDLE_TIME` | `30s` | Pending 消息接管阈值 |
| `MAX_ATTEMPTS` | `3` | 最大处理次数 |
| `LOG_MAX_BYTES` | `8192` | 单条日志最大处理字节数 |
| `RAW_STREAM_MAXLEN` | `100000` | raw Stream 近似最大长度，作为 log-collector 裁剪之外的兜底 |
| `INFERRED_STREAM_MAXLEN` | `100000` | inferred Stream 近似最大长度 |
| `DLQ_STREAM_MAXLEN` | `10000` | DLQ Stream 近似最大长度 |
| `REDIS_RETRY_DELAY` | `3s` | Redis 不可用或读取失败后的重试间隔 |
| `REDIS_CLEANUP_EVERY` | `1m` | 周期性执行 Stream 裁剪的间隔 |

## Redis 稳定性策略

edge-infer 不再在启动时强制要求 Redis 已经可用。启动后会持续尝试 `PING` 和创建 Consumer Group，Redis 未就绪时每 `REDIS_RETRY_DELAY` 打印一次日志并等待重连。

运行期间如果 `XREADGROUP`、Pending 接管或裁剪遇到 Redis 超时，进程不会退出；主循环会退避后继续重试。

为避免 Redis 被 Stream 写爆，edge-infer 会每 `REDIS_CLEANUP_EVERY` 执行近似裁剪：

```text
XTRIM cotriage.events.raw MAXLEN ~ RAW_STREAM_MAXLEN
XTRIM cotriage.events.inferred MAXLEN ~ INFERRED_STREAM_MAXLEN
XTRIM cotriage.events.dlq MAXLEN ~ DLQ_STREAM_MAXLEN
```

模板缓存读写是尽力而为。缓存不可用时会跳过缓存路径或在 evidence 中记录 `template_cache_error` / `template_cache_write_error`，不会阻断主推理结果发布。

## 推理顺序

1. 规则路径：命中明确规则后直接输出并缓存高置信模板；
2. 模板路径：查配置模板和 Redis 模板缓存；
3. 模型路径：当 `CLASSIFIER_URL` 已配置时，调用 MiniLM ONNX classifier；
4. 边缘小 LLM 路径：当 classifier 输出 `ABNORMAL` 且 `LLM_URL` 已配置时，调用远程 llama.cpp 服务做细粒度诊断；
5. 降级路径：未配置模型或模型不可用时输出 `FALLBACK/UNKNOWN`。

MiniLM 当前只做 `NON_ALERT/ALERT` 二分类。edge-infer 将 `NON_ALERT` 映射为 `NORMAL`，将 `ALERT` 映射为 `ABNORMAL`，后续再交给边缘 LLM 或云端进行细粒度诊断。

远程小模型使用 OpenAI 兼容接口，例如：

```bash
LLM_URL=http://100.85.92.31:18081/v1/chat/completions
```

小模型只能返回 `APP_ERROR/NET_ERROR/DISK_ERROR/INFRA_ERROR/SECURITY/UNKNOWN` 等细分类，不能返回 `ABNORMAL`。

## 测试

```bash
go test ./...
```
