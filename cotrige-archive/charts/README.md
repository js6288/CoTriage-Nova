# Kubernetes 部署说明

所有工作负载默认部署到 `cotriage` namespace，副本数为 1；`log-collector` 和日志回放器按节点采用 DaemonSet。

## 1. 准备 namespace 与 Secret

```bash
kubectl apply -f namespace.yaml
cp secrets.example.yaml secrets.local.yaml
# 替换全部 REPLACE_WITH_* 占位值
kubectl apply -f secrets.local.yaml
```

若使用私有仓库，请另外创建各清单引用的拉取凭据：

```bash
kubectl create secret docker-registry docker-registry-secret \
  --namespace cotriage \
  --docker-server=REGISTRY_HOST \
  --docker-username=REGISTRY_USER \
  --docker-password=REGISTRY_PASSWORD
```

提交前不要把 `secrets.local.yaml` 加入版本控制。

## 2. 替换镜像

清单保留了演示环境镜像版本。若需独立复现，请将以下清单的 `image` 替换为自行构建的镜像：

- `log-collector/daemonset.yaml`
- `edge-infer/deployment.yaml`
- `router-controller/deployment.yaml`
- `cloud-gateway/deployment.yaml`
- `server/deployment.yaml`
- `web/deployment.yaml`
- `onnx-classifier/deployment.yaml`
- `replayer/daemonset.yaml`

## 3. 推荐部署顺序

```bash
kubectl apply -f redis/
kubectl apply -f mysql/
kubectl apply -f onnx-classifier/
kubectl apply -f log-collector/rbac.yaml
kubectl apply -f log-collector/daemonset.yaml
kubectl apply -f edge-infer/deployment.yaml
kubectl apply -f router-controller/deployment.yaml
kubectl apply -f cloud-gateway/deployment.yaml
kubectl apply -f server/deployment.yaml
kubectl apply -f web/deployment.yaml
```

日志回放器是演示数据源，按需执行：

```bash
kubectl apply -f replayer/daemonset.yaml
```

## 4. 状态检查

```bash
kubectl get deployments,daemonsets,pods,services -n cotriage
kubectl rollout status deployment/server -n cotriage
kubectl rollout status deployment/web -n cotriage
```

Web 可通过 Service 端口转发访问：

```bash
kubectl port-forward -n cotriage service/web 8080:80
```

浏览器打开 `http://localhost:8080/`。
