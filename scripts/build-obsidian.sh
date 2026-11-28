#!/bin/sh
set +x
set -eu

fail() {
	printf '%s\n' "$1" >&2
	exit 1
}

cd "$(dirname "$0")/.."
expected=$(./scripts/source-hash.sh)
actual=$(./bin/blog-linux-amd64 source-hash)
if [ -z "$actual" ] || [ "$actual" != "$expected" ]; then
	fail 'The Linux generator does not match the Go source. Run ./scripts/build-linux.sh.'
fi

cli="$PWD/node_modules/.bin/ob"
[ -x "$cli" ] || fail 'Install Obsidian Headless with npm ci.'
: "${OBSIDIAN_AUTH_TOKEN:?Set OBSIDIAN_AUTH_TOKEN before the build.}"
: "${OBSIDIAN_VAULT:?Set OBSIDIAN_VAULT before the build.}"
export OBSIDIAN_AUTH_TOKEN
content=${OBSIDIAN_CONTENT_DIR:-blog-content}
case "/$content/" in
	*//*|*/../*|*/./*) fail 'Set OBSIDIAN_CONTENT_DIR to a relative folder within the vault.' ;;
esac

scratch=$(mktemp -d "${TMPDIR:-/tmp}/blog-vault.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 1' HUP INT TERM
state=$(cd "$scratch" && pwd -P)
vault="$state/vault"

# Keep the sync state out of the checkout and private vault paths out of build logs.
run_sync_command() {
	HOME="$state" XDG_CONFIG_HOME="$state/config" "$@" </dev/null >/dev/null 2>&1
}

set -- sync-setup "--vault=$OBSIDIAN_VAULT" --path "$vault" --device-name 'plain-blog build' --json
if [ -n "${OBSIDIAN_VAULT_PASSWORD:-}" ]; then
	set -- "$@" "--password=$OBSIDIAN_VAULT_PASSWORD"
fi
unset OBSIDIAN_VAULT_PASSWORD
run_sync_command "$cli" "$@" || fail 'Could not set up Obsidian Sync. Check the vault, auth token, and vault password.'
set --
run_sync_command "$cli" sync-config --path "$vault" --mode pull-only --json || fail 'Could not set Obsidian Sync to pull-only mode.'
printf '%s\n' 'Obsidian Sync download started.'
# Headless Sync can retry connection failures until it receives a stop signal.
run_sync_command timeout -k 5s 15m "$cli" sync --path "$vault" || fail 'Could not download the Obsidian vault. The download limit is 15 minutes.'
unset OBSIDIAN_AUTH_TOKEN

if ! content_path=$(cd "$vault/$content" 2>/dev/null && pwd -P) || [ "$content_path" != "$vault/$content" ]; then
	fail 'Blog content folder not found in the synced vault. Check OBSIDIAN_CONTENT_DIR.'
fi
printf '%s\n' 'Obsidian Sync download complete.'
BLOG_CONTENT_DIR="$content_path" ./bin/blog-linux-amd64 build
