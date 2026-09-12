# CoTriage 系统运行截图清单

> 整理日期：2026-08-09
>
> 来源：开发队友提供的系统运行截图
>
> 用途：作品报告、软件工程交付材料和参赛 PPT

本目录保留收到的原始截图内容，仅统一文件名并记录用途。截图中展示的即时数值只能证明当时系统状态，不能替代结构化实验数据，也不能直接作为完整实验结论。

## 1. 已收到截图

| 编号 | 文件 | 识别内容 | 当前结论 | 使用建议 |
| --- | --- | --- | --- | --- |
| S-00 | [`s-00-application-homepage.png`](s-00-application-homepage.png) | CoTriage 应用首页；展示项目品牌、云边协同定位、实时驾驶舱与管理中心入口，以及边缘实时推理、动态路由决策、弱网补偿、云端深度诊断等能力概览 | 可用 | PPT 作品亮相页、报告软件界面概览和用户说明书入口页的首选图片；首页中的“毫秒级响应”“可靠运行”等是产品文案，不能替代正式性能与可靠性实验 |
| S-01 | [`s-01-realtime-dashboard.png`](s-01-realtime-dashboard.png) | 实时日志分诊大盘；显示实时流连接、近一分钟接入/推理、P95 时延、异常比例、累计推理、RAISE 路由等 | 可用 | 报告系统实现和 PPT 总览的首选截图；当前画面中 Cloud 路由和 Defer 均为 0，不能用它证明云边协同或弱网延期效果 |
| S-02a | [`s-02-device-management.png`](s-02-device-management.png) | 设备管理；4 个工作节点均为 `Ready`，设备类型包括 Apache、Linux、Hadoop、ZooKeeper | 有条件可用 | 与 S-02b 组合证明多设备登记和持续接入；界面中的 `AppacheServer` 存在拼写问题，建议后续修正 |
| S-02b | [`s-02b-realtime-event-stream.png`](s-02b-realtime-event-stream.png) | 实时事件流；多个设备连续产生事件，展示分类、严重度、置信度、推理路径、诊断摘要和时延 | 可用 | 可强化多设备持续接入和分层推理说明；当前可见事件均为 `NORMAL` 且走 `CACHE`，不用于证明异常诊断或云端路由 |
| S-02c | [`s-02c-inference-trend-and-distribution.png`](s-02c-inference-trend-and-distribution.png) | 实时感知与推理统计；展示近 10 分钟完成推理/异常趋势、CACHE/MODEL/RULE 推理路径分布和 NORMAL/ABNORMAL 分类分布 | 可用 | 适合报告和 PPT 的运行效果页；图中计数是单次仪表盘快照，只用于说明系统运行构成，不能替代结构化实验结果 |
| S-02d | [`s-02d-realtime-event-stream-cache-model.png`](s-02d-realtime-event-stream-cache-model.png) | 实时事件流；同屏展示多个设备的 CACHE 与 MODEL 推理路径、置信度、诊断摘要和单事件时延 | 可用 | 比 S-02b 更适合说明分层推理路径；时延是单事件即时值而非 P95/均值，且界面仍有 `AppacheServer` 拼写问题 |
| S-03 | [`s-03-anomaly-trend-analysis-redacted.png`](s-03-anomaly-trend-analysis-redacted.png) | 异常态势分析；展示风险等级、异常聚合、优先队列、RULE/MODEL 推理路径、严重度与置信度 | 脱敏副本可用 | 原始日志中的真实 Tailscale 地址已遮挡；适合说明边缘异常识别和优先级排序，页面即时数量不作为实验统计结果 |
| S-03b | [`s-03b-anomaly-diagnosis-center-overview.png`](s-03b-anomaly-diagnosis-center-overview.png) | 异常诊断中心总览；同屏展示异常趋势、风险聚合、优先队列、云端根因和实时事件 | 有条件可用 | 适合作为 PPT 异常诊断流程总览；属于 Web 界面证据，不能证明诊断结果已持久化，且 `UNKNOWN` 与云端解释中 `pred: ABNORMAL` 的语义需开发队友确认 |
| S-04a-1 | [`s-04a-raise-routing-status.png`](s-04a-raise-routing-status.png) | RAISE 路由状态早期截图；展示 LOCAL/CLOUD 累计数量、上云压力、路由门槛、最近去向、云端健康和 Outbox 状态 | 有条件可用 | 可用于历史追溯；对外材料优先使用信息更完整的 S-04a-2 |
| S-04a-2 | [`s-04a-raise-routing-status-overview.png`](s-04a-raise-routing-status-overview.png) | RAISE 路由与可靠传输状态新版总览；展示 LOCAL/CLOUD 累计数量、云端压力与门槛、最近路由、云端健康、队列压力、预算及 Outbox 状态 | 有条件可用 | 当前 Outbox 为 0、最近路由为 LOCAL；可证明路由器在线和全局运行状态，但没有单事件评分、具体路由原因或 DEFER/弱网积压示例 |
| S-05a | [`s-05a-cloud-root-cause-diagnosis.png`](s-05a-cloud-root-cause-diagnosis.png) | 云端根因诊断；展示模型、根因、处置建议、结构化诊断解释、时延、置信度和 Token 数 | 有条件可用 | 顶部 `UNKNOWN` 与解释中的 `pred: ABNORMAL` 可能分别表示边缘初判和云端复判，需开发队友确认语义后再确定图注 |
| S-08a | [`s-08a-rule-management.png`](s-08a-rule-management.png) | 规则管理；展示异常、安全、网络、磁盘和基础设施规则及置信度、优先级、启用状态 | 可用 | 用于用户说明书、系统可配置性和规则优先推理说明 |
| S-08b | [`s-08b-template-knowledge-base.png`](s-08b-template-knowledge-base.png) | 模板知识库；展示一条来源为 `abnormal_cluster` 的模板 | 有条件可用 | 当前模板状态为 `draft`，只能证明管理界面和模板草稿存在，不能表述为已发布生效 |
| S-09a | [`s-09a-minilm-model-management.png`](s-09a-minilm-model-management.png) | 轻量分类模型管理；MiniLM、`bgl-minilm`、阈值 5%、已发布且生效 | 可用 | 用于说明轻量分类模型可配置、可发布和可生效 |
| S-09b | [`s-09b-edge-llm-provider-management-redacted.png`](s-09b-edge-llm-provider-management-redacted.png) | 边缘小 LLM；Qwen1.5-1.8B、llama.cpp、已发布且生效 | 脱敏副本可用 | Endpoint 中的真实 Tailscale IP 已使用不透明灰块遮挡；用于展示配置能力，不作为原始实验数据 |
| S-09c | [`s-09c-cloud-provider-management.png`](s-09c-cloud-provider-management.png) | 云端模型管理；DeepSeek Provider、模型配置、密钥掩码、已发布且生效 | 可用 | 可证明云端 Provider 已配置；不得点击显示明文密钥后截图 |
| S-11 | [`s-11-competition-metrics-dashboard.png`](s-11-competition-metrics-dashboard.png) | 赛题指标汇总看板；覆盖实时性、效果与一致性、资源与通信、稳定性与扩展性，并展示运行窗口、P95 时延、完成率、云边配对、一致率、通信量、Token、Outbox 和设备数 | 有条件可用 | 比赛指标展示和 PPT 指标总览的首选截图；页面混合 `recent-2000-events` 与 `cumulative-db` 口径，正式报告引用数值前必须取得计算公式、分母、时间窗口和原始数据 |

## 2. 原始文件名映射

| 原始文件名 | 整理后文件名 |
| --- | --- |
| `微信图片_20260809133027_94_171.png` | `s-00-application-homepage.png` |
| `微信图片_20260808192400_49_171.png` | `s-01-realtime-dashboard.png` |
| `微信图片_20260808191119_43_171.png` | `s-02-device-management.png` |
| `微信图片_20260808200610_51_171.png` | `s-02b-realtime-event-stream.png` |
| `微信图片_20260809131325_83_171.png` | `s-02c-inference-trend-and-distribution.png` |
| `微信图片_20260809131356_84_171.png` | `s-02d-realtime-event-stream-cache-model.png` |
| `微信图片_20260808200546_50_171.png` | `s-03-anomaly-trend-analysis-private-ip-do-not-submit.png` |
| `微信图片_20260809131249_82_171.png` | `s-03b-anomaly-diagnosis-center-overview.png` |
| `微信图片_20260809123743_77_171.png` | `s-04a-raise-routing-status.png` |
| `微信图片_20260809131230_81_171.png` | `s-04a-raise-routing-status-overview.png` |
| `微信图片_20260809123740_76_171.png` | `s-05a-cloud-root-cause-diagnosis.png` |
| `微信图片_20260808191143_44_171.png` | `s-08a-rule-management.png` |
| `微信图片_20260808191248_45_171.png` | `s-08b-template-knowledge-base.png` |
| `微信图片_20260808191311_46_171.png` | `s-09a-minilm-model-management.png` |
| `微信图片_20260808191327_47_171.png` | `s-09b-edge-llm-provider-private-ip-do-not-submit.png` |
| `微信图片_20260808191412_48_171.png` | `s-09c-cloud-provider-management.png` |
| `微信图片_20260809131129_78_171.png` | `s-11-competition-metrics-dashboard.png` |

S-03 和 S-09b 分别生成展示用脱敏副本 `s-03-anomaly-trend-analysis-redacted.png`、`s-09b-edge-llm-provider-management-redacted.png`。含真实地址的高分辨率原图仅在本机保留，并通过 `.gitignore` 禁止提交。

S-03 中事件封装时间为 2026 年，但原始日志正文包含历史数据集时间。报告中应将其描述为历史日志样本的当前回放与分析，不能写成 2026 年真实生产故障。

S-00 是产品入口和视觉展示证据，不是系统运行状态证据。顶部绿色状态点和“云边协同系统”字样只能说明首页呈现了该状态组件；系统是否真实在线，应结合 S-01、S-04a-2、服务健康记录或部署状态证明。

### S-11 指标读图与引用边界

| 指标类别 | 截图显示值 | 当前可以说明 | 正式引用前仍需确认 |
| --- | --- | --- | --- |
| 运行窗口 | 近一分钟接入 238 条；推理完成率 100%；异常日志比例 0% | 截图时系统持续接入且当前窗口内推理已完成 | `238` 的采集起止点、去重方式；完成率的分母；0% 是否表示当前窗口无异常样本 |
| 实时性 | 边缘处理 P95 345 ms；端到端 P95 690 ms；相对云端改善 76.4% | 页面已经实现边缘与端到端长尾时延展示 | 两类时延的起止时间、样本量和统计窗口；76.4% 的全云基线值及计算公式 |
| 效果与一致性 | 云边配对样本 12；云边一致率 100%；冲突解决率 0% | 已记录少量云边配对样本，当前显示判断一致 | 12 个样本的构造与标签；一致率分母；冲突数是否为 0。若没有冲突，0% 不应解释为冲突解决失败 |
| 资源与通信 | 云端卸载率 100%；估算减少上传 299.9 KiB；累计云端 Token 35919 | 页面可以汇总通信节省和云端资源消耗 | “云端卸载率”的定义与命名是否准确；全云基线、上传量统计层级、Token 来源和累计时间范围 |
| 稳定性 | 完成率 100%；Outbox 成功率 0%；Defer 队列 0 条；降级比例 0% | 当前画面没有积压或降级事件 | Outbox 应重放事件数；若分母为 0，应展示为 N/A 而不是 0%。该图不能单独证明弱网补传成功或失败 |
| 扩展性 | 设备/来源 4/0 | 当前系统登记或观察到 4 个设备 | “来源”的业务含义及为何为 0；不能据此推断多节点扩展上限 |

S-11 可以作为比赛指标页面的运行截图，但不能单独替代正式实验结果。开发队友仍需提供 CSV/JSON、统计脚本或数据库导出，使截图中的每个指标都能对应到可计算的分子、分母、时间窗口和样本集合。

## 3. 仍需向开发队友补充

| 编号 | 缺失截图 | 最低要求 |
| --- | --- | --- |
| S-04b | 单事件 RAISE 路由解释 | `LOCAL/CLOUD/DEFER`、评分、阈值和具体路由原因；当前 S-04a-1/2 均仅为全局状态 |
| S-05b | 云端诊断持久化证据 | MySQL `cloud_feedback`、Redis `cloud.resp` 或服务日志中的根因、建议与置信度；当前 S-05a 仅为 Web 展示 |
| S-06 | 弱网期间状态 | 网络异常、`DEFER` 数量及 Outbox 变化 |
| S-07 | 链路恢复状态 | Outbox 重试、积压变化及云端请求处理结果 |
| S-10 | Kubernetes 运行状态 | 核心 Pod、工作节点和服务健康状态 |

## 4. 使用优先级

- **PPT 封面后的作品亮相页**：优先使用 S-00；建议保留完整画面，不与大量文字或其他截图拼接。
- **PPT 系统运行总览页**：优先使用 S-01。
- **PPT 功能与效果页**：优先使用 S-02c 和 S-03b；需要展示事件明细时补充 S-02d 或已脱敏的 S-03。
- **PPT 云边协同页**：可组合使用 S-04a-2 和 S-05a，但必须保留上述限制说明。
- **PPT 比赛指标页**：优先使用 S-11；在口径未确认前，标题使用“运行窗口指标展示”，不要写成“正式实验最终结果”。
- **报告系统实现章节**：使用 S-01、S-02a、S-02c、S-02d、S-03、S-03b、S-04a-2、S-05a、S-08a、S-09a、S-09c。
- **报告软件界面概览和用户使用说明书入口**：使用 S-00；进入功能说明后再切换到对应运行或管理截图。
- **报告实验结果章节**：S-11 只作界面与指标采集能力证明；必须以结构化实验数据复核后再填写正式结果表。
- **用户使用说明书**：可使用已脱敏的 S-09b 和其他截图，并在 S-08b 图注中注明“草稿状态”。
- **禁止使用**：文件名含 `private-ip-do-not-submit` 的两张原图；只使用对应脱敏副本。

所有对外截图都应再次检查 API Key、真实 Tailscale 地址、个人通知、聊天窗口和无关桌面内容。
