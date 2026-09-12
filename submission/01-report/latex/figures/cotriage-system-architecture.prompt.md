# CoTriage 企业级系统架构图生成记录

> 归档说明：本文件记录最初的 AI 生图过程。该位图已于 2026-08-08 被人工可编辑 SVG `cotriage-system-architecture.svg` 替代；同名 PDF 与 PNG 均由 SVG 派生，不再作为独立图源维护。

- 生成方式：OpenAI 内置 `image_gen`，先文本生成，再使用同一工具逐项修订。
- 生成日期：2026-07-30。
- 最终尺寸：1672 × 941 px。
- 使用位置：系统设计说明书、LaTeX 完整报告和排版样张。
- 事实来源：只读核对 `reports/report.md`、`submission/01-report/report.md` 和系统设计说明书；未改动队友原稿。
- 构图参考：仅吸收企业公开技术文档常见的分区、分层、正交连线、数据面与控制面分离原则，不复制任何厂商图形、Logo 或专有素材。

## 主提示词

```text
Use case: infographic-diagram
Asset type: enterprise-grade system architecture figure for an A4 Chinese engineering competition report and technical design document

Create a completely new, publication-ready architecture diagram titled exactly “CoTriage 云边协同智能故障感知与决策系统”.
It must communicate the complete engineering system, not only a simplified pipeline.
Use disciplined zoning, clear hierarchy, orthogonal connectors, consistent component cards, compact readable typography and strong information density.

Use a wide 16:9 landscape canvas with 4K-style sharpness, white or #F7F9FC background and flat 2D vector-like technical-document styling.
Use deep navy #163A5F for boundaries and headings, enterprise blue #2674D9 for control, cyan #11A8B5 for real-time event flow, safety orange #E97924 for RAISE/DEFER/retry/feedback, muted red #C74343 only for DLQ, and light gray #E9EEF3 for secondary state.
No 3D, no isometric perspective, no decorative illustration, no logo and no watermark.

Create four main columns:
1. “多节点日志源”
2. “边缘智能数据面”
3. “弱网可靠性与动态路由”
4. “云端深度诊断”
Create one full-width bottom band titled “在线配置与可观测控制面”.

Column 1:
Show “工业设备节点 A”, “生产服务节点 B” and “基础设施节点 N”.
Each node connects to its own “log-collector” card with sublabel “DaemonSet 就近采集”.
All collectors visibly merge into “统一事件化”, containing “event_id · ts · equipment · source · raw_log”.

Column 2:
Connect unified events to “Redis Streams / cotriage.events.raw”, then to “edge-infer 分层推理”.
Inside edge-infer, show one ordered pipeline:
“1 规则 RULE” → “2 模板 TEMPLATE” → “3 轻量模型 MiniLM ONNX” → “4 边缘小语言模型 EDGE SLM”.
Add “FALLBACK / UNKNOWN” and output strip “类别 · 严重度 · 置信度 · 不确定性 · 证据”.
Then connect to “Redis Streams / cotriage.events.inferred”.

Column 3:
Create “RAISE 动态路由中心”.
Connect sensor chips “网络状态”, “调用预算”, “故障风险”, “模型不确定性”, “队列压力” and “运维硬策略”.
Inside show “价值 / 增益 / 成本” and “动态阈值 + 可解释原因”.
Draw exactly three branches:
- “LOCAL” → “本地结果 / 现场系统”, note “核心仓库不内置执行器”.
- “CLOUD” → “cloud request” → cloud-gateway.
- “DEFER” → “Priority Outbox”.
Inside Outbox show “PENDING → SENDING → SENT”, “失败 → RETRY（指数退避）”, “超限 → DLQ” and “到期任务：按路由分数 → 截止时间 → 创建时间”.
Connect Outbox back to cloud request with orange dashed arrow “到期优先重放”.
Do not depict network recovery or cloud health as the replay trigger.

Column 4:
Inside “cloud-gateway”, show “相关上下文聚合”, “日志脱敏与 Token 裁剪” and “幂等请求与失败分类”.
Then connect to “OpenAI-compatible 云端模型” and “结构化诊断”.
Structured diagnosis contains “故障类别 / 根因”, “处置建议 / 解释” and “置信度 / Token / 时延”.
Connect to “MySQL cloud_feedback”, then “feedback processor”.
Connect feedback processor back to the RAISE area with orange dashed arrow “历史分歧与 UCB 状态更新（部分接入）”.
Add terminal output “cloud.resp” with note “当前无内容消费者”; do not connect it to Web.

Bottom control band:
Create “Server 配置中心” with cards “规则集”, “模板知识库”, “边缘分类器”, “边缘模型 Provider”, “云端模型 Provider” and bus “版本 / ETag 在线发布”.
Create central “Server REST / SSE”.
Create “Web 可观测大盘” with cards “实时事件流”, “异常态势”, “路由解释” and “Outbox 状态”.
Connect event, route and Outbox read/status sources to a shared navy bus, then to Server REST / SSE, then to Web.
Do not claim that cloud diagnosis detail is already displayed.
Add footer “Kubernetes 边缘集群 · Redis Streams 事件总线 · MySQL 状态与配置存储”.

Connector constraints are highest priority:
Every connector must attach to a real source and destination; no floating segments or empty endpoints.
Use orthogonal routing and shared buses.
Keep the primary event flow continuously left-to-right.
LOCAL, CLOUD and DEFER originate from the same RAISE core.
Keep control lines below the data plane and feedback above it.
Do not route lines through text.
All quoted labels must be verbatim, with no fake metrics or experimental numbers.
```

## 定向修订链

主图生成后，仍使用 `image_gen` 按单一目标逐步完成以下修订：

1. 删除 RAISE 绕过 `cloud request` 的直达线，闭合 `RAISE → CLOUD → cloud request → cloud-gateway`；
2. 将 `cloud.resp` 接为 cloud-gateway 的终端输出，但不连接 Web；
3. 清除误连到 inferred Stream 和 `cloud_feedback` 的配置虚线；
4. 修正采集链为“统一事件化 → raw Stream → edge-infer”；
5. 删除 `feedback processor → Web` 的错误直连；
6. 补齐 `MySQL cloud_feedback → feedback processor`；
7. 将“边缘小模型”改为“边缘小语言模型”；
8. 将 LOCAL 边界改为“核心仓库不内置执行器”；
9. 将事件、路由与 Outbox 读取总线接入 `Server REST / SSE`。

## 人工验收清单

- 八项需求均有独立视觉落点；
- raw、inferred、CLOUD、DEFER、反馈和可观测读取链路均连续；
- `cloud.resp` 不连接 Web，反馈仅标记为“部分接入”；
- Outbox 表达的是到期优先重放，不把网络恢复或云健康误画成触发条件；
- 无实验数值、未经核验的效果结论、赛事 Logo 或敏感环境信息。
