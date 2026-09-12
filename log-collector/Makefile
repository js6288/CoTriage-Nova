# log-collector 镜像构建与推送（多架构）
#
# 依赖：
# - Docker Desktop / docker engine
# - buildx（docker buildx 版本）
#
# 用法示例：
#   make login
#   make push VERSION=v0.1.0
#
# 说明：push 使用 buildx 一次性构建并推送 multi-arch manifest。

REGISTRY ?= crpi-fq916lepz6l0u7gr.cn-guangzhou.personal.cr.aliyuncs.com
NAMESPACE ?= pangjian001
IMAGE_NAME ?= log-collector
IMAGE ?= $(REGISTRY)/$(NAMESPACE)/$(IMAGE_NAME)

# 镜像版本号（建议显式传入）
VERSION ?= dev

PLATFORMS ?= linux/amd64,linux/arm64
BUILDER ?= cotriage-builder

.PHONY: help
help:
	@echo "Targets:"
	@echo "  login                Docker 登录阿里云个人仓库"
	@echo "  buildx-setup          创建/切换 buildx builder（支持多架构）"
	@echo "  build                本地构建单架构镜像（默认本机架构）"
	@echo "  push                 构建并推送多架构镜像 (VERSION=..., PLATFORMS=...)"
	@echo "  imagetools-inspect    查看已推送镜像的多架构 manifest"
	@echo "  tag                  （可选）本地镜像重新打 tag（适用于 build 之后手动 push）"

.PHONY: login
login:
	@echo "Logging in to $(REGISTRY) ..."
	docker login --username=$(NAMESPACE) $(REGISTRY)

.PHONY: buildx-setup
buildx-setup:
	@echo "Setting up buildx builder: $(BUILDER)"
	@docker buildx inspect $(BUILDER) >/dev/null 2>&1 || docker buildx create --name $(BUILDER) --use
	docker buildx use $(BUILDER)
	@# 确保 QEMU 可用（在 Docker Desktop 上通常已自带；失败也不影响 amd64 本地 build）
	@docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null 2>&1 || true
	@docker buildx inspect --bootstrap >/dev/null

.PHONY: build
build:
	@echo "Building local image $(IMAGE):$(VERSION) ..."
	docker build -t $(IMAGE):$(VERSION) .

.PHONY: push
push: buildx-setup
	@echo "Building + pushing multi-arch image $(IMAGE):$(VERSION) for $(PLATFORMS) ..."
	docker buildx build \
		--platform $(PLATFORMS) \
		-t $(IMAGE):$(VERSION) \
		--push \
		.

.PHONY: imagetools-inspect
imagetools-inspect:
	docker buildx imagetools inspect $(IMAGE):$(VERSION)

# （可选）适用于你先 build 得到本地镜像后，再按阿里云指引手动 tag/push
.PHONY: tag
tag:
	@echo "Tagging local image to $(IMAGE):$(VERSION) ..."
	@echo "Example: docker tag <local-image-id-or-name> $(IMAGE):$(VERSION)"
