#!/bin/sh
# Cross-compiles the static linux/amd64 binary that ./Dockerfile copies in.
# Run this (and commit the result) after any change under cmd/ or internal/,
# then rebuild the image: docker-compose build --no-cache server
set -eu
cd "$(dirname "$0")"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server-linux-amd64 ./cmd/server

echo "built server-linux-amd64:"
ls -la server-linux-amd64
