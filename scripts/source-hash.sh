#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

hash() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum | cut -d ' ' -f 1
	else
		shasum -a 256 | cut -d ' ' -f 1
	fi
}

{
	find . -type d \( -name .git -o -name node_modules -o -name dist -o -name .cloudflare -o -name .wrangler \) -prune -o \
		-type f -name '*.go' ! -name '*_test.go' -print
	printf '%s\n' ./go.mod ./go.sum ./scripts/build-linux.sh
} | LC_ALL=C sort | while IFS= read -r file; do
	printf '%s\000' "$file"
	hash < "$file"
done | hash
