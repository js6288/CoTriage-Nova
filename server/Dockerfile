FROM docker.xuanyuan.run/library/golang:1.22 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go env -w GOSUMDB=off && go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server

FROM docker.xuanyuan.run/library/alpine:3.20
RUN adduser -D -H -u 10001 app
USER 10001
COPY --from=builder /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
