#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
expected=$(./scripts/source-hash.sh)
actual=$(./bin/blog-linux-amd64 source-hash)
if [ -z "$actual" ] || [ "$actual" != "$expected" ]; then
	echo 'The Linux generator does not match the Go source. Run ./scripts/build-linux.sh.' >&2
	exit 1
fi
exec ./bin/blog-linux-amd64 build-artifacts
