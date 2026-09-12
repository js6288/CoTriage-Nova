# CoTriage 参赛材料工作区

本目录用于组织挑战杯揭榜挂帅专项赛题 `XH-202606` 的当前材料工作。现阶段个人责任范围仅包含三类成果：

1. 编写并排版作品报告；
2. 编写软件工程交付材料；
3. 制作参赛 PPT。

实验数据和运行截图由负责开发与实验的队友提供，作为报告、工程文档和 PPT 的外部输入。当前不要求本材料负责人重新部署系统、运行完整实验或维护队友服务器。

## 责任边界

| 分类 | 内容 | 当前处理方式 |
| --- | --- | --- |
| 本人负责 | 作品报告 Markdown、LaTeX 工程与 PDF | 编写、核对、排版和逐页检查 |
| 本人负责 | 软件工程交付材料 | 从现有设计、代码、运行说明和实验输入中整理 |
| 本人负责 | 参赛 PPTX/PDF | 在报告内容与实验结论冻结后制作 |
| 外部输入 | 实验数据、环境信息、运行日志和系统截图 | 由开发与实验负责人按交接格式提供 |
| 暂不处理 | 参赛申报信息及相关表格 | 不建立、不填写、不纳入当前里程碑 |
| 暂不处理 | 运行效果视频 | 暂停脚本、录制、剪辑和成片工作 |
| 不在当前范围 | 实验重跑、服务器运维、最终投递和行政打包 | 仅在责任人另行明确后调整 |

## 目录结构

```text
submission/
├── 00-management/       # 当前任务基线、总清单和统一术语
├── 01-report/           # 作品报告 LaTeX 源文件、图表和最终 PDF
├── 02-engineering-docs/ # 交付、用户、设计、部署、测试等说明书
├── 03-evidence/         # 开发队友提供的实验数据与运行截图入口
└── 04-presentation/     # 参赛 PPTX、PDF 和配套图件
```

## 工作入口

- 当前材料清单：[`00-management/materials-checklist.md`](00-management/materials-checklist.md)
- 当前责任基线：[`00-management/materials-baseline.md`](00-management/materials-baseline.md)
- 统一术语：[`00-management/terminology.md`](00-management/terminology.md)
- 实验交接格式：[`03-evidence/experiment-handoff-template.md`](03-evidence/experiment-handoff-template.md)
- 开发队友材料提供清单：[`03-evidence/developer-evidence-request.md`](03-evidence/developer-evidence-request.md)
- 第二条线作品报告事实源：[`01-report/report.md`](01-report/report.md)
- 队友原始报告（只读参考）：[`../reports/report.md`](../reports/report.md)
- 总体系统设计：[`../design_doc/main.md`](../design_doc/main.md)
- RAISE 算法设计：[`../design_doc/router_design.md`](../design_doc/router_design.md)

## 工作原则

1. **一项结论对应一份证据。** 报告、工程文档和 PPT 中的指标必须来自 `03-evidence/` 接收的实验结果。
2. **不把目标值写成实测值。** 尚未收到的数据统一标记为“待外部输入”，不得根据赛题阈值反推或补造。
3. **一份事实源，多种交付形态。** 第二条线只维护 `01-report/report.md`，不修改队友的 `reports/report.md`；LaTeX、工程文档和 PPT 复用同一套架构、术语、证据编号和图表。
4. **不把实验执行转嫁给材料编写。** 缺失实验由开发与实验负责人确认或补充，材料负责人只做核对、分析和表达。
5. **公开材料必须脱敏。** 不使用 API Key、真实 Tailscale 地址、个人凭据、私有 kubeconfig、内部域名和可识别的服务器信息。
6. **内部记录不直接交付。** `progress_doc/` 仅作研发历史参考，对外材料只呈现已确认的设计、结果和必要局限。

> 发布排除项：`submission/00-management/AI-chating-archive.md` 是内部历史记录，含本机绝对路径和已废弃的 ElegantBook/旧目录方案，不得进入公开 PDF、源文件包或最终提交压缩包。禁止直接把整个 `submission/` 目录原样对外打包。

## 内部完成时间

赛题文件规定的官方截止时间为 **2026 年 8 月 31 日 23:59**。三类当前材料应在 **2026 年 8 月 10 日** 前完成内部版本，比官方截止时间提前 3 周。

该日期是报告、软件工程交付材料和 PPT 的内部完成日，不表示本材料负责人承担申报、投递、视频或最终材料打包工作。
