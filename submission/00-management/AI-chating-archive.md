Q: 目前系统已经基本上开发完了，实验数据、大模型API、工作节点在另一位队员的服务器上运行，我只需要通过Tailscale连接上队员的服务器就可以运行，或者说我甚至不需要运行这个项目，我目前现在的任务就是结合现有文档和撰写需要提交的材料和制作参赛PPT, 关于要提交的材料由于 [XH-202606_面向云边协同场景的分布式人工智能感知与决策关键技术研究.pdf](E:/AI_project/Unveil-the List-Take-the-lead/云边协调参赛文档/XH-202606_面向云边协同场景的分布式人工智能感知与决策关键技术研究.pdf) 比赛文件写的模糊，就先按照软件工程交付所需的材料来安排比如交付说明书、用户说明书、设计文档等，还有根据现有文档完善论文项目报告 [report.md](E:/AI_project/CoTriage-dev/reports/report.md) ，找一个美观的latex模板，输出成pdf。对此你有什么建议和方案吗

A: 建议把接下来的工作定义为“参赛交付工程”，而不是继续做研发。核心原则是：一份事实源，多种交付形态。系统架构、算法说明、实验数据、截图和指标只维护一套，再分别编排成作品报告、说明书、PPT和视频。

## 一、建议的提交材料体系

赛题明确要求的是作品报告、运行效果视频、代码与数据、其他佐证材料。软件工程文档属于“其他佐证材料”，应做精而不是做全。

### A. 必交材料

| 材料 | 建议形式 | 作用 |
|---|---|---|
| 作品报告 | PDF，约30–40页 | 最核心的评分材料 |
| 运行效果视频 | MP4，建议6–10分钟 | 证明系统真实可运行 |
| 源代码 | 脱敏后的压缩包或仓库快照 | 可执行性和完整性 |
| 数据与实验结果 | 数据说明、结果CSV、图表、配置 | 支撑所有量化结论 |
| 参赛申报表 | 盖章PDF | 行政要求 |

### B. 建议附带的软件工程材料

我建议控制在5份，不要把所有 `design_doc` 和 `progress_doc` 原样打包：

1. 《项目交付说明书》
2. 《用户使用说明书》
3. 《系统设计说明书》
4. 《部署与运维说明书》
5. 《实验与测试报告》

可以再附一份较短的《API与数据接口说明》，也可以作为系统设计说明书的附录。

各文档边界如下：

- 交付说明书：交付物清单、版本、依赖、目录、安装方式、验收项、已知限制、第三方组件。
- 用户说明书：评委如何进入系统、查看大盘、配置规则/模型、观察云边路由、模拟弱网、查看诊断结果。
- 系统设计说明书：总体架构、模块划分、事件生命周期、Redis Streams、数据库、RAISE、可靠性和安全设计。
- 部署运维说明书：Tailscale、Kubernetes、服务启动、健康检查、故障排查、停止与恢复。
- 实验测试报告：实验环境、基线、数据集、指标定义、原始结果、统计方法、弱网实验、一致性实验和消融实验。

现有文档资源其实很充足：

- [总体设计](/E:/AI_project/CoTriage-dev/design_doc/main.md)
- [RAISE算法设计](/E:/AI_project/CoTriage-dev/design_doc/router_design.md)
- [云端网关设计](/E:/AI_project/CoTriage-dev/design_doc/cloud-gateway.md)
- [Web设计](/E:/AI_project/CoTriage-dev/design_doc/web_design.md)
- [运行SOP](/E:/AI_project/CoTriage-dev/sop/router-controller运行SOP.md)
- [当前报告](/E:/AI_project/CoTriage-dev/reports/report.md)

真正缺少的是统一编排、版本信息、验收口径和实验佐证，不是重新从头写五套文档。

`progress_doc/` 更适合保留为内部研发记录，不建议原样提交。它包含开发过程、旧状态和待办项，容易与最终系统产生矛盾。`sop/` 也应整理成公开版，隐藏真实IP、API Key、Tailscale节点信息和内部路径。

## 二、建立“实验数据交接包”

你确实不需要长期运行系统，但不能只从队友那里拿几张截图。建议让负责服务器的队员一次性提供一个标准化实验包：

```text
experiment_bundle/
├── manifest.yaml              # 实验名称、时间、代码提交、机器和软件版本
├── configs/                   # RAISE参数、模型配置、网络配置
├── raw/                       # 每轮实验原始CSV/JSON
├── summary/                   # 汇总结果CSV
├── logs/                      # 关键运行日志和错误日志
├── screenshots/               # 大盘、K8s、模型服务、弱网恢复截图
├── figures/                   # 队友已有图表
├── commands.md                # 实验执行命令
└── checksums.txt              # 文件校验值
```

`manifest.yaml` 至少包含：

- Git commit；
- 边缘节点数量、CPU、内存、GPU；
- Kubernetes、Redis、MySQL版本；
- MiniLM和边缘LLM名称、量化方式、模型大小；
- 云端API模型；
- RTT、丢包、带宽和断网设置；
- 数据集划分、随机种子、重复次数；
- 每个结果文件对应的实验名称。

这样你只需在必要时通过Tailscale补录演示视频或截图，不需要为了写报告反复重新跑项目。

## 三、`report.md` 的完善方案

当前报告的整体结构已经很好，特别是架构、RAISE、实验设计和赛题指标对照都已经存在。主要工作不是重写，而是完成以下升级。

### 1. 从“设计说明”变成“有结论的作品报告”

目前最大缺口集中在第6、7章：

- 填写真实实验环境；
- 用实际数据替换所有“待实验”；
- 每张表后增加分析，而不是只给数字；
- 区分平均值、标准差和重复次数；
- 明确哪些是实测、哪些是推导、哪些是局限；
- 删除文末所有内部检查标记。

### 2. 补充必要图件

建议报告最终包含约12–16张核心图：

1. 云边端总体架构图；
2. 一条事件的生命周期；
3. Redis Streams消息拓扑；
4. 边缘分层推理流程；
5. RAISE特征、成本和决策框架；
6. `LOCAL/CLOUD/DEFER`决策流程；
7. 弱网Outbox恢复时间轴；
8. 云端反馈闭环；
9. 两类应用场景拓扑；
10. Web实时大盘截图；
11. 模型F1—时延—内存对比；
12. 不同方法端到端效果对比；
13. 上云比例/通信量对比；
14. 弱网业务保持率；
15. 多节点一致性结果；
16. RAISE消融实验。

### 3. 增加文献与相关工作

目前只有3条参考资料，作为竞赛作品报告偏少。建议增加到15–25条，重点覆盖：

- 云边协同推理；
- AIOps与日志异常检测；
- 不确定性校准；
- 动态任务卸载；
- Outbox与可靠消息处理；
- BGL、Loghub等数据集；
- 边缘模型压缩和ONNX部署。

正式撰写时应查原始论文和官方技术文档，避免只引用博客。

### 4. Markdown与LaTeX的关系

建议先把 [report.md](/E:/AI_project/CoTriage-dev/reports/report.md) 的内容和数据补完整，完成一次“内容冻结”；然后迁移成模块化LaTeX。从迁移完成开始，以LaTeX为最终报告唯一事实源，避免Markdown和LaTeX长期双向修改导致漂移。

推荐结构：

```text
reports/latex/
├── main.tex
├── elegantbook.cls
├── frontmatter/
│   ├── cover.tex
│   └── abstract.tex
├── chapters/
│   ├── 01-background.tex
│   ├── 02-requirements.tex
│   ├── 03-architecture.tex
│   ├── 04-edge-infer.tex
│   ├── 05-raise.tex
│   ├── 06-reliability.tex
│   ├── 07-experiments.tex
│   └── 08-conclusion.tex
├── figures/
├── tables/
├── appendices/
├── references.bib
└── README.md
```

最终PDF放在：

```text
output/pdf/CoTriage_挑战杯作品报告.pdf
```

## 四、LaTeX模板建议

我建议采用 [ElegantBook 4.7](https://ctan.org/pkg/elegantbook?lang=en) 作为基础，再做克制的比赛报告定制。它适合带封面、目录、多章、公式、算法、图表和附录的中文长篇技术报告；当前CTAN版本为4.7，许可证为LPPL 1.3c。

具体建议：

- 使用XeLaTeX编译；Overleaf也建议中文文档使用XeLaTeX或LuaLaTeX及`ctex`类。[Overleaf中文排版指南](https://www.overleaf.com/learn/latex/Chinese)
- 将`elegantbook.cls`随项目保存，避免本机与Overleaf版本不一致；
- 采用A4单面、蓝青色科技主题；
- 自定义挑战杯作品封面，正文保持正式、低装饰；
- 使用`booktabs`、`longtable`、`algorithm2e`、`listings`、`tcolorbox`；
- 参考文献使用GB/T 7714格式；
- 架构图和实验图统一输出为PDF/SVG矢量图；
- 编译完成后逐页渲染检查字体、表格、图注、页眉和断页。

备选是Overleaf的[中文单栏技术报告模板](https://www.overleaf.com/latex/templates/zhong-wen-ji-zhu-bao-gao-latexmo-ban-dan-lan-cjc-xelatex/tcnttxfsqykx)，但它更像期刊论文；CoTriage包含大量系统架构、实验和附录，ElegantBook更合适。

## 五、参赛PPT建议

PPT不要用Beamer。比赛演示需要强视觉、可编辑图形、产品截图和数据图表，更适合生成16:9的原生PPTX。

建议15–18页：

1. 封面；
2. 行业痛点；
3. 赛题挑战与核心目标；
4. CoTriage一句话方案；
5. 总体云边架构；
6. 边缘分层推理；
7. RAISE核心思想；
8. `LOCAL/CLOUD/DEFER`决策；
9. 弱网自治；
10. 云端反馈闭环；
11. 系统实现与部署；
12. 两类实验场景；
13. 核心效果对比；
14. 弱网与资源效率；
15. 多节点一致性与消融；
16. 赛题指标对照；
17. 创新性与应用价值；
18. 总结。

视觉上建议使用“工业控制台”风格：深海军蓝、青色数据流、安全橙告警色；避免大面积AI渐变和密集卡片。每页标题直接写结论，例如“RAISE在保持高风险召回的同时减少X%的上云请求”，不要只写“实验结果”。

等报告数据和图表冻结后，再进入`ppt-master`正式流程。届时会先确认画布、页数、受众、风格、配色、图标、字体和图片使用八项，再生成逐页SVG、演讲备注和PPTX。

## 六、推荐执行顺序

1. 建立提交材料目录和总清单。
2. 冻结交付版本、系统名称、团队信息和术语。
3. 向队友发出标准化实验数据交接清单。
4. 完善`report.md`内容，填充实验结果。
5. 制作统一架构图和实验图。
6. 迁移到ElegantBook并生成最终PDF。
7. 从最终报告提炼交付说明、用户手册和设计说明。
8. 制作PPT及逐页演讲稿。
9. 根据PPT编写视频脚本并录制。
10. 最后做脱敏、链接、许可证、文件名和压缩包验收。

如果现在开始落地，我建议第一步先在仓库中建立一份“参赛材料总计划与目录”，明确每份材料的负责人、来源、状态和缺失证据；然后优先完善`report.md`，暂时不要直接开始做PPT。报告一旦冻结，PPT、视频和说明书都会快很多。