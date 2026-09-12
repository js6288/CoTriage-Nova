# CoTriage-Nova

CoTriage 的 Java 25 迁移工程。五个核心服务使用 Spring Boot 4.1.1，
保留 Redis Streams、MySQL、Kubernetes、Vue 管理界面和 Python 模型服务。

| 模块 | 职责 |
| --- | --- |
| log-collector | 节点日志采集与 raw 事件发布 |
| edge-infer | 规则、模板、分类器和边缘 LLM 推理 |
| router-controller | RAISE 路由、Outbox、反馈与状态 |
| cloud-gateway | 上下文聚合、云模型诊断与反馈 |
| server | 47 个管理 API、Dashboard 与 SSE |
| cotriage-common | 旧格式兼容、连接和 schema 初始化 |
| migration-tests | 五个独立 Java 进程的隔离集成验证 |

## 构建和测试

需要 JDK 25；Maven Wrapper 固定为 3.9.16。从仓库根目录执行：

```powershell
.\mvnw.cmd verify
npm --prefix web ci
npm --prefix web run build
```

Linux/macOS 使用 `./mvnw verify`。`verify` 需要已运行的 Docker 引擎，
测试使用一次性的 MySQL 8.4 和 Redis 7.4，不连接已有集群。
仅打包可使用 `./mvnw -DskipTests package`。

独立服务的产物为
`<service>/target/<service>-0.1.0-SNAPSHOT.jar`。
配置沿用原环境变量，包括 Go 风格 `MYSQL_DSN`、duration、
Redis Stream 和消费者组；无需将已有部署配置重写为 JDBC URL。

## 镜像与 Kubernetes

```bash
./script/build-java-images.sh
IMAGE_PREFIX=registry.example/team IMAGE_TAG=java25 ./script/build-java-images.sh
```

默认镜像为 `cotriage-nova/<service>:java25`，构建脚本不推送镜像。
五个 Dockerfile 都以仓库根目录为 build context。渲染清单：

```powershell
.\script\render-java-manifests.ps1 -ImagePrefix cotriage-nova -ImageTag java25 -OutputPath .migration/java-manifests.yaml
```

清单保留原资源名称、RBAC、Secret 引用和挂载。部署者需提供目标集群的
kubeconfig、基础设施和 Secret；本次迁移不执行集群替换。

## 混合集群开发启动

原 Bash 入口及开关继续保留：

```bash
./start-cotriage.sh
./start-cotriage.sh --no-cloud
./start-cotriage.sh --no-web
./start-cotriage.sh --with-avatar
```

此入口会检查集群、**清空目标 Redis 数据库**、重启采集器并建立端口转发，
然后构建和运行 Java 服务。只对预期的开发集群使用此命令。
日志位于 `.runtime/logs`；Ctrl-C 停止该入口拥有的本地进程。

## 迁移范围与证据

来源为 CoTriage-dev `c04802d3ea9fadb765d3a0d9dc6d87202330fc21`。
原目录完整保留；Nova 是独立 Git 仓库，不继承原 origin。

- [迁移说明、已知限制和验证方式](migration/README.md)
- [本次验收记录：120 项测试、五个镜像和文件完整性](migration/validation-report.md)
- [原始文件与 SHA-256 清单](migration/source-manifest.json)
- [旧 Go 实际 MySQL schema](migration/fixtures/go-schema.sql)

本轮按当前行为迁移，保留已知可靠性限制，未重设计 RAISE。
`direct-mode`、`backend-mode`、`platform-integrations`、
`cotrige-archive` 及历史材料保留原样，其中的 Go 示例不属于五个 Java 核心服务。
历史实验报告不能作为 Java 版本的性能或模型质量结论。
