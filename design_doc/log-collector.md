log-collector 设计文档

raw event定义：
event_id：UUID（全局唯一，幂等键）
ts：采集时间（RFC3339）
equipment：设备名称，从节点的label中获取，如cotriage.io/equipment=LinuxServer
log.raw：这一行原始日志文本

Stream：cotriage.events.raw 
每条消息写两个字段就够：
event_id：字符串
payload：JSON 字符串

log-collector实现逻辑：
1. tail /var/log/cotriage/input/current.log
2. 每读到一行：
- 生成 event_id
- 组装 payload
- XADD 写入 Redis Stream

需要注入的环境变量：
1. kubeconfig：用于连接集群
2. LOG_PATH：/var/log/cotriage/input/current.log
3. REDIS_ADDR： redis:6379