面向工业多设备日志的云边协同智能故障感知与一致性决策系统

背景：
在工业生产场景中，各类设备全天持续产生海量运行日志与异常告警信息。传统日志异常监测主要采用基于人工规则的告警机制，主要包含两类实现方式：一是日志分级告警，按照 FATAL、ERROR、WARN、INFO、DEBUG、TRACE 六个等级对日志严重程度进行划分，针对不同级别执行差异化处置策略；二是关键词匹配告警，通过预设异常关键词库，当日志文本命中指定关键词时触发告警。

基于规则的日志告警方案具备实现简单、逻辑清晰、可解释性强等优势，但高度依赖运维人员的领域经验与先验知识。受人工经验局限性影响，该类方法易出现规则覆盖不全导致的漏报、关键词误匹配引发的错报问题，难以适配复杂多变的工业日志场景。

近年来，AIOps 技术结合大语言模型被广泛应用于工业日志智能运维场景。依托强大的语义理解与推理能力，大模型能够深度挖掘日志背后的故障模式，实现精细化故障归因、智能生成运维处置方案，并提前识别潜在异常隐患，有效弥补了传统规则化告警方法的局限性。
然而，工业场景下设备日志体量庞大，若将全部原始数据直接上传至云端由大模型统一处理，将带来云端算力负载过高、网络带宽占用大、传输成本激增、数据隐私泄露风险、网络传输时延不可控等一系列问题，难以满足工业生产对实时性、安全性与经济性的多重约束。

针对上述痛点，我们计划设计并实现面向工业多设备日志的云边协同智能故障感知与一致性决策系统。由边缘端承担实时决策与初步过滤任务，快速判别告警优先级，区分需要立即触发停机或人工介入的紧急故障、可延迟处理的次要异常以及无效噪声日志；云端部署大模型，仅对边缘筛选后的高价值日志开展深度分析，实现精准根因定位与智能运维建议生成。在此架构下，如何实现工业日志的实时智能分诊，在保障运维响应时效性的前提下降低云端算力与传输带宽开销、实现整体运维体系降本增效，同时支持运维人员灵活配置与自定义日志路由规则，是云边协同式智能运维系统落地的核心诉求。

架构设计：
1. 总体思路：
本系统采用“云边协同”的实时日志分诊告警架构，其中“边缘”并非单一端设备，而是部署在设备近端的边缘集群（例如以厂房、车间为管理域的边缘Kubernetes小集群）。该边缘集群通常由 3～5 个master节点组成，具备一定算力与存储能力，能够承担实时日志接入、初步推理、路由决策与弱网自治等任务。系统的管控面（Control Plane）整体部署在边缘集群内，实现对本域设备日志的统一接入、分诊、协同推理调度、结果存储与可视化展示。云端大模型能力以外部API方式接入，由边缘侧按需调用。

2. 边缘管控面核心组件与部署方式
2.1 log-collector（日志采集，DaemonSet）
部署形态：DaemonSet，运行在边缘集群每个worker节点上。
职责：在节点侧主动采集设备/应用日志。采集路径可配置（例如指定目录、文件模式或容器日志来源），实现“就近接入、统一汇聚”。
输出：将原始日志事件写入边缘消息队列（采用 Redis 实现），作为后续推理与分诊的输入流。

2.2 edge-infer（边缘推理，Deployment）
部署形态：Deployment（可水平扩缩容）。
职责：消费日志队列中的原始日志事件，调用本地部署的边缘小模型（以API方式）完成快速推理与特征抽取。
输出：产生结构化推理结果，例如 severity、confidence、edge_pred、features 等，并写入redis streams中供router模块决策使用。
注：edge-infer与“本地小模型服务”可解耦：edge-infer负责业务编排与数据处理，小模型服务负责具体推理（便于替换模型或推理后端）。

2.3 router-controller（协同路由与弱网自治，Deployment）
部署形态：Deployment
职责：作为云边协同推理的核心调度模块，基于以下信息对每条日志事件进行动态路由决策：
边缘推理结果（如 confidence、severity、不确定性等）
系统态信息（网络 RTT/失败率、队列积压长度、云调用预算/限流等）
运维人员可配置的自定义路由策略（规则阈值/策略模型/白名单黑名单等）
决策输出：
LOCAL：边缘侧直接落库/告警（满足实时性与离线可用）
CLOUD：发送至云端进一步推理（用于低置信/高风险/罕见模式日志的深度分析）
弱网/断网处理：当网络不可用或云端不可达时，将需要上云的事件缓存至数据库（或缓存队列），待网络恢复后再进行补传，确保业务连续性与最终一致性。

2.4 server（后端服务，Deployment）
职责：
接收router判定需要上云的事件，调用云端大模型API进行日志理解、根因推断、处置建议生成，并将结果回写至数据库。
提供统一REST API（事件列表、告警详情、路由决策、云端结果、策略配置等）
聚合统计与报表（上云比例、P95时延、漏报/误报等指标）
与数据库交互，供前端查询与运维使用

2.5 web（前端，Deployment）
职责：提供可视化看板与运维交互界面，至少包括：
实时事件流（边缘判定/上云判定/最终结果）
告警列表与详情（解释、建议、时间线）
系统指标（上云比例、时延分布、队列积压、云调用次数/预算）
路由策略配置入口（阈值/规则/模式）

2.6 Redis（消息队列）
职责：承载边缘侧的日志流转（collector→infer→router→gateway等阶段）。
说明：使用Redis Streams 实现。

2.7 MySQL（数据库）
职责：
存储分诊结果（边缘推理与路由决策）
存储云端复核结果（解释/建议/根因）
存储弱网缓存的待上云事件（outbox/补传队列）
存储策略版本与审计（可选）

3. 业务处理流程
- log-collector采集到日志 → 写入 Redis（raw stream）
- edge-infer消费 raw stream → 小模型推理 → 输出结构化结果（infer stream）
- router消费 infer stream + 系统态 + 策略配置 → 决策 LOCAL/CLOUD/DEFFER
- LOCAL：写库/触发告警；CLOUD：发送到 cloud-gateway；DEFFER： 缓存到数据库中
- cloud-gateway调用云端API → 写回云端分析结果到数据库
- server/web展示端到端结果与指标；策略可在线调整并作用于router

event-driven的业务流程设计：
在我们的系统实现中，event是一次需要被系统做分诊决策的最小单位，我们通过event来实现路由、追踪以及落库的全流程。一个event对应的就是一行日志。

我们通过event来把一次分诊在各组件中经历的步骤串联起来，便于链路追踪。

event最初是被log-collector从真实世界中采集出来的，然后写入到redis stream中。设备不断地产生日志并落盘到本地的文件系统中，然后log-collector从文件系统中读取这些日志，按时间窗口聚合这些日志，构造一个event对象出来，写入到redis streams中。

event生命周期：
- CREATED：log-collector 聚合完成，事件创建
- INFERRED：edge-infer 完成边缘推理与特征抽取
- ROUTED：router-controller 做出路由决策（LOCAL/CLOUD/DEFER）
- CLOUD_PROCESSING：已发送到云端，等待云端结果
- CLOUD_DONE：云端推理完成并回写
- DONE：本地与云端（如有）结果齐全，事件闭环，落库
- FAILED：达到最大重试次数或不可恢复错误（进入DLQ）

一个Streams 拓扑
- cotriage.events.raw
写入者：log-collector
消费者组：edge-infer-g

- cotriage.events.inferred
写入者：edge-infer
消费者组：router-g

- cotriage.events.routed
写入者：router-controller
消费者组：server-g

- cotriage.events.cloud.req
写入者：router-controller（action=CLOUD 或 outbox补传）
消费者组：server-g

- cotriage.events.cloud.resp（可选）
写入者：cloud-gateway
消费者组：server-g 

- cotriage.events.dlq
写入者：任何阶段（超过重试阈值）
消费：server-g 

1. log-collecto生成的raw event
写入者：log-collector
消费者组：edge-infer-g
cotriage.events.raw:
- event_id: uuid，唯一标识一条event。
- trace_id: 暂不明确用途
- window:
    - start_ts: 开始时间戳
    - end_ts: 结束时间戳
    - duration_ms: 时间间隔，结束时间戳-开始时间戳
- source: 节点ID（这里的节点ID就等同于设备ID）
- logs[]: 窗口内日志行数组，控制上限，最多200行
- schema_version: 暂不明确用途

2. edge-infer进行本地推理
写入者：edge-infer
消费者组：router-g
cotriage.events.inferred:
- edge_pred：待补充具体含义
- severity：待补充具体含义
- confidence：待补充具体含义
- uncertainty：待补充具体含义
以上都是本地小模型的推理结果
- model_version： 记录模型版本
- latency_ms： 记录推理耗时

3. router-controller进行路由决策
写入者：router-controller
消费者组：server-g
cotriage.events.routed:
- action: LOCAL/CLOUD/DEFER
- policy_ref: 路由策略名称
- reason：命中的规则、阈值、预算拒绝原因等
- network_snapshot：rtt、失败率、云端健康状态
- budget_snapshot：令牌桶剩余、今日已用等
- latency_ms：路由决策耗时
action=CLOUD 的还需写入 triage.events.cloud.req；action=DEFER 的写入 outbox 表。

4. server进行云端推理
server-g 消费 cloud.req：
调用云API（超时、重试、幂等）
cotriage.events.cloud.resp:
- root_cause
- recommendation
- explanation
- cloud_confidence
- cloud.usage：
    - token数、
    - 费用
- latency_ms：云端调用耗时
- cloud.model：模型版本

幂等设计：
1. 幂等键
幂等主键统一使用 event_id
每个阶段落库表都对 event_id 做 unique
每个consumer处理成功后才 XACK；失败则留在pending并重试
2. 重试策略（Streams层）
每个consumer group定期扫描 PEL（pending entries list）
超过 idle_time 未ack 的消息重新 claim（XCLAIM / XAUTOCLAIM）
维护 attempt 计数（可以放在payload里或放在DB里）
超过最大attempt写入 triage.events.dlq
3. 云API幂等
server 在调用云API前先查 cloud_results 表是否已存在该 event_id。已存在则直接返回缓存结果，避免重复计费/重复调用

弱网/断网补传机制：
1. router-controller 判定网络不可用时。若策略要求上云但网络 down：将事件写入 cloud_outbox 表（status=PENDING）
同时在 routed event 标记 action=DEFER 和 defer_reason=network_down
2. outbox-replayer（router里的一个协程）
定期探测网络情况，扫描 PENDING 且到期的 outbox，推送到 triage.events.cloud.req
成功后更新 outbox 状态（SENT/DONE）