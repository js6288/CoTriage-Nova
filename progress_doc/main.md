Progress Report

2026-5-27
1. 在集群上部署redis与mysql。
2. 下载日志数据集。下载了Apache和Linux两种数据集，模拟不同产线设备的日志。
3. 在worker节点上运行replayer进程，模拟设备写入日志。
4. 完成log-collector开发工作，并在集群中以daemonset形式部署。
5. 新增了server，完成了设备管理接口的开发，但是有两个参数没有返回值，初步来看好像是log-collector没有再往redis streams中写消息了，需要看一下是怎么回事。

2026-5-29
1. 修复了ListDevices中last_event_time和ingest_rate_1m参数返回异常的问题，经排查发现是出现了时间漂移的问题，重新修改了时间戳的获取逻辑，以最新的一条events记录作为基准，而非使用本地时间作为基准。

2026-6-1
1. 今天把edge-infer的prompt给调试好了，但是对于edge-infer的形态还是没有想清楚，以及它跟server和web的关系也没有理清楚。
2. 今天把llama-server以API的形式暴露出来的，之后可以通过调API的方式来调用本地LLM，但是可能还得再看一下如何在集群里边调用。

2026-6-22
1. 新增仓库根目录 `.gitignore`，统一忽略操作系统与 IDE 文件、环境变量、证书密钥、Kubernetes 本地访问文件、临时目录、调试二进制、Go/Node/Python 缓存和本地数据库文件。
2. 将 `.vscode/`、`kind-host`、`charts/mysql/secret.yaml` 和 `web/tsconfig.tsbuildinfo` 从 Git 索引移除，但保留本地文件，避免本地调试配置、集群凭据、数据库 Secret 和生成缓存继续上传。
3. 使用 `git check-ignore` 验证上述文件均已命中根级忽略规则，并确认 Git 当前不再跟踪已识别的 kubeconfig、Secret、证书或密钥候选文件。
4. 将 `script/forward-datastores.sh` 改为常驻端口巡检脚本，默认每 60 秒使用 TCP 检查 MySQL 3306 和 Redis 6379；单个端口不可达时，仅重建对应 Service 的 port-forward。
5. 脚本支持通过 `CHECK_INTERVAL`、`MYSQL_LOCAL_PORT`、`REDIS_LOCAL_PORT`、`NAMESPACE` 和 `KUBECONFIG_PATH` 覆盖默认值，启动后进行就绪重试，退出时只清理由脚本自身创建的转发进程。

2026-8-8
1. 完成 `server`、`edge-infer`、`router-controller`、`cloud-gateway` 和 `web` 的 Linux AMD64 镜像构建，并推送至阿里云镜像仓库，统一版本为 `v0.0.1-202608081730`。
2. 为上述五个组件补齐 Kubernetes Deployment，其中 `server` 和 `web` 同时配置 ClusterIP/NodePort Service；全部部署在 `cotriage` namespace，副本数均为 1。
3. 集群内五个 Deployment 均已滚动发布成功并达到 `1/1 Ready`；验证 Web 反向代理、Dashboard API、边缘配置加载、Redis Stream 推理与路由链路正常，DLQ 为 0。
4. 完成上云分析结构化诊断接口与 Web 页面展示，并在 cloud-gateway 配置拉取接口增加 Bearer Token 鉴权，Token 通过 Kubernetes Secret 注入，避免云端供应商密钥经 Web 代理泄露。
5. Web NodePort 配置为 `30517`，但当前 Kind 集群创建时未将该端口映射到物理服务器，因此集群内访问正常、通过 `100.85.92.31:30517` 暂不可达；后续需要在宿主机增加端口转发或反向代理。
