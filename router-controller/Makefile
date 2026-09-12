BINARY=router-controller
IMAGE?=router-controller:dev

.PHONY: test run build docker-build

test:
	go test ./...

run:
	go run ./cmd/router-controller

build:
	go build -o bin/$(BINARY) ./cmd/router-controller

docker-build:
	docker build -t $(IMAGE) .
