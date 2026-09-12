# BGL Binary Classifier Training

本目录用于训练 CoTriage edge-infer 的第一版轻量二分类模型：输入 BGL 日志模板文本，输出 `NON_ALERT` 或 `ALERT`。

当前训练目标不是让模型直接完成故障诊断，而是做边缘侧异常初筛：

1. 高置信正常日志留在边缘侧；
2. 疑似异常日志进入边缘 LLM 或云端进一步诊断；
3. 训练结果后续导出为 ONNX，接入 `edge-infer` 的轻量分类路径。

## 环境

推荐使用 Conda 单独创建环境：

```bash
cd /Users/pangjian/researchProjects/cotriage-dev
conda env create -f dataset/model-training/environment.yml
conda activate cotriage-model-training
```

如果已经有可用 Python 环境，也可以使用 pip：

```bash
pip install -r dataset/model-training/requirements.txt
```

如果 ONNX 导出时报 `ModuleNotFoundError: No module named 'onnxscript'`，说明当前 PyTorch 版本触发了新 ONNX 导出器依赖，补装即可：

```bash
pip install onnxscript
```

如果 ONNX 推理校验时报 `ModuleNotFoundError: No module named 'onnxruntime'`，说明当前环境缺少 ONNX Runtime，补装即可：

```bash
pip install onnxruntime
```

## 训练 MiniLM 主模型

MiniLM 是当前第一版部署主线。它更轻，适合作为边缘侧轻量分类模型，只负责 `NON_ALERT/ALERT` 初筛：

```bash
python dataset/model-training/train_bgl_classifier.py \
  --model-name sentence-transformers/all-MiniLM-L6-v2 \
  --data-dir dataset/processed/bgl \
  --output-dir dataset/model-training/outputs/bgl-minilm \
  --epochs 20 \
  --batch-size 16 \
  --learning-rate 2e-5 \
  --max-length 128 \
  --class-weight auto \
  --patience 5
```

## 导出 MiniLM ONNX 部署包

训练完成后，导出 ONNX：

```bash
python dataset/model-training/export_onnx.py \
  --model-dir dataset/model-training/outputs/bgl-minilm/model \
  --metrics-file dataset/model-training/outputs/bgl-minilm/metrics.json \
  --output-dir dataset/model-training/outputs/bgl-minilm/onnx \
  --max-length 128 \
  --overwrite-output-dir
```

导出目录包含：

- `model.onnx`：ONNX Runtime 推理文件；
- `tokenizer/`：tokenizer 配置；
- `deployment.json`：edge-infer 后续加载所需的阈值、标签、长度等元数据；
- `training_metrics.json`：训练评估快照。

## 本地推理校验

使用 PyTorch checkpoint 推理：

```bash
python dataset/model-training/predict.py \
  --backend torch \
  --model-dir dataset/model-training/outputs/bgl-minilm/model \
  --text "KERNEL FATAL rts panic! - stopping execution"
```

使用 ONNX Runtime 推理：

```bash
python dataset/model-training/predict.py \
  --backend onnx \
  --onnx-dir dataset/model-training/outputs/bgl-minilm/onnx \
  --text "KERNEL FATAL rts panic! - stopping execution"
```

## 启动 ONNX HTTP 服务

edge-infer 通过 HTTP 调用 ONNX classifier：

```bash
python dataset/model-training/serve_onnx.py \
  --onnx-dir dataset/model-training/outputs/bgl-minilm/onnx \
  --host 127.0.0.1 \
  --port 18080
```

健康检查：

```bash
curl http://127.0.0.1:18080/healthz
```

推理接口：

```bash
curl -X POST http://127.0.0.1:18080/predict \
  -H 'Content-Type: application/json' \
  -d '{"template_id":"demo","template_text":"KERNEL FATAL rts panic! - stopping execution"}'
```

edge-infer 启动时配置：

```bash
CLASSIFIER_URL=http://127.0.0.1:18080/predict \
REDIS_ADDR=127.0.0.1:6379 \
go run ./cmd/edge-infer
```

## 构建 Docker 镜像并部署到集群

当前 Dockerfile 会把 `outputs/bgl-minilm/onnx` 部署包直接打进镜像，适合 MVP 和比赛演示。构建上下文必须使用 `dataset/model-training`，避免把整个仓库和原始数据集打进镜像：

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f dataset/model-training/Dockerfile \
  -t crpi-fq916lepz6l0u7gr.cn-guangzhou.personal.cr.aliyuncs.com/pangjian001/onnx-classifier:v0.0.1-202606251354 \
  dataset/model-training \
  --push
```

推送镜像：

```bash
docker push crpi-fq916lepz6l0u7gr.cn-guangzhou.personal.cr.aliyuncs.com/pangjian001/onnx-classifier:bgl-minilm-v0.0.1
```

部署到 Kubernetes：

```bash
kubectl apply -f charts/onnx-classifier/service.yaml
kubectl apply -f charts/onnx-classifier/deployment.yaml
```

检查服务：

```bash
kubectl -n cotriage get pod -l app=onnx-classifier
kubectl -n cotriage port-forward svc/onnx-classifier 18080:18080
curl http://127.0.0.1:18080/healthz
```

集群内 edge-infer 可以使用：

```text
CLASSIFIER_URL=http://onnx-classifier:18080/predict
```

## 训练 DistilBERT 对照组

DistilBERT 更大，可以作为精度对照实验：

```bash
python dataset/model-training/train_bgl_classifier.py \
  --model-name distilbert-base-uncased \
  --data-dir dataset/processed/bgl \
  --output-dir dataset/model-training/outputs/bgl-distilbert \
  --epochs 20 \
  --batch-size 16 \
  --learning-rate 2e-5 \
  --max-length 128 \
  --class-weight auto \
  --patience 5
```

## 产物

训练完成后，输出目录包含：

- `model/`：Hugging Face 格式模型和 tokenizer；
- `metrics.json`：验证集最佳阈值、测试集指标和训练配置；
- `history.jsonl`：每个 epoch 的训练/验证指标；
- `label_mapping.json`：标签 ID 与名称映射。

## 注意事项

当前 BGL 预处理结果只有 888 个模板样本，测试集 ALERT 模板只有 9 个。因此第一版结果只能用于判断方案是否跑通，不能作为最终比赛指标。后续需要补充时间切分事件级数据和其他公开日志数据集。
