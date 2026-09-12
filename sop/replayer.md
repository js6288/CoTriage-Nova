Cotriage-Replayer 多节点日志文件部署 SOP（Kind 五节点集群）
1. 文档目的
本文档用于完成 Kind 五节点集群所有 Worker 节点日志数据源部署，为 cotriage-replayer（DaemonSet 日志回放组件） 提供标准数据源文件 source.log，解决节点日志文件缺失、回放失败问题，支撑系统日志压测、性能实验。
集群环境：Kind 5 节点集群（1控制节点 + 4Worker节点）
组件依赖：cotriage-replayer 固定读取路径 /var/log/cotriage/replay_src/source.log
2. 前置准备
1. 本地准备4份独立日志样本文件（对应4个Worker节点）：
- Apache_2k.log
- Linux_2k.log
- Hadoop_2k.log
- Zookeeper_2k.log
2. 集群节点名称：host-5-worker、host-5-worker2、host-5-worker3、host-5-worker4
3. 目标统一路径：/var/log/cotriage/replay_src/
3. 标准部署步骤（逐节点执行）
核心流程：节点创建目录 → 本地日志文件拷贝至节点 → 重命名为标准数据源 source.log
3.1 节点 host-5-worker（挂载 Apache 日志）
# 1. 节点创建存放目录
docker exec host-5-worker mkdir -p /var/log/cotriage/replay_src

# 2. 本地日志拷贝至节点目录
docker cp Apache_2k.log host-5-worker:/var/log/cotriage/replay_src/

# 3. 进入节点终端重命名为标准文件
docker exec -it host-5-worker /bin/bash

# 节点内执行
cd /var/log/cotriage/replay_src/
mv Apache_2k.log source.log
ls # 校验生成 source.log
exit
3.2 节点 host-5-worker2（挂载 Linux 日志）
docker exec host-5-worker2 mkdir -p /var/log/cotriage/replay_src
docker cp Linux_2k.log host-5-worker2:/var/log/cotriage/replay_src/
docker exec -it host-5-worker2 /bin/bash

cd /var/log/cotriage/replay_src/
mv Linux_2k.log source.log
ls
exit
3.3 节点 host-5-worker3（挂载 Hadoop 日志）
docker exec host-5-worker3 mkdir -p /var/log/cotriage/replay_src
docker cp Hadoop_2k.log host-5-worker3:/var/log/cotriage/replay_src/
docker exec -it host-5-worker3 /bin/bash

cd /var/log/cotriage/replay_src/
mv Hadoop_2k.log source.log
ls
exit
3.4 节点 host-5-worker4（挂载 Zookeeper 日志）
docker exec host-5-worker4 mkdir -p /var/log/cotriage/replay_src
docker cp Zookeeper_2k.log host-5-worker4:/var/log/cotriage/replay_src/
docker exec -it host-5-worker4 /bin/bash

cd /var/log/cotriage/replay_src/
mv Zookeeper_2k.log source.log
ls
exit
4. 全局校验步骤（必做）
4.1 校验所有节点文件部署成功
逐个进入节点查看是否存在 source.log 文件，确保无缺失：
# 示例校验单节点
docker exec -it host-5-worker ls /var/log/cotriage/replay_src/
所有节点均输出：source.log 即为部署成功。
4.2 校验 Replayer 组件正常回放
# 查看回放日志，无 missing 报错、正常输出启动日志即为正常
kubectl logs -n cotriage daemonset/cotriage-replayer
正常日志特征：[replayer] start: xxx SRC=/var/log/cotriage/replay_src/source.log
5. 关键规则说明
- 文件名强制要求：replayer 代码硬读取 source.log，必须统一重命名，否则报文件缺失错误；
- 多节点差异化日志：4个Worker节点分别挂载不同组件日志，模拟集群多类型业务日志流量，丰富实验场景；
- 临时部署特性：本方式为 docker 临时拷贝，Kind 集群删除重建后文件丢失，长期使用建议配置 Kind 目录持久挂载；
- DaemonSet 特性：每个 Worker 节点独立读取本地日志，实现全集群分布式日志回放压测。
6. 常见报错规避
- docker exec 缺少参数：必须携带-it 参数 + 终端命令（/bin/bash）；
- 文件不存在报错：未执行目录创建命令 mkdir -p，或拷贝路径错误；
- 回放提示 missing source file：节点未完成重命名，文件名称不匹配。