# CoTriage 

## 系统组成

| 目录 | 层级 | 作用 | 技术栈 |
|---|---|---|---|
| `log-collector/` | 端侧 | 就近采集节点日志并统一事件化 | Go |
| `edge-infer/` | 边缘侧 | 规则、模板、轻量分类器与边缘 LLM 分层推理 | Go |
| `router-controller/` | 边缘控制侧 | RAISE 动态路由、Outbox、反馈与状态管理 | Go + MySQL + Redis |
| `cloud-gateway/` | 云侧 | 上下文聚合、云模型调用、结构化诊断和反馈写回 | Go |
| `server/` | 管理面 | 设备、知识库、模型、路由和可观测 API | Go |
| `web/` | 展示层 | 实时态势、诊断结果和配置治理界面 | Vue 3 + TypeScript |
| `dataset/model-training/` | 模型工具 | MiniLM/DistilBERT 训练、ONNX 导出和推理服务 | Python |
| `charts/` | 部署层 | Kubernetes 工作负载、服务、RBAC 与存储清单 | YAML |

## 核心数据链

```text
日志源 -> log-collector -> Redis Streams -> edge-infer
       -> router-controller -> LOCAL / CLOUD / DEFER
       -> cloud-gateway -> 结构化诊断与反馈
       -> server API -> web
```

MySQL 保存配置、路由记录与诊断结果；Redis Streams 承载组件间事件流。边缘快速路径不依赖云端，网络不可用时由本地结论和 Outbox 延期请求维持业务连续性。

## 本地检查

环境要求：Go 1.22+、Node.js 20+、npm 10+；模型工具另需 Python 3.11。

```bash
make test
make build
```

也可以单独检查组件：

```bash
cd edge-infer && go test ./...
cd ../router-controller && go test ./...
cd ../web && npm ci && npm run build
```

模型训练与 ONNX 服务的依赖、数据预处理和启动方式见 `dataset/model-training/README.md`。模型权重未进入代码归档，需要按照该文档训练/导出，或在构建镜像前放入指定的 `outputs/.../onnx` 目录。

## 容器构建

每个正式组件均包含独立 `Dockerfile`。在归档根目录执行：

```bash
make docker-build
```

镜像名称和版本可以覆盖：

```bash
IMAGE_PREFIX=registry.example.com/cotriage TAG=v1.0.0 make docker-build
```

ONNX 分类器镜像需要先准备导出的模型文件，具体见模型工具目录说明。

## Kubernetes 部署

部署清单位于 `charts/`，默认 namespace 为 `cotriage`。部署前必须创建自己的 Secret，禁止把真实凭据提交到仓库：

```bash
kubectl apply -f charts/namespace.yaml
cp charts/secrets.example.yaml charts/secrets.local.yaml
# 编辑本地文件中的占位值后：
kubectl apply -f charts/secrets.local.yaml
```

私有镜像仓库还需创建 `docker-registry-secret`，或删除清单中的 `imagePullSecrets` 并替换为公开镜像。随后按基础设施、服务组件的顺序应用清单；完整说明见 `charts/README.md`。
