package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func putExecutable(t *testing.T, root, name, contents string) {
	t.Helper()
	put(t, root, name, contents)
	if err := os.Chmod(filepath.Join(root, name), 0755); err != nil {
		t.Fatal(err)
	}
}

func scriptFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"source-hash.sh", "build-obsidian.sh", "build-linux.sh"} {
		path := filepath.Join("scripts", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		putExecutable(t, root, path, string(data))
	}
	put(t, root, "main.go", "package main\n")
	put(t, root, "go.mod", "module example\n")
	put(t, root, "go.sum", "dependency\n")
	return root
}

func runScript(root, name string) (string, error) {
	cmd := exec.Command(filepath.Join(root, "scripts", name))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSourceHash(t *testing.T) {
	root := scriptFixture(t)
	initial, err := runScript(root, "source-hash.sh")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "main_test.go", "package main\n")
	put(t, root, "node_modules/example/ignored.go", "package ignored\n")
	unchanged, err := runScript(root, "source-hash.sh")
	if err != nil || unchanged != initial {
		t.Fatalf("ignored files changed the hash: %q, %v", unchanged, err)
	}
	put(t, root, "extra.go", "package main\n")
	added, err := runScript(root, "source-hash.sh")
	if err != nil || added == initial {
		t.Fatalf("new Go file did not change the hash: %q, %v", added, err)
	}
	if err := os.Remove(filepath.Join(root, "extra.go")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "go.sum", "changed dependency\n")
	changed, err := runScript(root, "source-hash.sh")
	if err != nil || changed == initial {
		t.Fatalf("dependency change did not change the hash: %q, %v", changed, err)
	}
	put(t, root, "go.sum", "dependency\n")
	put(t, root, "scripts/build-linux.sh", "#!/bin/sh\n# changed\n")
	recipe, err := runScript(root, "source-hash.sh")
	if err != nil || recipe == initial {
		t.Fatalf("build recipe change did not change the hash: %q, %v", recipe, err)
	}
}

func TestBuildObsidianScript(t *testing.T) {
	root := scriptFixture(t)
	hash, err := runScript(root, "source-hash.sh")
	if err != nil {
		t.Fatal(err)
	}
	putExecutable(t, root, "bin/blog-linux-amd64", `#!/bin/sh
set -eu
case "$1" in
source-hash) printf '%s\n' '`+strings.TrimSpace(hash)+`' ;;
build)
	[ -z "${OBSIDIAN_AUTH_TOKEN+x}${OBSIDIAN_VAULT_PASSWORD+x}" ]
	state=$(cd "$TMPDIR" && pwd -P)
	set -- "$state"/blog-vault.*/vault/"$TEST_FOLDER"
	[ "$BLOG_CONTENT_DIR" = "$1" ]
	[ -f "$BLOG_CONTENT_DIR/blog-source" ]
	[ ! -e "$BLOG_CONTENT_DIR/private-note" ]
	echo 'generator ran'
	;;
*) exit 1 ;;
esac
`)
	putExecutable(t, root, "node_modules/.bin/ob", `#!/bin/sh
set -eu
[ "$OBSIDIAN_AUTH_TOKEN" = 'test-auth-token' ]
[ -z "${OBSIDIAN_VAULT_PASSWORD+x}" ]
[ "$HOME" = "$TMPDIR/build-home" ]
[ "$XDG_CONFIG_HOME" = "$TMPDIR/build-config" ]
state=$(cd "$TMPDIR"/blog-vault.* && pwd -P)
vault="$state/vault"
printf '%s\n' "$OBSIDIAN_AUTH_TOKEN" >&2
printf '%s\n' "$1" >> "$TEST_CALLS"
if [ "$1" = "$TEST_FAIL" ]; then
	printf '%s\n' "$TEST_ERROR" 'test-vault-password' >&2
	exit 2
fi
case "$1" in
sync-setup)
	[ "$2|$3|$4|$5|$6|$7" = "--vault=Test Vault|--path|$vault|--device-name|plain-blog build|--json" ]
	[ "${8-}" = "$TEST_PASSWORD_ARG" ]
	mkdir "$vault"
	;;
sync-config)
	[ "$*" = "sync-config --path $vault --mode pull-only --json" ]
	touch "$state/pull-only"
	;;
sync)
	[ "$*" = "sync --path $vault" ]
	[ -f "$state/pull-only" ]
	touch "$vault/private-note"
	mkdir -p "$vault/$TEST_FOLDER"
	touch "$vault/$TEST_FOLDER/blog-source"
	;;
*) exit 1 ;;
esac
`)
	putExecutable(t, root, "tools/timeout", `#!/bin/sh
set -eu
[ "$1|$2|$3" = '-k|5s|15m' ]
shift 3
"$@"
[ "$TEST_FAIL" != timeout ] || exit 124
`)
	for _, tc := range []struct {
		name, path, password, failStep, setupError, wantCalls, wantOutput string
	}{
		{"default", "", "", "", "", "sync-setup\nsync-config\nsync\n", "generator ran"},
		{"custom path and password", "nested/blog folder", "test-vault-password", "", "", "sync-setup\nsync-config\nsync\n", "generator ran"},
		{"unknown setup error", "", "", "sync-setup", "internal error: Private Vault", "sync-setup\n", "Obsidian Headless exited with status 2"},
		{"token rejected", "", "", "sync-setup", "Setup failed: s [Error]: Not logged in", "sync-setup\n", "Obsidian rejected the account token"},
		{"password rejected", "", "", "sync-setup", "Wrong vault key", "sync-setup\n", "Obsidian rejected the vault encryption password"},
		{"vault not found", "", "", "sync-setup", `Vault "Private Vault" not found.`, "sync-setup\n", "The token cannot see the vault in OBSIDIAN_VAULT"},
		{"TLS error", "", "", "sync-setup", "TypeError: fetch failed\n  code: 'UNABLE_TO_VERIFY_LEAF_SIGNATURE'", "sync-setup\n", "Obsidian API TLS error: UNABLE_TO_VERIFY_LEAF_SIGNATURE"},
		{"API error", "", "", "sync-setup", "Setup failed: Error: HTTP Error 503", "sync-setup\n", "The Obsidian API returned 503"},
		{"pull-only config fails", "", "", "sync-config", "", "sync-setup\nsync-config\n", "Could not set Obsidian Sync to pull-only mode"},
		{"sync fails", "", "", "sync", "", "sync-setup\nsync-config\nsync\n", "Could not download the Obsidian vault"},
		{"sync times out", "", "", "timeout", "", "sync-setup\nsync-config\nsync\n", "The download limit is 15 minutes"},
		{"folder missing", "missing", "", "", "", "sync-setup\nsync-config\nsync\n", "Blog content folder not found"},
		{"path traversal", "notes/../blog-content", "", "", "", "", "relative folder within the vault"},
		{"absolute path", "/blog-content", "", "", "", "", "relative folder within the vault"},
		{"vault root", ".", "", "", "", "", "relative folder within the vault"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			calls := filepath.Join(tmp, "calls")
			put(t, tmp, "calls", "")
			folder := "blog-content"
			if tc.path == "nested/blog folder" {
				folder = tc.path
			}
			passwordArg := ""
			if tc.password != "" {
				passwordArg = "--password=" + tc.password
			}
			t.Setenv("TMPDIR", tmp)
			t.Setenv("HOME", filepath.Join(tmp, "build-home"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "build-config"))
			t.Setenv("PATH", filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("BLOG_CONTENT_DIR", "/not/the/content/source")
			t.Setenv("OBSIDIAN_AUTH_TOKEN", "test-auth-token")
			t.Setenv("OBSIDIAN_VAULT", "Test Vault")
			t.Setenv("OBSIDIAN_CONTENT_DIR", tc.path)
			t.Setenv("OBSIDIAN_VAULT_PASSWORD", tc.password)
			t.Setenv("TEST_FOLDER", folder)
			t.Setenv("TEST_PASSWORD_ARG", passwordArg)
			t.Setenv("TEST_CALLS", calls)
			t.Setenv("TEST_FAIL", tc.failStep)
			t.Setenv("TEST_ERROR", tc.setupError)
			out, err := runScript(root, "build-obsidian.sh")
			success := tc.wantOutput == "generator ran"
			if (err == nil) != success || !strings.Contains(out, tc.wantOutput) || (!success && strings.Contains(out, "generator ran")) {
				t.Fatalf("unexpected build result: %q, %v", out, err)
			}
			if strings.Contains(out, "test-auth-token") || strings.Contains(out, "test-vault-password") || strings.Contains(out, "Private Vault") {
				t.Fatal("build logs contain private data")
			}
			got, err := os.ReadFile(calls)
			if err != nil || string(got) != tc.wantCalls {
				t.Fatalf("unexpected sync calls: %q, %v", got, err)
			}
			entries, err := os.ReadDir(tmp)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary vault was not removed: %v, %v", entries, err)
			}
		})
	}
}

func TestBuildObsidianRejectsStaleGenerator(t *testing.T) {
	root := scriptFixture(t)
	putExecutable(t, root, "bin/blog-linux-amd64", "#!/bin/sh\nprintf 'stale hash\\n'\n")
	out, err := runScript(root, "build-obsidian.sh")
	if err == nil || !strings.Contains(out, "does not match the Go source") {
		t.Fatalf("stale executable was accepted: %q, %v", out, err)
	}
}
