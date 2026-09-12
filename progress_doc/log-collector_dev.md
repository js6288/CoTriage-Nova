# log-collector 组件开发记录

## 日期：2026年06月22日

### 内容

1. 排查实时大盘卡顿与 Redis 内存增长时，发现 `cotriage.events.raw` 未设置长度上限，实测已累计约 294488 条消息。
2. 新增环境变量 `REDIS_STREAM_MAXLEN`，默认值为 `100000`。
3. log-collector 执行 `XADD` 时增加近似 `MAXLEN`，由 Redis 在写入过程中增量裁剪，避免 raw Stream 无限增长。
4. 对无效的 `REDIS_STREAM_MAXLEN` 配置记录告警并回退到默认值。
5. 启动日志增加 Stream 最大长度，便于部署审计和运行排查。
6. VS Code `log-collector` 调试配置增加 `REDIS_STREAM_MAXLEN=100000`。
7. Kubernetes DaemonSet 增加同名环境变量，确保集群部署与本地调试行为一致。

### 验证结果

- `go test ./...`：通过。
- Go 代码已通过 `gofmt`。
- 未主动执行破坏性的 `XTRIM` 清理已有数据；组件重新启动并产生新消息后，将通过 `XADD MAXLEN ~` 逐步裁剪到近似上限。

### 后续事项

1. 根据日志速率和所需回溯时长调整 100000 条默认上限。
2. 增加 `REDIS_STREAM_MAXLEN` 配置单元测试。
3. 将近一分钟接入量维护为独立计数器，减少 server 对 raw Stream 的范围扫描。
