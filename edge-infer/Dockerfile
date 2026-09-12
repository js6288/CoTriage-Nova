FROM docker.xuanyuan.run/library/golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/edge-infer ./cmd/edge-infer

FROM docker.xuanyuan.run/library/alpine:3.20
RUN adduser -D -H -u 10001 app
USER 10001
COPY --from=builder /out/edge-infer /edge-infer
ENTRYPOINT ["/edge-infer"]
