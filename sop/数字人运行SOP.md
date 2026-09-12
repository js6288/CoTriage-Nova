# CoTriage 数字人运行 SOP

本文档用于本地启动和验证 CoTriage 中的两个数字人场景：

- AI 产品主持人：位于管理台 `/manage/avatar-host`，使用 Spatius Direct Mode，播放预置 PCM 音频完成产品介绍。
- 运维值守数字人：位于分诊大盘，使用 LiveKit Agents + Spatius Avatar，支持麦克风实时语音对话。

两者使用同一套前端页面，但底层链路不同。Direct Mode 依赖 `VITE_SPATIUS_SESSION_TOKEN`；运维值守数字人依赖 LiveKit token server 和 agent worker。

## 1. 前置准备

确认本地已安装：

```bash
node -v
npm -v
uv --version
```

如果要重新生成产品介绍音频，还需要安装 `ffmpeg`。

```bash
ffmpeg -version
```

## 2. 配置前端环境变量

前端环境变量位于 `web/.env.local`。如果文件不存在，先复制模板：

```bash
cd web
cp .env.example .env.local
```

至少需要配置以下字段：

```bash
VITE_SPATIUS_APP_ID=your_spatius_app_id
VITE_SPATIUS_AVATAR_ID=your_spatius_avatar_id
VITE_SPATIUS_SESSION_TOKEN=your_spatius_session_token
VITE_SPATIUS_REGION=us-west

VITE_LIVEKIT_TOKEN_ENDPOINT=http://127.0.0.1:18090/token
VITE_LIVEKIT_ROOM_NAME=cotriage-ops-watch
```

说明：

- `VITE_SPATIUS_SESSION_TOKEN` 只用于 AI 产品主持人的 Direct Mode。
- `VITE_LIVEKIT_TOKEN_ENDPOINT` 用于运维值守数字人，建议固定为 `http://127.0.0.1:18090/token`，避免和 CoTriage server 的 `8080` 端口冲突。
- 修改 `.env.local` 后需要重启 Vite 前端服务，否则浏览器不会读取新变量。

## 3. 启动 CoTriage 前端

```bash
cd web
npm install
npm run dev -- --host 127.0.0.1
```

启动成功后访问 Vite 输出的地址，通常是：

```text
http://127.0.0.1:5173
```

## 4. 运行 AI 产品主持人

AI 产品主持人是最简单的 PoC，只验证浏览器端数字人渲染、Direct Mode Motion Server 连接，以及 PCM 音频播报。

操作步骤：

1. 打开 `http://127.0.0.1:5173/manage/avatar-host`。
2. 确认右侧“配置检查”为绿色。
3. 点击“连线AI主持人”。
4. 状态变为 `Connected` 后，点击“播放产品介绍”。

成功现象：

- 页面状态显示 `Connected`。
- 数字人形象加载出来。
- 点击“播放产品介绍”后，数字人开始播报并进行口型同步。

常见问题：

- 如果显示缺少环境变量，检查 `web/.env.local` 是否配置了 `VITE_SPATIUS_APP_ID`、`VITE_SPATIUS_AVATAR_ID`、`VITE_SPATIUS_SESSION_TOKEN`。
- 如果显示连接失败，优先刷新页面后重试。因为运维值守数字人和 AI 产品主持人使用不同的 AvatarSDK 初始化模式，同一个浏览器页面里切换模式时可能需要刷新。
- 如果提示 Session Token 相关错误，重新签发 `VITE_SPATIUS_SESSION_TOKEN`，并确认 token 与 App ID、region 匹配。
- 如果能连接但不播报，检查 `web/public/audio/cotriage_avatar_intro.pcm` 是否存在，并确认音频格式为 PCM16 mono 16 kHz。

## 5. 运行运维值守数字人

运维值守数字人需要同时启动三个部分：

- LiveKit token server：给浏览器签发 room token，并调度 agent。
- LiveKit agent worker：接收麦克风音频，执行 VAD、STT、LLM、TTS，并驱动 Spatius Avatar。
- CoTriage Web：分诊大盘中的前端面板。

### 5.1 配置 cascade 后端环境变量

编辑：

```text
platform-integrations/livekit-agents-demo/livekit-agents-reference-demo/backend/cascade/.env
```

至少需要配置：

```bash
LIVEKIT_URL=wss://your-livekit-url
LIVEKIT_API_KEY=your_livekit_api_key
LIVEKIT_API_SECRET=your_livekit_api_secret

DEEPGRAM_API_KEY=your_deepgram_api_key
DEEPGRAM_LANGUAGE=zh-CN

LLM_MODEL=your_model_name
LLM_BASE_URL=your_openai_compatible_base_url
LLM_API_KEY=your_llm_api_key

CARTESIA_API_KEY=your_cartesia_api_key
CARTESIA_LANGUAGE=zh

SPATIUS_APP_ID=your_spatius_app_id
SPATIUS_AVATAR_ID=your_spatius_avatar_id
SPATIUS_API_KEY=your_spatius_api_key
SPATIUS_REGION=us-west
```

如果语音不容易被识别，可以降低 VAD 阈值：

```bash
VAD_ACTIVATION_THRESHOLD=0.55
```

### 5.2 启动 token server

打开一个终端：

```bash
cd platform-integrations/livekit-agents-demo/livekit-agents-reference-demo/backend/cascade
TOKEN_SERVER_PORT=18090 uv run token_server.py
```

成功现象：

```text
Token server running on http://localhost:18090
```

如果提示 `Address already in use`，说明端口被占用。可以改用其他端口，同时同步修改 `web/.env.local` 中的 `VITE_LIVEKIT_TOKEN_ENDPOINT`。

### 5.3 启动 agent worker

再打开一个终端：

```bash
cd platform-integrations/livekit-agents-demo/livekit-agents-reference-demo/backend/cascade
VAD_ACTIVATION_THRESHOLD=0.55 uv run agent.py dev
```

成功现象：

- 日志中没有 `LIVEKIT_URL environment variable` 之类的配置错误。
- token server 收到前端请求后，日志出现 `Successfully dispatched agent to room`。
- agent worker 日志出现 `Connecting to room`、`Avatar session started`。

### 5.4 在分诊大盘连接数字人

1. 打开 CoTriage Web。
2. 进入分诊大盘。
3. 在“运维值守数字人”面板点击“连接值守数字人”。
4. 连接成功后点击“开启麦克风”。
5. 对着麦克风说话，例如：“当前异常影响范围是什么？”

成功现象：

- 页面显示已连接或对话中。
- 浏览器弹出麦克风授权时选择允许。
- agent worker 日志出现 `User input transcribed`。
- 数字人通过语音回答，并在页面中进行口型同步。

## 6. 故障排查

### 6.1 浏览器一直等待输入，数字人不回答

按顺序检查：

1. token server 是否打印 `Successfully dispatched agent to room`。
2. agent worker 是否正在运行，并且没有 LiveKit 凭证错误。
3. agent worker 是否打印 `User input transcribed`。
4. 浏览器是否允许麦克风权限。
5. `DEEPGRAM_LANGUAGE` 是否适合当前输入语言，中文建议使用 `zh-CN`。
6. `VAD_ACTIVATION_THRESHOLD` 是否过高，建议先用 `0.55`。

如果没有 `User input transcribed`，问题通常在麦克风发布、VAD 或 STT。

如果有 `User input transcribed`，但没有回答，问题通常在 LLM 或 TTS。

如果 agent 日志显示已回答，但页面没有数字人播报，问题通常在 Spatius Avatar 或 LiveKit 音视频订阅。

### 6.2 AI 产品主持人连接 failed

优先刷新浏览器页面后重试。

原因是 AI 产品主持人使用 Direct Mode，运维值守数字人使用 Backend/LiveKit Mode。AvatarSDK 在同一个前端运行时中只能按一套配置初始化，切换模式后需要刷新页面重新初始化。

同时检查：

- `VITE_SPATIUS_SESSION_TOKEN` 是否过期。
- `VITE_SPATIUS_APP_ID` 是否与 Session Token 匹配。
- `VITE_SPATIUS_REGION` 是否为正确 region。
- 浏览器 DevTools Network 中是否有 Motion Server 连接失败。

### 6.3 token server 端口冲突

如果启动 token server 时出现：

```text
Address already in use
Port 8080 is in use by another program
```

使用 `18090` 启动：

```bash
TOKEN_SERVER_PORT=18090 uv run token_server.py
```

并确认 `web/.env.local`：

```bash
VITE_LIVEKIT_TOKEN_ENDPOINT=http://127.0.0.1:18090/token
```

### 6.4 修改环境变量后没有生效

前端 `.env.local` 是 Vite 启动时读取的。修改后必须重启：

```bash
cd web
npm run dev -- --host 127.0.0.1
```

Python 后端 `.env` 也是进程启动时读取的。修改 cascade `.env` 后，需要重启 `token_server.py` 和 `agent.py`。
