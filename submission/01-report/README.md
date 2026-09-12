# 作品报告

本目录存放第二条线作品报告事实源、LaTeX 排版工程和当前可核验的非实验版 PDF。队友的 `reports/report.md` 始终保持只读，不在本目录工作中回写。

建议流程：

1. 从队友的 `reports/report.md` 复制初稿，此后只在本目录的 `report.md` 中补全和审核；
2. 只在本目录的 `report.md` 维护正文事实，模块化 LaTeX 是其派生排版形态；
3. 架构、方法和实现描述先与当前代码核对，实验结果只从统一证据索引接入；
4. 每次修改 LaTeX 后重新编译，并逐页渲染检查 PDF。

当前结构：

```text
01-report/
├── report.md   # 第二条线 Markdown 事实源
├── latex/      # main.tex、章节、参考文献和模板
└── output/     # 稳定命名 PDF 与构建信息，不存放临时编译文件
```

当前输出：

- [`output/CoTriage-作品报告-非实验内容稿.pdf`](output/CoTriage-作品报告-非实验内容稿.pdf)
- [`output/CoTriage-LaTeX-样张.pdf`](output/CoTriage-LaTeX-样张.pdf)
- [`output/BUILD-INFO.md`](output/BUILD-INFO.md)
