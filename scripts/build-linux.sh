#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
hash=$(./scripts/source-hash.sh)
mkdir -p bin
tmp=$(mktemp "bin/blog-linux-amd64.XXXXXX")
trap 'rm -f "$tmp"' EXIT HUP INT TERM

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.27.3 \
	go build -trimpath -buildvcs=false -ldflags="-s -w -X main.sourceHash=$hash" -o "$tmp" .
chmod 755 "$tmp"
mv "$tmp" bin/blog-linux-amd64
