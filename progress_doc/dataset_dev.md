# Dataset 开发记录

## 日期：2026年06月22日

### 内容

1. 检查 `dataset/BGL.log` 完整文件：大小约 709MB，共 4,747,963 行。
2. 统计 BGL 标签分布：`NON_ALERT` 4,399,503 行，`ALERT` 348,460 行，告警比例约 7.34%，第一列包含约 40 种告警类别。
3. 确认 BGL 第一列为训练标签，不能出现在模型输入中；模型输入仅保留组件、日志级别和规范化消息模板。
4. 新增 `dataset/preprocess_bgl.py`，使用 Python 标准库和 SQLite 进行流式磁盘聚合，避免一次性加载 709MB 数据。
5. 实现 IP、UUID、十六进制、路径和数字占位符归一化，并生成稳定的 16 字符模板 ID。
6. 按模板统计 alert/non-alert 次数，以多数标签作为二分类目标，并计算 `label_purity`。
7. 标签纯度低于默认阈值 0.90 的模板写入 `ambiguous.jsonl`，不直接进入训练集。
8. 使用模板 ID 与固定 seed 的哈希结果划分 train/validation/test，保证同一模板不会跨数据集泄漏。
9. 输出模板文本、模型输入、二分类标签、原始告警类别、组件、级别、出现次数、标签计数、纯度、样例和时间范围。
10. 新增 BGL 解析、正常/告警标签、规范化、模板 ID、稳定切分和 malformed 输入测试。
11. 根 `.gitignore` 增加 `dataset/*` 规则，保留 README、Python 程序、测试和示例配置，忽略所有原始数据及生成训练产物。

### 验证结果

- 单元测试 4 项全部通过。
- 前 10,000 行烟雾测试全部成功解析，无 malformed；聚合得到 11 个模板。
- 烟雾样本位于 BGL 文件开头，全部为 non-alert，因此不能用于判断完整类别分布或训练效果。

### 后续事项

1. 执行全量预处理并检查 `dataset_stats.json`、模板数量、纯度分布和三个集合的类别比例。
2. 对 `ambiguous.jsonl` 中的冲突模板进行人工复核。
3. 在训练前决定采用类别权重、加权采样或 focal loss 处理约 1:12.6 的类别不平衡。
4. 在模板分组测试之外增加按时间切分的 OOD 测试集，评估时间分布漂移。
5. 使用生成的数据分别训练 DistilBERT 和 MiniLM 基线。

## 日期：2026年06月22日（全量结果审计与模板化修正）

### 内容

1. 审计全量预处理结果：有效解析 4,713,493 行，生成 46,134 个高纯度模板和 1 个冲突模板。
2. 三个集合的模板级告警比例分别为 4.93%、4.94% 和 5.01%，哈希分组后的类别分布保持稳定。
3. 发现原规范化规则未覆盖 `003a929c` 等裸十六进制地址以及 `FF:F2:...` 等冒号分隔硬件地址，导致相同语义被拆成过多模板。
4. 新增裸十六进制与长冒号十六进制序列归一化规则，分别替换为 `<HEX>` 和 `<HEXSEQ>`。
5. 确认原统计中的 34,470 条 malformed 实际为只有 9 个元数据字段、没有日志正文的记录；预处理程序现将其单独统计为 `empty_message_lines` 并继续排除。
6. 新增十六进制归一化单元测试。
7. 深入检查告警类别后发现 `bglio817` 等字母数字设备标识未归一化，使 `KERNMNTF` 产生 1405 个近重复模板并主导测试集。
8. 新增字母数字设备标识归一化，将 `bglio817`、`R73-M1-N4`、`eth0` 等动态编号替换为带 `<NUM>` 的稳定形式。

### 验证结果

- 当前 `dataset/processed/bgl/` 是修正规则前生成的版本，不能直接作为最终训练输入。
- 需要重新执行全量预处理，再检查模板数量、空正文数量和类别分布。

### 后续事项

1. 删除或覆盖现有 processed 输出并重新执行 `python3 dataset/preprocess_bgl.py`。
2. 重新审计 `dataset_stats.json` 后再进入 DistilBERT 基线训练。

## 日期：2026年06月22日（最终预处理复检）

### 内容

1. 使用新增的字母数字设备标识归一化规则重新处理全部 4,747,963 行 BGL 日志。
2. 高纯度模板从上一版 7,193 个下降到 887 个，另有 1 个纯度 0.5 的冲突模板进入 ambiguous。
3. 检查全部 888 个模板：template ID 唯一，train/validation/test/ambiguous 之间不存在交叉泄漏。
4. 检查所有 JSONL 字段、标签名称、多数标签、occurrences 计数、纯度阈值、split 字段和模型输入前缀，未发现一致性错误。
5. 检查模板正文，未发现残留 IP、长裸十六进制或冒号分隔硬件地址。
6. 输入长度中位数约 13 词，P95 为 24-26 词，最大 95 词，使用 tokenizer `max_length=128` 基本合理。
7. 训练集包含 559 个 NON_ALERT 和 65 个 ALERT 模板；验证集为 132/14；测试集为 108/9。
8. 测试集 ALERT 模板仅 9 个，单个样本会使 Recall 波动约 11.1%；同时 `KERNRTSA` 和 `KERNSTOR` 在测试集中出现但训练集中没有。

### 验证结果

- 预处理数据在格式、标签纯度、动态字段归一化和模板隔离方面符合要求。
- 当前数据可用于 DistilBERT/MiniLM 的第一版模板级二分类基线。
- 当前测试集 ALERT 模板过少，不足以作为最终比赛指标或证明模型具备稳定泛化能力。

### 后续事项

1. 保留当前模板隔离集合，用于严格的“未见模板泛化”评测，并报告置信区间。
2. 另行生成事件级时间切分数据，用于评估线上流量分布下的 Precision、Recall、F1 和 PR-AUC。
3. 引入 Thunderbird、HDFS 或 OpenStack 等其他带标签日志，扩充 ALERT 语义模板。
4. 先训练 DistilBERT 基线，但不得将当前 9 个 ALERT 测试模板的单次结果作为最终结论。

## 日期：2026年06月23日（模型训练目录初始化）

### 内容

1. 新增 `dataset/model-training/`，用于训练 BGL 模板级 `NON_ALERT/ALERT` 二分类模型。
2. 新增 `environment.yml` 和 `requirements.txt`，支持通过 Conda 或 pip 安装 `torch`、`transformers`、`scikit-learn` 等训练依赖。
3. 新增 `train_bgl_classifier.py`，使用自定义 PyTorch 训练循环，避免 Hugging Face Trainer 带来的额外内存与抽象成本。
4. 训练脚本支持 MiniLM 和 DistilBERT 等 Hugging Face 序列分类模型，并默认使用 `sentence-transformers/all-MiniLM-L6-v2`。
5. 脚本支持自动选择 `cuda`、`mps` 或 `cpu`，适配当前 10c16g Mac 的本地微调环境。
6. 支持 `class_weight=auto`，基于训练集标签分布自动提高 ALERT 类别权重。
7. 在验证集上搜索最佳二分类阈值，并将该阈值用于测试集评估。
8. 输出 Hugging Face 格式模型、tokenizer、`metrics.json`、`history.jsonl` 和 `label_mapping.json`。
9. 更新根 `.gitignore`，允许提交 `dataset/model-training/` 代码，同时忽略训练输出、checkpoint、run 和本地缓存目录。

### 验证结果

- 当前训练代码尚未执行完整微调。
- 当前 shell 环境未检测到 `torch`、`transformers`、`datasets`、`sklearn` 或 `onnxruntime`，需要先创建 Conda 环境或安装 requirements。
- 当前训练数据规模为 624/146/117 条模板样本，本机 Mac 可承担第一版 MiniLM/DistilBERT 微调。

### 后续事项

1. 创建并激活 `cotriage-model-training` Conda 环境。
2. 优先训练 MiniLM baseline，记录 validation 阈值与 test 指标。
3. 再训练 DistilBERT 对照组，比较 F1、Recall、PR-AUC 和推理成本。
4. 若 MiniLM 指标可接受，继续实现 ONNX 导出与 edge-infer 轻量分类路径接入。

## 日期：2026年06月23日（DistilBERT 部署产物脚本）

### 内容

1. 根据 MiniLM 与 DistilBERT 训练结果，确定 DistilBERT 作为第一版轻量分类模型主线。
2. 新增 `dataset/model-training/export_onnx.py`，支持将 `outputs/bgl-distilbert/model` 导出为 ONNX Runtime 部署包。
3. ONNX 部署包包含 `model.onnx`、`tokenizer/`、`deployment.json` 和训练指标快照。
4. `deployment.json` 固化模型类型、ONNX 文件名、tokenizer 路径、`max_length`、验证集最佳阈值、标签映射和 opset。
5. 新增 `dataset/model-training/predict.py`，支持使用 PyTorch checkpoint 或 ONNX Runtime 对单条模板文本执行本地推理。
6. 更新训练环境依赖，加入 `onnx` 和 `onnxruntime`。
7. 修正 Conda 环境名为 `cotriage-model-training`，与 README 中的激活命令保持一致。
8. 更新 README，将 DistilBERT 训练、ONNX 导出和本地推理校验作为当前主流程。

### 验证结果

- 已完成脚本静态语法检查。
- 尚未在当前回合执行 ONNX 导出；需要在已安装训练依赖的 Conda 环境中运行导出命令。

### 后续事项

1. 使用已训练的 DistilBERT checkpoint 执行 ONNX 导出。
2. 分别使用 `predict.py --backend torch` 和 `predict.py --backend onnx` 对同一条日志模板做推理，确认输出概率接近。
3. 将 ONNX 部署包接入 `edge-infer` 的轻量分类模型路径。

## 日期：2026年06月23日（ONNX 导出依赖修复）

### 内容

1. 用户执行 DistilBERT ONNX 导出时遇到 `ModuleNotFoundError: No module named 'onnxscript'`。
2. 原因是当前 PyTorch 版本默认导出路径会加载新的 ONNX exporter 模块，该模块依赖 `onnxscript`。
3. `export_onnx.py` 现在显式传入 `dynamo=False`，默认使用传统 ONNX 导出路径，减少额外依赖摩擦。
4. 新增 `--dynamo` 参数，后续如需测试 PyTorch 新导出器，可显式启用。
5. `environment.yml`、`requirements.txt` 和 README 均补充 `onnxscript` 依赖说明，保证新旧导出器都可工作。

### 验证结果

- 已完成脚本静态语法检查。
- 需要在用户当前 `cotriage` Conda 环境中补装或更新依赖后重新执行导出命令。

### 后续事项

1. 在当前环境执行 `pip install onnxscript` 或 `pip install -r dataset/model-training/requirements.txt`。

## 日期：2026年06月23日（MiniLM ONNX 部署包切换）

### 内容

1. 将模型训练与 ONNX 服务默认路径从 `outputs/bgl-distilbert` 切换为 `outputs/bgl-minilm`。
2. 使用已训练的 MiniLM checkpoint 导出 ONNX 部署包：`dataset/model-training/outputs/bgl-minilm/onnx`。
3. MiniLM 部署包包含 `model.onnx`、`tokenizer/`、`deployment.json` 和 `training_metrics.json`。
4. `deployment.json` 中阈值来自 MiniLM 验证集最佳阈值，当前为 `0.05`。
5. 更新 `export_onnx.py`、`predict.py`、`serve_onnx.py` 的默认路径，后续不传参数时默认使用 MiniLM。
6. 更新 `dataset/model-training/README.md`，将 MiniLM 调整为当前主模型，DistilBERT 保留为对照组。

### 验证结果

- `/opt/anaconda3/envs/cotriage/bin/python dataset/model-training/export_onnx.py --model-dir dataset/model-training/outputs/bgl-minilm/model --metrics-file dataset/model-training/outputs/bgl-minilm/metrics.json --output-dir dataset/model-training/outputs/bgl-minilm/onnx --max-length 128 --overwrite-output-dir`：通过。
- `/opt/anaconda3/envs/cotriage/bin/python dataset/model-training/predict.py --backend onnx --onnx-dir dataset/model-training/outputs/bgl-minilm/onnx --text "KERNEL FATAL rts panic! - stopping execution"`：输出 `ALERT`，`alert_probability=0.989299`，阈值 `0.05`。

### 后续事项

1. 使用 VS Code `onnx-classifier` 启动 MiniLM ONNX HTTP 服务。
2. 观察 edge-infer 中 `model_version` 是否显示 `dataset/model-training/outputs/bgl-minilm/model`。
3. 若误报偏高，可重新调高阈值或回切 DistilBERT 对照模型。
2. 重新运行 `export_onnx.py`。
3. 使用 `predict.py --backend onnx` 校验 ONNX Runtime 推理结果。

## 日期：2026年06月23日（ONNX Runtime 推理依赖提示）

### 内容

1. 用户执行 `predict.py --backend onnx` 时遇到 `ModuleNotFoundError: No module named 'onnxruntime'`。
2. 原因是 ONNX 文件导出后，还需要 ONNX Runtime 作为本地推理引擎。
3. `predict.py` 现在会捕获 `torch`、`transformers`、`onnxruntime` 缺失错误，并输出明确安装命令。
4. README 增加 ONNX Runtime 缺失时的处理方式。

### 验证结果

- 已完成脚本静态语法检查。
- 需要在用户当前 Conda 环境中安装 `onnxruntime` 后重新执行 ONNX 推理校验。

### 后续事项

1. 在当前环境执行 `pip install onnxruntime` 或安装完整 requirements。
2. 重跑 `predict.py --backend onnx`。
3. 若 ONNX 推理输出正常，再进入 edge-infer 分类模型接入。

## 日期：2026年06月23日（ONNX HTTP Classifier 服务）

### 内容

1. 用户完成 DistilBERT ONNX 推理校验，示例日志输出 `ALERT`，`alert_probability=0.451605`，阈值为 `0.25`。
2. 新增 `dataset/model-training/serve_onnx.py`，将导出的 ONNX 部署包封装为本地 HTTP 服务。
3. 服务读取 `deployment.json`、`model.onnx` 和 `tokenizer/`，保持阈值、标签映射和 `max_length` 与训练产物一致。
4. 服务提供 `GET /healthz` 健康检查和 `POST /predict` 推理接口。
5. `/predict` 支持输入 `template_text` 或 `text`，输出 `label`、`label_id`、`alert_probability`、`threshold`、`probabilities` 和 `model_version`。
6. README 增加 ONNX HTTP 服务启动、健康检查、curl 推理和 edge-infer 接入示例。

### 验证结果

- 已完成 Python 脚本静态语法检查。
- 尚未在当前回合启动 HTTP 服务做端到端推理，因为服务运行需要用户当前 Conda 环境中的 ONNX Runtime。

### 后续事项

1. 在 `cotriage` 环境中启动 `serve_onnx.py`。
2. 使用 `curl /healthz` 和 `curl /predict` 做服务级校验。
3. 配置 edge-infer 的 `CLASSIFIER_URL` 并完成 Redis Stream 端到端联调。

## 日期：2026年06月23日（VS Code Python 环境固定）

### 内容

1. VS Code 启动 `onnx-classifier` 时使用了当前选中的 `recommender` Conda 环境，导致运行环境与模型训练/导出环境不一致。
2. `.vscode/launch.json` 的 `onnx-classifier` 配置新增 `python=/opt/anaconda3/envs/cotriage/bin/python`。
3. 该配置固定使用 `cotriage` Conda 环境启动 `serve_onnx.py`，避免 VS Code 当前解释器切换影响 ONNX Runtime 依赖。

### 验证结果

- 需要重新通过 VS Code 启动 `onnx-classifier`，确认终端命令使用 `/opt/anaconda3/envs/cotriage/bin/python`。

### 后续事项

1. 如未来改用 `cotriage-model-training` 环境，需要同步修改 `.vscode/launch.json` 中的 Python 路径。

## 日期：2026年06月25日（ONNX classifier 容器化与集群部署）

### 内容

1. 新增 `dataset/model-training/Dockerfile`，将 `serve_onnx.py` 和 `outputs/bgl-minilm/onnx` 打包为 ONNX classifier 镜像。
2. 新增 `dataset/model-training/requirements-serve.txt`，仅保留在线推理所需依赖：`numpy`、`onnxruntime`、`transformers`。
3. 新增 `dataset/model-training/.dockerignore`，限制 Docker build context，避免把训练 checkpoint、runs 和无关输出打进镜像。
4. 新增 `charts/onnx-classifier/deployment.yaml`，部署 `onnx-classifier` Deployment，暴露 18080 端口，并配置 `/healthz` readiness/liveness probe。
5. 新增 `charts/onnx-classifier/service.yaml`，提供集群内 `onnx-classifier:18080` ClusterIP Service。
6. 更新 `dataset/model-training/README.md`，补充镜像构建、推送、Kubernetes 部署、健康检查和 edge-infer 集群内 `CLASSIFIER_URL`。

### 验证结果

- 已完成 Dockerfile、dockerignore 和 Kubernetes YAML 静态检查。
- 未在当前回合执行 `docker build`，因为构建需要拉取基础镜像和安装 pip 依赖，依赖网络环境。
