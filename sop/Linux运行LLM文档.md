# Linux 服务器运行 LLM SOP

## 1. 文档目的

本文档用于在 Linux 服务器上通过 `llama.cpp` 启动轻量 LLM 服务，为 CoTriage 的 `edge-infer` 提供远程小模型诊断能力。

当前用途：

- DistilBERT 在边缘侧完成 `NORMAL/ABNORMAL` 初筛；
- 当日志被判定为异常或不确定时，`edge-infer` 调用 Linux 服务器上的小模型；
- 小模型负责细粒度诊断、证据提取和简短处置建议；
- 后续云端大模型只处理高风险、高不确定或跨事件关联场景。

## 2. 当前服务器环境

llama.cpp 目录：

```bash
~/llama.cpp
```

可执行文件目录：

```bash
~/llama.cpp/build/bin
```

已确认存在：

```bash
~/llama.cpp/build/bin/llama-server
```

模型文件已移动到 `/data` 目录：

```bash
/data/pangjian/llama.cpp/models/
```

当前可用 Qwen 模型包括：

```text
qwen1_5-1_8b-chat-q4_k_m.gguf
qwen1_5-1_8b-chat-q5_k_m.gguf
qwen1_5-1_8b-chat-q6_k.gguf
qwen1_5-1_8b-chat-q8_0.gguf
qwen1_5-1_8b-chat-q4_0.gguf
qwen1_5-1_8b-chat-q3_k_m.gguf
qwen1_5-1_8b-chat-q2_k.gguf
```

推荐第一版使用：

```bash
/data/pangjian/llama.cpp/models/qwen1_5-1_8b-chat-q4_k_m.gguf
```

原因：

- 内存占用比 `q8_0`、`q6_k` 更低；
- 效果通常比 `q2_k`、`q3_k_m` 更稳；
- 适合作为 edge-infer 的第一版远程小模型。

## 3. 前置检查

进入 llama.cpp 目录：

```bash
cd ~/llama.cpp
```

确认 server 可执行文件存在：

```bash
ls build/bin/llama-server
```

确认模型文件存在：

```bash
ls /data/pangjian/llama.cpp/models/qwen1_5-1_8b-chat-q4_k_m.gguf
```

如果忘记模型位置，可重新搜索：

```bash
find /data /mnt /home -name "*.gguf" 2>/dev/null
```

## 4. 前台启动服务

无 GPU 或不确定 GPU 环境时，优先使用 CPU 启动：

```bash
cd ~/llama.cpp

./build/bin/llama-server \
  -m /data/pangjian/llama.cpp/models/qwen1_5-1_8b-chat-q4_k_m.gguf \
  --host 0.0.0.0 \
  --port 18081 \
  -c 4096 \
  -ngl 0
```

参数说明：

| 参数 | 含义 |
| --- | --- |
| `-m` | GGUF 模型文件路径 |
| `--host 0.0.0.0` | 监听所有网卡，允许 Mac 访问 |
| `--port 18081` | 服务端口 |
| `-c 4096` | 上下文长度 |
| `-ngl 0` | 不使用 GPU，全部走 CPU |

如果服务器有 NVIDIA GPU，可尝试：

```bash
cd ~/llama.cpp

./build/bin/llama-server \
  -m /data/pangjian/llama.cpp/models/qwen1_5-1_8b-chat-q4_k_m.gguf \
  --host 0.0.0.0 \
  --port 18081 \
  -c 4096 \
  -ngl 32
```

如果启动失败或显存不足，退回：

```bash
-ngl 0
```

## 5. 健康检查

在 Linux 服务器本机执行：

```bash
curl http://127.0.0.1:18081/health
```

正常情况下应返回健康状态信息。

检查端口监听：

```bash
ss -lntp | grep 18081
```

期望看到：

```text
0.0.0.0:18081
```

如果只监听 `127.0.0.1:18081`，Mac 无法直接访问，需要确认启动参数中使用了：

```bash
--host 0.0.0.0
```

## 6. OpenAI 兼容接口测试

llama-server 提供 OpenAI 兼容接口。测试命令：

```bash
curl http://127.0.0.1:18081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen1.5-1.8b-chat",
    "messages": [
      {
        "role": "system",
        "content": "你是一个日志故障诊断助手。请只输出JSON。"
      },
      {
        "role": "user",
        "content": "请诊断这条日志：KERNEL FATAL rts panic! - stopping execution"
      }
    ],
    "temperature": 0.1,
    "max_tokens": 256
  }'
```

期望模型返回诊断文本。后续接入 CoTriage 时，应通过 prompt 约束模型输出 JSON，例如：

```json
{
  "edge_pred": "INFRA_ERROR",
  "severity": 80,
  "confidence": 0.7,
  "evidence": ["rts panic", "stopping execution"],
  "advice_code": "CHECK_SYSTEM_RUNTIME",
  "advice": "检查系统运行时、内核错误和近期节点状态。"
}
```

## 7. 后台运行

如果需要后台常驻：

```bash
cd ~/llama.cpp

nohup ./build/bin/llama-server \
  -m /data/pangjian/llama.cpp/models/qwen1_5-1_8b-chat-q4_k_m.gguf \
  --host 0.0.0.0 \
  --port 18081 \
  -c 4096 \
  -ngl 0 \
  > /data/pangjian/llama-server.log 2>&1 &
```

查看日志：

```bash
tail -f /data/pangjian/llama-server.log
```

查找进程：

```bash
ps aux | grep llama-server
```

停止服务：

```bash
pkill -f "llama-server.*18081"
```

## 8. Mac 侧连通性检查

在 Mac 上执行：

```bash
curl http://<Linux服务器IP>:18081/health
```

再测试 chat 接口：

```bash
curl http://<Linux服务器IP>:18081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen1.5-1.8b-chat",
    "messages": [
      {
        "role": "system",
        "content": "你是一个日志故障诊断助手。请只输出JSON。"
      },
      {
        "role": "user",
        "content": "请诊断这条日志：connection refused to mysql:3306"
      }
    ],
    "temperature": 0.1,
    "max_tokens": 256
  }'
```

如果 Mac 无法访问：

1. 确认 Linux 上 `llama-server` 使用 `--host 0.0.0.0`；
2. 确认 Linux 防火墙或安全组放行 `18081`；
3. 确认 Mac 使用的是 Linux 服务器可达 IP；
4. 在 Linux 本机执行 `ss -lntp | grep 18081` 检查监听状态。

## 9. edge-infer 后续接入地址

后续 edge-infer 应接入：

```text
http://<Linux服务器IP>:18081/v1/chat/completions
```

建议新增环境变量：

```bash
LLM_URL=http://<Linux服务器IP>:18081/v1/chat/completions
LLM_TIMEOUT=3s
LLM_ENABLED=true
```

调用策略建议：

- `DistilBERT -> NORMAL`：不调用小模型；
- `DistilBERT -> ABNORMAL`：调用小模型做细粒度诊断；
- 小模型返回高置信结果：输出 `APP_ERROR / NET_ERROR / DISK_ERROR / INFRA_ERROR / SECURITY`；
- 小模型超时或输出不合法：保留 `ABNORMAL` 或降级为 `FALLBACK/UNKNOWN`；
- 高严重度、高不确定或跨事件关联场景再交给云端大模型。

## 10. 常见问题

### 10.1 `find models -name "*.gguf"` 找不到模型

模型可能被移动到了 `/data`。执行：

```bash
find /data /mnt /home -name "*.gguf" 2>/dev/null
```

### 10.2 找不到 `llama-server`

确认是否已经编译：

```bash
ls ~/llama.cpp/build/bin/llama-server
```

如果不存在，重新编译：

```bash
cd ~/llama.cpp
cmake -B build
cmake --build build -j
```

### 10.3 启动后 Mac 访问不到

优先检查：

```bash
ss -lntp | grep 18081
```

如果不是 `0.0.0.0:18081`，重新用以下参数启动：

```bash
--host 0.0.0.0
```

### 10.4 内存不足

按资源消耗从低到高尝试：

```text
qwen1_5-1_8b-chat-q2_k.gguf
qwen1_5-1_8b-chat-q3_k_m.gguf
qwen1_5-1_8b-chat-q4_0.gguf
qwen1_5-1_8b-chat-q4_k_m.gguf
qwen1_5-1_8b-chat-q5_k_m.gguf
qwen1_5-1_8b-chat-q6_k.gguf
qwen1_5-1_8b-chat-q8_0.gguf
```

优先推荐：

```text
qwen1_5-1_8b-chat-q4_k_m.gguf
```

如果仍然内存不足，降低上下文长度：

```bash
-c 2048
```
