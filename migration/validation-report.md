# Java 25 迁移验收记录

验收日期：2026-09-12（Asia/Shanghai）。本记录对应五个核心服务的 Java 实现。
原始基线：CoTriage-dev `c04802d3ea9fadb765d3a0d9dc6d87202330fc21`。

## 完成范围

- Nova 为独立 Git 工程，开发分支为 `codex/java25-migration`。
- 五个顶层服务的 103 个旧 Go 源码及 Go 模块构建文件已移除，由 Java 实现替代。
- 包含 Maven 父工程、公共兼容层、五个可独立运行的 Boot JAR、端到端测试模块。
- 保留 Vue、Python、数字人示例、历史归档及业务材料。示例和归档中的 Go 不作重构。
- 当前运行文档、Bash/Makefile、五个 Dockerfile、Kubernetes 镜像与 JVM 资源配置已同步。

## 测试结果

完整命令：`.\mvnw.cmd -B -ntp verify`，2026-09-12 14:20:28 完成，**BUILD SUCCESS**。

| 模块 | 测试数 | 失败 | 错误 | 跳过 |
| --- | ---: | ---: | ---: | ---: |
| cotriage-common | 3 | 0 | 0 | 0 |
| log-collector | 4 | 0 | 0 | 0 |
| edge-infer | 54 | 0 | 0 | 0 |
| router-controller | 18 | 0 | 0 | 0 |
| cloud-gateway | 17 | 0 | 0 | 0 |
| server | 22 | 0 | 0 | 0 |
| migration-tests | 2 | 0 | 0 | 0 |
| **合计** | **120** | **0** | **0** | **0** |

逐测试集结果：[java-test-results.json](evidence/java-test-results.json)。

端到端测试启动五个真实 Java 进程、隔离 MySQL 8.4/Redis 7.4，以及确定性的
Kubernetes/模型 HTTP 桩，实际执行全部 **47 个 HTTP 路由**。验证包括：

- 原日志内容被采集器跳过，新增日志按原 schema 发布。
- `raw → inferred → LOCAL`、`CLOUD → cloud_feedback`、
  `DEFER → cloud_outbox → Redis 回放 → cloud_feedback`。
- 两次模型诊断、两条 NET_ERROR 反馈及 router 处理标记。
- 两条真实 HTTP SSE 流：`inferred` 和 `routed`，包含事件 ID 和 JSON 数据。
- 配置 CRUD、激活、PATCH、ETag/304、可选 Bearer、软删除冲突及管理查询。

完整执行清单：[end-to-end.json](evidence/end-to-end.json)。
数据库兼容测试从旧 Go 生成的 12 张表开始，补回缺失列与命名索引后，
确认原记录保留、无额外业务表：[schema-compatibility.json](evidence/schema-compatibility.json)。

旧 Go 使用隔离 Go 1.22.12 执行五个模块测试成功；collector 没有原单测。
原 server 的两个 MySQL 测试另外在独立数据库执行成功，未使用现有业务数据库。
参见 [Go 基线结果](evidence/go-baseline-results.json) 和
[原 MySQL 测试输出](evidence/go-server-mysql-tests.txt)。

## 镜像、部署与前端

五个镜像均从当前 Java 源码构建成功，并在隔离容器网络实际启动。
默认标签为 `cotriage-nova/<service>:java25`。
所有容器的 JVM 参数为 `-Xms64m -Xmx256m`，内存上限为 512 MiB；
Kubernetes 内存 request 为 256 MiB。

2026-09-12 14:23:38 启动检查时的单次低负载容器内存采样：

| 服务 | Docker 内存采样 | 容器上限 |
| --- | ---: | ---: |
| log-collector | 178.1 MiB | 512 MiB |
| edge-infer | 153.1 MiB | 512 MiB |
| router-controller | 164.1 MiB | 512 MiB |
| cloud-gateway | 150.6 MiB | 512 MiB |
| server | 209.8 MiB | 512 MiB |

这是启动检查时的采样，不是峰值、压力测试或性能提升结论。
镜像中设备、事件、配置、路由状态和赛事指标接口均返回 HTTP 200。
检查结束后已移除本次临时容器和网络，构建的镜像保留。
参见 [镜像构建结果](evidence/image-build-results.json) 和
[镜像启动及内存记录](evidence/image-smoke.json)。

- Kubernetes：kubeconform 0.6.7、Kubernetes schema 1.30.3，9 个资源全部有效，无跳过。
- Bash：启动脚本和镜像构建脚本语法检查通过。
- Vue：`npm ci` 和 `npm run build` 通过；Vite 保留原有大 chunk 提示，本轮未改前端打包结构。
- 未推送镜像，未发布 GitHub，未连接或替换已有 Kubernetes 集群。

## 完整性和兼容边界

原目录的 **1,452 个受版本控制文件** SHA-256 全部保持原值。
Nova 中排除明确重构/更新文件后的 **1,324 个保留文件**全部哈希一致。
原目录仍只有已有的 Flutter Android 构建目录未跟踪。
参见 [source-files.json](evidence/source-files.json) 和
[preserved-files.json](evidence/preserved-files.json)。

已知 Go 可靠性限制按要求保留，详见 [迁移说明](README.md)。
本轮没有新增 PEL 恢复、跨步骤事务、云调用租约去重或改写 RAISE。
PATCH null、激活响应的旧提交快照、错误类型事件过滤等按原 Go 实测保持。
Java 的 Servlet 缓冲刷新已适配，使真实 HTTP SSE 能及时发送数据。

Schema 初始化支持缺表、缺列、缺命名索引的非破坏性补齐；不自动重写既有列类型。
这次通过的旧数据兼容目标为所记录的 c04802d 表结构。
真实模型质量、长时间压力测试和真实集群部署均未纳入本次结果。
