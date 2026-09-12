# CoTriage-dev

CoTriage development workspace for log collection, edge inference, server APIs, web dashboard, and the RAISE router-controller.

## One-command startup

The root startup script checks the cluster infrastructure, clears Redis, restarts
the cluster `cotriage-log-collector`, creates the required port-forwards, and
starts the local application components:

```bash
./start-cotriage.sh
# or
make up
```

The digital-human/LiveKit processes are optional and are not started by default:

```bash
./start-cotriage.sh --with-avatar
# or
make up-avatar
```

Useful alternatives:

```bash
./start-cotriage.sh --no-cloud
./start-cotriage.sh --no-web
./start-cotriage.sh --help
```

Local process logs are stored under `.runtime/logs`. Keep the script in the
foreground and press `Ctrl-C` to stop every process and port-forward it started.
An optional root `.env` can provide values such as `EDGE_LLM_URL`. Cloud provider
API keys are entered and managed directly in the Web management page. Cold Go builds are allowed up to 180 seconds by default; override
this with `STARTUP_TIMEOUT` when needed. Remote MySQL migrations use a three-minute
local-start timeout and can be adjusted with `SERVER_MIGRATION_TIMEOUT` and
`ROUTER_MIGRATION_TIMEOUT`. The avatar option requires the cascade `.env` documented in
[`sop/数字人运行SOP.md`](sop/数字人运行SOP.md).

## Router Controller

- Design: [design_doc/router_design.md](design_doc/router_design.md)
- Development plan and progress: [progress_doc/router_dev_plan.md](progress_doc/router_dev_plan.md)
- Runtime README: [router-controller/README.md](router-controller/README.md)
- Local/Kubernetes SOP: [sop/router-controller运行SOP.md](sop/router-controller运行SOP.md)
- Kubernetes deployment: [charts/router-controller/deployment.yaml](charts/router-controller/deployment.yaml)

Quick local path:

```bash
./script/forward-datastores.sh
cd router-controller
REDIS_ADDR=127.0.0.1:6379 \
MYSQL_DSN='cotriage:cotriage-pass@tcp(127.0.0.1:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC' \
go run ./cmd/router-controller
```

在本地终端执行：
KUBECONFIG=/Users/pangjian/researchProjects/cotriage-dev/kind-host \
kubectl port-forward -n cotriage service/web 8080:80
保持该终端窗口运行，然后浏览器访问：
http://localhost:8080

在本地通过tailscale直接访问
open -na "Google Chrome" --args \
  --user-data-dir=/tmp/cotriage-chrome-direct \
  --no-proxy-server \
  --disable-extensions \
  http://100.85.92.31:8080/dashboard