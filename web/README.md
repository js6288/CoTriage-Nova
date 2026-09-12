# CoTriage Web

基于 `design_doc/web.md` 的 MVP 前端：Vue 3 + Vite + Ant Design Vue。

## 功能

- 首页：欢迎页 + 进入按钮
- 管理台：左侧导航 + 右侧内容
- 设备管理：设备列表（node_name/equipment/status）
- 设备详情：展示完整字段（通过 list 接口前端过滤，后续可补 GET /devices/:node）

## 开发说明

- 默认通过 Vite 代理把 `/api` 转发到 `http://127.0.0.1:8080`（对应本地 server）。
- 如果你的 server 监听地址不是 `:8080`，请修改 `vite.config.ts` 的 proxy。

## Spatius 数字人 PoC

管理台中的“AI 产品主持人”页面使用 Spatius Direct Mode 做最小集成验证：

1. 复制 `web/.env.example` 为 `web/.env.local`；
2. 填入 `VITE_SPATIUS_APP_ID`、`VITE_SPATIUS_AVATAR_ID`、`VITE_SPATIUS_SESSION_TOKEN`；
3. 执行 `npm run dev` 后进入 `/manage/avatar-host`；
4. 点击“连接数字人”，再点击“播放 PoC 讲解”。

当前 PoC 只验证浏览器端数字人渲染、Motion Server 连接和 PCM 示例音频播报。Session Token 有有效期，过期后需要重新签发。

## LiveKit 运维值守数字人

分诊大盘内的“运维值守数字人”使用 Spatius LiveKit Agents 集成方式，支持实时语音对话。前端会请求 LiveKit token endpoint，连接 LiveKit room，渲染 Spatius 数字人，并通过麦克风与 Agent 对话。

前端需要配置：

```bash
VITE_SPATIUS_APP_ID=your_spatius_app_id
VITE_SPATIUS_AVATAR_ID=your_spatius_avatar_id
VITE_SPATIUS_REGION=us-west
VITE_LIVEKIT_TOKEN_ENDPOINT=http://127.0.0.1:18090/token
VITE_LIVEKIT_ROOM_NAME=cotriage-ops-watch
```

后端可参考 `platform-integrations/livekit-agents-demo/livekit-agent-quickstart/backend`：

```bash
cd platform-integrations/livekit-agents-demo/livekit-agent-quickstart/backend
uv run token_server.py
uv run agent.py dev
```

官方 `token_server.py` 默认监听 `8080`，容易与 CoTriage server 冲突。建议把 token server 改到 `18090`，并让 `VITE_LIVEKIT_TOKEN_ENDPOINT` 指向 `http://127.0.0.1:18090/token`。
