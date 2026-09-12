# Redis 排查与清理 SOP

## 1. 先看 Redis 是否可用

```bash
kubectl -n cotriage exec deploy/redis -- redis-cli PING
```

返回 `PONG` 表示 Redis 服务本身可用。

## 2. 查看 Stream 长度

```bash
kubectl -n cotriage exec deploy/redis -- redis-cli XLEN cotriage.events.raw
kubectl -n cotriage exec deploy/redis -- redis-cli XLEN cotriage.events.inferred
kubectl -n cotriage exec deploy/redis -- redis-cli XLEN cotriage.events.dlq
```

如果 raw 或 inferred 持续增长到十万级以上，说明日志输入或推理输出正在快速占用 Redis 内存。

## 3. 推荐的安全清理方式：裁剪 Stream

优先使用 `XTRIM`，不要一上来 `FLUSHDB`。`XTRIM` 只裁剪 Stream 历史消息，不会清空整个 Redis DB。

```bash
kubectl -n cotriage exec deploy/redis -- redis-cli XTRIM cotriage.events.raw MAXLEN '~' 100000
kubectl -n cotriage exec deploy/redis -- redis-cli XTRIM cotriage.events.inferred MAXLEN '~' 100000
kubectl -n cotriage exec deploy/redis -- redis-cli XTRIM cotriage.events.dlq MAXLEN '~' 10000
```

说明：

- `~` 表示近似裁剪，性能更好；
- raw 和 inferred 默认保留约 100000 条；
- DLQ 默认保留约 10000 条；
- edge-infer 现在也会每分钟自动执行一次上述裁剪，作为兜底保护。

## 4. 查看 Pending 消息

```bash
kubectl -n cotriage exec deploy/redis -- redis-cli XINFO GROUPS cotriage.events.raw
```

关注 `pending` 字段：

- 偶尔有少量 pending 正常；
- pending 持续升高，通常表示 edge-infer 处理慢、Redis 超时或模型调用过慢。

## 5. 最后手段：清空当前 DB

只有在确认可以丢弃所有运行态数据时才执行：

```bash
kubectl -n cotriage exec deploy/redis -- redis-cli FLUSHDB
```

影响：

- 清空 raw/inferred/DLQ Stream；
- 清空 Consumer Group 状态；
- 清空 edge-infer 模板缓存、幂等键和重试计数；
- 前端大盘会从空状态重新开始。

执行后需要重启或等待以下组件恢复：

1. `log-collector` 重新写入 raw Stream；
2. `edge-infer` 自动重建 Consumer Group；
3. `server` 大盘重新读取 Redis 快照。

## 6. edge-infer Redis 自动保护参数

edge-infer 支持以下环境变量：

```bash
RAW_STREAM_MAXLEN=100000
INFERRED_STREAM_MAXLEN=100000
DLQ_STREAM_MAXLEN=10000
REDIS_CLEANUP_EVERY=1m
REDIS_RETRY_DELAY=3s
```

含义：

- `RAW_STREAM_MAXLEN`：raw Stream 自动裁剪上限；
- `INFERRED_STREAM_MAXLEN`：inferred Stream 自动裁剪上限；
- `DLQ_STREAM_MAXLEN`：DLQ Stream 自动裁剪上限；
- `REDIS_CLEANUP_EVERY`：自动裁剪周期；
- `REDIS_RETRY_DELAY`：Redis 不可用时的重试间隔。

edge-infer 启动时不再强依赖 Redis。Redis 暂时不可用时，edge-infer 会持续等待并重试，不会因为 `PING` 失败直接退出。
