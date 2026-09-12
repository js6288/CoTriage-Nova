# cloud-gateway 开发进展

更新时间：2026-07-19

## 当前状态

第一轮设计范围的核心链路已实现，代码位于独立 `cloud-gateway/` Go module；
server 控制面也已补齐。仓库实际使用 `progress_doc` 目录，因此开发进展记录在
本文件（用户请求中的 `process_doc` 按仓库既有目录名解释）。

## 已完成

- server 新增独立 `cloud_providers` 数据模型、AutoMigrate 和 SQL migration；
- server 新增 cloud provider 创建、列表、更新、软删除、激活接口；
- Web 管理台“模型管理”新增独立“云端大模型”页签，支持 DeepSeek、
  Qwen、OpenAI-compatible 和 custom HTTP provider 的创建、编辑、启用与删除；
- Web 支持用户直接录入真实 API Key，并通过密码输入框的眼睛按钮显示/隐藏；
- server 直接保存并通过配置接口管理 API Key，适配当前 Demo 使用边界；
- Web 的新增、编辑和列表均使用带眼睛按钮的密码输入框，可在明文与星号之间切换；
- server cloud provider 接口补充单项查询、统一分页响应字段及
  `api_key_env`/temperature/top_p 参数校验；
- server 新增 `GET /api/v1/cloud-config`，支持 ETag/304，且只下发
  `api_key_env`，不保存或返回明文 API key；
- 新增独立 `cloud-gateway` 服务、Dockerfile、Makefile 和运行文档；
- `.vscode/launch.json` 新增 `cloud-gateway` Go Debug 配置，可在 VS Code
  中连接本地端口转发后的 Redis、MySQL 和本地 Server 直接调试；
- Redis consumer group 初始化或 `XREADGROUP` 遇到临时网络错误时不再退出，
  而是按 `CLOUD_GATEWAY_REDIS_RETRY_DELAY` 持续重试；Redis 客户端读取超时
  可通过 `CLOUD_GATEWAY_REDIS_READ_TIMEOUT` 配置，默认 10 秒；
- Redis consumer group 消费 `cotriage.events.cloud.req`；
- 请求字段和 `payload_json` 校验，非法/不可重试消息写入 DLQ；
- 从 `cotriage.events.inferred` 读取时间窗口日志，按时间、模板、source、
  equipment、边缘分类、严重度计算相关性并按 token 预算裁剪；
- prompt 构造前进行 IPv4、邮箱、password/token/key、长十六进制串脱敏；
- OpenAI-compatible Chat Completions 调用，API key 按 provider 的
  `api_key_env` 从 cloud-gateway 环境变量读取；
- provider timeout、429、5xx 和配置缺失错误分类；
- JSON 直接解析、包裹文本中的首个 JSON object 提取、非结构化响应降级；
- CLOUD-DIAG reward、云边分歧、可执行信息、高风险确认计算；
- `cloud_feedback` 幂等写入，固定写入 `processed_by_router=false`；
- DB 成功但 response 发布失败时不 ACK；重试会读取已保存的
  `feedback_json` 并补发 response；
- 发布 `cotriage.events.cloud.resp` 后 ACK；
- 增加结构化输出修复、降级、脱敏和 reward 单元测试。

## 验证记录

- server：`go test ./...` 通过；
- cloud-gateway：`go test ./...` 通过；
- 本机全局 `GOROOT=/usr/local/go` 指向 Go 1.25.3，而 PATH 中 Go 为
  1.26.1；验证时已显式使用 Homebrew Go 1.26.1 的 `libexec` GOROOT，
  避免工具链版本混用。

## 后续增强（不属于第一轮主链路）

- `XAUTOCLAIM` pending 消息回收与按 attempt 上限进入 DLQ；
- provider health probe、QPS token-bucket 和指标/heartbeat；
- transactional cloud response outbox，彻底避免响应流重复补发；
- 多 provider 自动切换及 `cloud_invocations` 审计表。

## 日期：2026年08月08日（结构化云端诊断展示闭环）

### 已完成

- server 新增 `GET /api/v1/router/cloud-feedback`，分页读取 `cloud_feedback` 中的云端结构化诊断；
- API 返回云端分类、根因、处置建议、解释、置信度、模型、耗时、token、成本与 reward 信息；
- Web 大盘按 `event_id` 合并实时 inferred event 与 cloud feedback；
- 实时事件“诊断结果”优先展示云端建议、根因或解释；
- 事件展开区新增“云端结构化诊断”，展示分类、置信度、模型、耗时、根因、建议和解释；
- Cloud Gateway 已有的结构化诊断生成与 `cloud_feedback` 持久化逻辑保持不变。

### 验证结果

- server 使用 Homebrew Go 1.26.1：`go test ./...` 通过；
- web：`npm run build` 通过；
- 默认 Go 环境仍存在已知的 Go 1.25.3/1.26.1 工具链混用问题。
