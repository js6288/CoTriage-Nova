# CoTriage 作品报告 LaTeX 工程

本目录使用 `ctexrep` 构建中文工程技术作品报告，不复制或修改 CTeX 类文件，也不依赖 ElegantBook。正文内容与 `../report.md` 保持同一术语体系；这里负责模块化排版、交叉引用与最终 PDF 输出。

## 文件说明

- `main.tex`：10 章正文与附录 A/B 的正式入口。
- `style-sample.tex`：覆盖封面、目录、能力速览、正文、公式、算法、宽表、矢量图、运行截图和参考文献的版式样张。
- `metadata.tex`：作品名称、团队、版本与日期。
- `cotriage-report.sty`：A4 版式、字体、色板、标题、页眉页脚与图表样式。
- `chapters/`：功能展示型报告正文与技术附录。
- `figures/`：系统架构矢量图和经过脱敏检查的系统运行截图。
- `tables/`：作品能力速览与赛事指标口径表。
- `references.bib`：GB/T 7714 参考文献数据库。

## 编译

复现基线为 TeX Live 2025、CTeX 2.5.10、XeTeX、Biber 2.20、`latexmk` 4.86a 和 `biblatex-gb7714-2015`。

```powershell
Set-Location submission/01-report/latex
latexmk -xelatex -interaction=nonstopmode -halt-on-error -outdir=build main.tex
latexmk -xelatex -interaction=nonstopmode -halt-on-error -outdir=build style-sample.tex
```

输出分别为 `build/main.pdf` 与 `build/style-sample.pdf`。正式 V0.3 PDF 从最新 `build/main.pdf` 复制到 `../output/CoTriage-挑战杯作品报告-V0.3.pdf`，不得覆盖已有稳定版本。

## 发布约束

1. 不把运行截图中的瞬时值扩写为赛题达标结论。
2. 平均值、P95、TTFT、窗口百分比和累计值必须按第 9 章及附录 B 的口径解释。
3. 公开稿只保留正式内容，不显示内部编稿字段。
4. 未经授权不得加入赛事 Logo；真实 IP、密钥、内部账号和文件路径不得进入提交版 PDF。
5. 带有禁用标记的敏感截图不得复制到 `figures/` 或被任何入口引用。
