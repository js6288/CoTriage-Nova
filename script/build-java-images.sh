#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${IMAGE_PREFIX:-cotriage-nova}"
TAG="${IMAGE_TAG:-java25}"
cd "$ROOT"
for service in log-collector edge-infer router-controller cloud-gateway server; do
  docker build --file "$service/Dockerfile" --tag "$PREFIX/$service:$TAG" .
done
