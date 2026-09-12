GO_COMPONENTS := cloud-gateway edge-infer log-collector router-controller server
IMAGE_PREFIX ?= cotriage
TAG ?= submission

.PHONY: test test-go build build-go build-web docker-build

test: test-go

test-go:
	@set -e; for component in $(GO_COMPONENTS); do \
		echo "==> testing $$component"; \
		(cd $$component && go test ./...); \
	done

build: build-go build-web

build-go:
	@set -e; for component in $(GO_COMPONENTS); do \
		echo "==> building $$component"; \
		case $$component in \
			log-collector) (cd $$component && go build ./...) ;; \
			*) (cd $$component && go build ./cmd/...) ;; \
		esac; \
	done

build-web:
	cd web && npm ci && npm run build

docker-build:
	@set -e; for component in $(GO_COMPONENTS) web; do \
		echo "==> docker build $(IMAGE_PREFIX)/$$component:$(TAG)"; \
		docker build -t $(IMAGE_PREFIX)/$$component:$(TAG) $$component; \
	done
