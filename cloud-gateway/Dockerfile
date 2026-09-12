FROM docker.xuanyuan.run/library/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cloud-gateway ./cmd/cloud-gateway
FROM docker.xuanyuan.run/library/alpine:3.20
RUN adduser -D -H -u 10001 app
USER 10001
COPY --from=build /cloud-gateway /cloud-gateway
ENTRYPOINT ["/cloud-gateway"]
