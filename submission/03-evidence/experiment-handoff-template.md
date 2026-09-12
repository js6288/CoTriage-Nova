# CoTriage 实验材料交接格式

> 版本：V1.1
> 冻结日期：2026-07-29
> 提供方：实验负责人
> 接收方：报告、软件工程文档与 PPT 材料负责人

本格式用于将队员服务器上的实验数据一次性交接给作品报告、软件工程交付材料和参赛 PPT 的编写人员。材料负责人无需重新运行完整系统，只需根据交接内容理解实验条件、核对指标并生成图表。

## 1. 最小交接目录

```text
experiment_bundle/
├── manifest.yaml           # 批次、版本、环境、数据、随机种子与文件映射
├── experiment-overview.md  # 便于人工阅读的环境、模型和实验说明
├── results.csv             # 统一汇总结果，未测字段保持为空
├── configs/                # 路由、模型、网络和场景配置
├── raw/                    # 必要的逐事件或逐轮原始 CSV/JSON
├── summary/                # 按统一口径生成的汇总表
├── logs/                   # 支撑关键结论的运行日志
├── screenshots/            # 系统、大盘和实验状态截图
├── figures/                # 已生成的可选图表及绘图数据
├── commands.md             # 可复现命令、执行顺序和异常说明
└── checksums.txt           # 交付文件校验值
```

大型原始数据和模型可以通过共享目录交接，但 `manifest.yaml` 必须记录其位置、版本和校验信息；仓库内仍保留足以核对主要结论的小型原始结果、汇总表和配置。

### 1.1 `manifest.yaml` 必填字段

```yaml
bundle_version: 1
experiment_id: ""
executed_at: ""
owner: ""
git_commit: ""
scenarios: []
datasets: []
seeds: []
repeats: null
nodes: []
software_versions: {}
models: {}
network_profiles: []
result_files: []
known_failures: []
```

`result_files` 中每项应填写相对路径、对应证据编号和内容说明。任何字段未测或未知时使用 `null` 或空列表，不使用零值代替缺失。

## 2. `experiment-overview.md` 必填内容

### 2.1 基本信息

| 字段 | 填写内容 |
| --- | --- |
| 实验批次名称 |  |
| 执行日期 |  |
| 负责人 |  |
| Git commit |  |
| 场景 | 工业设备监测 / 分布式基础设施监测 / 其他 |
| 数据集 |  |
| 重复次数 |  |
| 随机种子 |  |

### 2.2 环境信息

| 字段 | 填写内容 |
| --- | --- |
| 边缘节点数量 |  |
| CPU / 内存 / GPU |  |
| 操作系统与架构 |  |
| Kubernetes 版本 |  |
| Redis / MySQL 版本 |  |
| MiniLM ONNX 模型版本与大小 |  |
| 边缘小语言模型及量化方式 |  |
| 云端模型/API |  |
| 基线 RTT / 带宽 |  |

### 2.3 实验说明

每组实验说明以下内容：

- 研究问题和对比方法；
- 输入数据范围和划分方式；
- RAISE及基线参数；
- 网络延迟、丢包、带宽、断网与恢复设置；
- 时延的起点和终点；
- 异常、失败或人工干预情况。

## 3. `results.csv` 冻结字段

每行表示一种方法在一个场景和网络条件下的一次运行或汇总结果。

```text
run_id,scenario,dataset,method,network_condition,repeat_count,accuracy,macro_f1,high_risk_recall,edge_infer_p50_ms,edge_infer_p95_ms,e2e_p50_ms,e2e_p95_ms,e2e_p99_ms,ttft_ms,capability_retention_ratio,cloud_ratio,upload_bytes,token_per_event,cloud_call_count,estimated_cost,model_size_mb,peak_memory_mb,throughput_eps,local_decision_rate,total_success_rate,defer_count,outbox_max_size,outbox_replay_success_rate,outbox_drain_seconds,conflict_count,conflict_rate,resolved_conflict_count,conflict_resolution_rate,notes
```

没有测量的字段保留为空，不填 `0`。百分比统一保存为 `0` 到 `1` 的小数，时延统一使用毫秒，内存和模型大小统一使用 MB，上传量统一使用字节。`estimated_cost` 必须在实验概览中说明币种和估算方式。

## 4. 文件命名

```text
<日期>-<场景>-<方法>-<网络条件>-<内容>.<扩展名>
```

示例：

```text
20260731-industrial-raise-network-fluctuation-results.csv
20260731-infrastructure-all-cloud-normal-dashboard.png
```

文件名使用小写英文、数字和连字符，不包含空格、成员姓名、服务器地址或密钥。

## 5. 交接验收

- [ ] `manifest.yaml` 的提交版本、节点、模型、场景、数据集、随机种子和结果文件映射已填写。
- [ ] `experiment-overview.md` 的环境、模型、版本和测量口径已填写。
- [ ] `results.csv` 可以直接读取，字段名和单位符合本规范。
- [ ] 报告计划使用的每项指标都有对应结果，不只有截图。
- [ ] 关键图表能够追溯到 `results.csv` 或 `raw/` 中的数据。
- [ ] `commands.md` 足以说明运行顺序、参数和已知异常，且不包含凭据。
- [ ] `checksums.txt` 覆盖实际交接的配置、结果、日志、截图和图表。
- [ ] 日志和截图中没有 API Key、真实 Tailscale 地址或个人信息。
- [ ] 缺失、失败和异常实验已经如实说明。
