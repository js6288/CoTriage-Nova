# CoTriage 作品报告构建信息

## 当前输出

- `CoTriage-挑战杯作品报告-V0.3.pdf`：由 `../latex/main.tex` 生成的正式功能展示型作品报告。
- `../latex/build/main.pdf`：同一次构建产生的工程内 PDF；其 SHA-256 与正式输出一致。
- `../latex/build/style-sample.pdf`：排版系统样张，覆盖封面、目录、能力速览、正文、公式、图表和参考文献。

历史稳定 PDF 继续保留，当前提交与后续修改以 V0.3 为准。

## 文档结构

V0.3 采用 10 章功能主线，并保留两个技术附录：

1. 项目痛点与赛事目标；
2. 典型应用场景与用户需求；
3. 总体方案与系统架构；
4. 多节点采集与统一事件化；
5. 边缘分层推理与在线配置；
6. RAISE 动态路由与弱网补偿；
7. 云端根因诊断与反馈闭环；
8. 工程实现、管理中心与演示路径；
9. 赛事指标与系统运行效果；
10. 创新点、应用价值、当前边界与结论。

附录 A 收录 RAISE 方法定义与算法细节，附录 B 统一赛事指标术语与计算口径。

## 构建基线

- 构建日期：2026-08-09
- TeX Live：2025
- CTeX：2.5.10
- XeTeX：3.141592653-2.6-0.999997
- Biber：2.20
- latexmk：4.86a
- 字体：Fandol 中文字体、TeX Gyre 西文字体
- 参考文献：`biblatex-gb7714-2015` + Biber

## 复现命令

在仓库根目录执行：

```powershell
Set-Location submission/01-report/latex
latexmk -xelatex -interaction=nonstopmode -halt-on-error -outdir=build main.tex
latexmk -xelatex -interaction=nonstopmode -halt-on-error -outdir=build style-sample.tex
```

## 验收结果

- 正式报告：31 页 A4；排版样张：6 页 A4。
- 主报告通过 XeLaTeX、Biber 与 latexmk 完整构建。
- 构建日志未发现未定义引用、未定义文献、缺字或严重溢出。
- 报告逐页渲染检查未发现图片裁切、文字重叠、黑块或严重分页问题。
- 系统架构图继续使用矢量 PDF；运行截图均来自脱敏后的参赛素材。
- 第 9 章完整展示 `S-11` 当前运行窗口，并明确区分 P95、平均端到端时延、TTFT、混合统计窗口和零分母比例。
- 构建工程未引用文件名带敏感禁用标记的截图。
