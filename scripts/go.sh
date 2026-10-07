#!/usr/bin/env bash
# Runs the Go toolchain inside the dev image. Usage: scripts/go.sh test ./...
# The Docker socket is mounted so testcontainers-go can start Postgres next to the dev container.
set -euo pipefail
cd "$(dirname "$0")/.."
docker build -q -t briefklar-dev -f dev.Dockerfile . > /dev/null
MSYS_NO_PATHCONV=1 docker run --rm \
  -v "$PWD:/src" -w /src \
  -v briefklar-gomod:/go/pkg/mod -v briefklar-gocache:/root/.cache/go-build \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal \
  -e TESTCONTAINERS_RYUK_DISABLED=true \
  --add-host host.docker.internal:host-gateway \
  briefklar-dev go "$@"
