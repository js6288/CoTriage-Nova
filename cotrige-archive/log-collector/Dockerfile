FROM docker.xuanyuan.run/library/golang:1.22 AS builder
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

# 在受限网络环境下避免访问 sum.golang.org；必要时可在 buildx 时覆盖
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

COPY go.mod ./
# 单文件项目，先下载依赖
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/log-collector ./

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /out/log-collector /log-collector

USER nonroot:nonroot
ENTRYPOINT ["/log-collector"]
