# 实验与佐证材料

本目录用于接收开发与实验负责人提供的材料，是作品报告、软件工程交付材料和参赛 PPT 中量化结论的证据入口。实验执行与服务器运维不属于材料负责人的任务。

实验负责人按 [`experiment-handoff-template.md`](experiment-handoff-template.md) 提供以下最小交接包：

报告、工程文档和 PPT 统一使用 [`evidence-index.yaml`](evidence-index.yaml) 中的证据编号。量化主张只有在对应条目的状态由 `pending_external` 更新为 `verified_external` 后，才能进入正式材料。

需要开发与实验负责人提供的具体数据和截图见 [`developer-evidence-request.md`](developer-evidence-request.md)。

已经收到并完成识别的系统运行截图及使用状态见 [`figure/README.md`](figure/README.md)。

```text
experiment_bundle/
├── manifest.yaml
├── experiment-overview.md
├── results.csv
├── configs/
├── raw/
├── summary/
├── logs/
├── screenshots/
├── figures/
├── commands.md
└── checksums.txt
```

仓库可以只保存适合版本控制的小型配置、汇总表、绘图数据和必要截图；大型原始数据、模型和日志通过外部存储交付。

严禁将 API Key、真实 Tailscale 地址、kubeconfig 或其他凭据放入本目录。
