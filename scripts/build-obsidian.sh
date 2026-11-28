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

# Keep private vault paths out of build logs.
run_sync_command() {
	"$@" </dev/null >/dev/null
}

# Report only fixed diagnostics. Headless errors can contain vault details.
fail_sync_setup() {
	status=$1
	log=$2
	match=$(grep -Eo 'Wrong vault key|Password not provided|Not logged in|No account logged in|Multiple vaults named|^Vault ".*" not found\.|HTTP Error [1-5][0-9][0-9]|UNABLE_TO_VERIFY_LEAF_SIGNATURE|UNABLE_TO_GET_ISSUER_CERT(_LOCALLY)?|SELF_SIGNED_CERT_IN_CHAIN|DEPTH_ZERO_SELF_SIGNED_CERT|CERT_HAS_EXPIRED|ERR_TLS_CERT_ALTNAME_INVALID|ENOTFOUND|EAI_AGAIN|ECONNREFUSED|ECONNRESET|ENETUNREACH|ETIMEDOUT|UND_ERR_CONNECT_TIMEOUT|WebSocket is not defined' "$log" | head -n 1)
	case $match in
		'Wrong vault key') reason='Obsidian rejected the vault encryption password.' ;;
		'Password not provided') reason='Set the OBSIDIAN_VAULT_PASSWORD build secret.' ;;
		'Not logged in'|'No account logged in'|'HTTP Error 401') reason='Obsidian rejected the account token. Check OBSIDIAN_AUTH_TOKEN.' ;;
		'Multiple vaults named') reason='Set OBSIDIAN_VAULT to the vault ID instead of its name.' ;;
		Vault\ \"*\"\ not\ found.) reason='The token cannot see the vault in OBSIDIAN_VAULT.' ;;
		'WebSocket is not defined') reason='Run Obsidian Headless with Node.js 22 or later.' ;;
		'HTTP Error '[1-5][0-9][0-9]) reason="The Obsidian API returned ${match#HTTP Error }." ;;
		UNABLE_TO_*|*SELF_SIGNED_CERT*|CERT_HAS_EXPIRED|ERR_TLS_CERT_ALTNAME_INVALID) reason="Obsidian API TLS error: $match." ;;
		ENOTFOUND|EAI_AGAIN|ECONNREFUSED|ECONNRESET|ENETUNREACH|ETIMEDOUT|UND_ERR_CONNECT_TIMEOUT) reason="Obsidian API connection error: $match." ;;
		*)
			if grep -Fq 'fetch failed' "$log"; then
				reason='Node.js could not reach the Obsidian API.'
			else
				reason="Obsidian Headless exited with status $status. The error does not match a known cause."
			fi
			;;
	esac
	fail "Could not set up Obsidian Sync. $reason"
}

set -- sync-setup "--vault=$OBSIDIAN_VAULT" --path "$vault" --device-name 'plain-blog build' --json
if [ -n "${OBSIDIAN_VAULT_PASSWORD:-}" ]; then
	set -- "$@" "--password=$OBSIDIAN_VAULT_PASSWORD"
fi
unset OBSIDIAN_VAULT_PASSWORD
setup_log="$state/setup.err"
run_sync_command "$cli" "$@" 2>"$setup_log" || fail_sync_setup "$?" "$setup_log"
rm -f "$setup_log"
set --
run_sync_command "$cli" sync-config --path "$vault" --mode pull-only --json 2>/dev/null || fail 'Could not set Obsidian Sync to pull-only mode.'
printf '%s\n' 'Obsidian Sync download started.'
# Headless Sync can retry connection failures until it receives a stop signal.
run_sync_command timeout -k 5s 15m "$cli" sync --path "$vault" 2>/dev/null || fail 'Could not download the Obsidian vault. The download limit is 15 minutes.'
unset OBSIDIAN_AUTH_TOKEN

if ! content_path=$(cd "$vault/$content" 2>/dev/null && pwd -P) || [ "$content_path" != "$vault/$content" ]; then
	fail 'Blog content folder not found in the synced vault. Check OBSIDIAN_CONTENT_DIR.'
fi
printf '%s\n' 'Obsidian Sync download complete.'
BLOG_CONTENT_DIR="$content_path" ./bin/blog-linux-amd64 build
