# Cotriage 本地调试 MySQL 连接排查 SOP

## 1. 文档目的

本文档用于排查本机通过 VS Code/Delve 启动 `server` 时无法连接 Kubernetes 集群内 MySQL 的问题。

适用环境：

- Kubernetes namespace：`cotriage`
- MySQL Service：`mysql`
- 本地 kubeconfig：`kind-host`
- 数据库：`cotriage`
- 用户：`cotriage`
- 本地调试入口：`.vscode/launch.json` 中的 `server`

## 2. 排查原则

根据错误信息从外到内逐层定位：

1. `no such host`：本机无法解析 Kubernetes 集群内部域名。
2. `connection refused`：本地端口没有监听，port-forward 未建立或已退出。
3. `Host ... is not allowed to connect`：网络已连通，但 MySQL 用户授权不允许该来源连接。

## 3. 集群域名无法解析

### 3.1 报错特征

```text
dial tcp: lookup mysql.cotriage.svc.cluster.local: no such host
```

### 3.2 原因

`mysql.cotriage.svc.cluster.local` 是 Kubernetes 集群内部 DNS 名称，通常只能被集群内 Pod 解析。本机启动的 `server` 不能直接使用该地址。

`kubectl port-forward` 只建立本地端口到集群 Service 的转发，不会让本机获得 `*.svc.cluster.local` 的 DNS 解析能力。

### 3.3 解决方案

在 `.vscode/launch.json` 的 `server.env` 中配置本地 MySQL DSN：

```json
"MYSQL_DSN": "cotriage:cotriage-pass@tcp(127.0.0.1:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"
```

集群内部署 `server` 时仍可使用默认的集群域名，无需修改服务端默认配置。

## 4. 本地端口拒绝连接

### 4.1 报错特征

```text
dial tcp 127.0.0.1:3306: connect: connection refused
```

### 4.2 原因

DSN 已正确指向本机，但本地 `3306` 端口没有 port-forward 进程监听。常见原因包括：

- 尚未执行 port-forward；
- port-forward 所在终端已关闭；
- port-forward 因 MySQL Pod 重启而退出；
- 实际映射的是其他本地端口。

### 4.3 解决方案

在仓库根目录新开终端并执行：

```bash
kubectl --kubeconfig kind-host -n cotriage port-forward svc/mysql 3306:3306
```

保持该终端运行，出现以下输出表示转发成功：

```text
Forwarding from 127.0.0.1:3306 -> 3306
```

检查本地监听状态：

```bash
lsof -nP -iTCP:3306 -sTCP:LISTEN
```

如果本地 `3306` 已被占用，可以改用 `13306`：

```bash
kubectl --kubeconfig kind-host -n cotriage port-forward svc/mysql 13306:3306
```

同时将 `.vscode/launch.json` 中 `MYSQL_DSN` 的地址改为 `127.0.0.1:13306`。

## 5. MySQL 拒绝当前 Host

### 5.1 报错特征

```text
Error 1130: Host '127.0.0.1' is not allowed to connect to this MySQL server
```

### 5.2 原因

此错误说明 DNS 和端口转发都已正常，连接已经到达 MySQL，但数据库中的 `cotriage` 用户不允许当前来源连接。

MySQL 用户由“用户名 + Host”共同标识，例如 `'cotriage'@'localhost'` 与 `'cotriage'@'%'` 是不同账号。允许通过 Service 或 port-forward 连接时，需要存在合适的 Host 授权。

### 5.3 解决方案

进入 MySQL Pod：

```bash
kubectl --kubeconfig kind-host -n cotriage exec -it deploy/mysql -- \
  mysql -uroot -proot-pass
```

在 MySQL 终端执行：

```sql
CREATE DATABASE IF NOT EXISTS cotriage;

CREATE USER IF NOT EXISTS 'cotriage'@'%' IDENTIFIED BY 'cotriage-pass';
ALTER USER 'cotriage'@'%' IDENTIFIED BY 'cotriage-pass';

GRANT ALL PRIVILEGES ON cotriage.* TO 'cotriage'@'%';
FLUSH PRIVILEGES;
```

可选检查用户与授权：

```sql
SELECT User, Host FROM mysql.user WHERE User = 'cotriage';
SHOW GRANTS FOR 'cotriage'@'%';
```

执行完成后输入 `exit`，保持 port-forward 运行，再重新启动 VS Code 的 `server` 调试配置。

## 6. 完整验证流程

### 6.1 检查 MySQL 工作负载

```bash
kubectl --kubeconfig kind-host -n cotriage get pod -l app=mysql
kubectl --kubeconfig kind-host -n cotriage get svc mysql
```

确认 MySQL Pod 为 `Running` 且已就绪。

### 6.2 启动端口转发

```bash
kubectl --kubeconfig kind-host -n cotriage port-forward svc/mysql 3306:3306
```

### 6.3 确认调试配置

`.vscode/launch.json` 中的 `server.env` 应包含：

```json
"MYSQL_DSN": "cotriage:cotriage-pass@tcp(127.0.0.1:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"
```

### 6.4 启动调试

在 VS Code 中选择 `server` 配置并启动。若不再出现 `init mysql` 错误，说明 MySQL 连接成功。

## 7. 关键注意事项

- port-forward 是前台长连接，终端关闭后转发立即失效。
- MySQL Pod 重建后，原 port-forward 可能中断，需要重新执行。
- `MYSQL_USER`、`MYSQL_PASSWORD` 等容器环境变量通常只在空数据目录首次初始化时生效。
- 当前 MySQL 使用 PVC `mysql-data`。数据库初始化完成后，仅修改 Kubernetes Secret 或 Deployment 环境变量不会自动修改 PVC 中已有用户的密码和授权。
- 不要为修复授权直接删除 PVC，除非确认数据库数据可以全部丢弃。
- 本地调试使用 `127.0.0.1`；集群内 Pod 访问 MySQL 仍应使用 `mysql.cotriage.svc.cluster.local:3306` 或 `mysql:3306`。
