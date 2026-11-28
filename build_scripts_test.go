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
[ "$XDG_CONFIG_HOME" = "$HOME/config" ]
[ -d "$HOME" ]
vault="$HOME/vault"
printf '%s\n' "$OBSIDIAN_AUTH_TOKEN" >&2
printf '%s\n' "$1" >> "$TEST_CALLS"
[ "$1" != "$TEST_FAIL" ] || exit 1
case "$1" in
sync-setup)
	[ "$2|$3|$4|$5|$6|$7" = "--vault=Test Vault|--path|$vault|--device-name|plain-blog build|--json" ]
	[ "${8-}" = "$TEST_PASSWORD_ARG" ]
	mkdir "$vault"
	;;
sync-config)
	[ "$*" = "sync-config --path $vault --mode pull-only --json" ]
	touch "$HOME/pull-only"
	;;
sync)
	[ "$*" = "sync --path $vault" ]
	[ -f "$HOME/pull-only" ]
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
		name, path, password, failStep, wantCalls, wantOutput string
	}{
		{"default", "", "", "", "sync-setup\nsync-config\nsync\n", "generator ran"},
		{"custom path and password", "nested/blog folder", "test-vault-password", "", "sync-setup\nsync-config\nsync\n", "generator ran"},
		{"setup fails", "", "", "sync-setup", "sync-setup\n", "Could not set up Obsidian Sync"},
		{"pull-only config fails", "", "", "sync-config", "sync-setup\nsync-config\n", "Could not set Obsidian Sync to pull-only mode"},
		{"sync fails", "", "", "sync", "sync-setup\nsync-config\nsync\n", "Could not download the Obsidian vault"},
		{"sync times out", "", "", "timeout", "sync-setup\nsync-config\nsync\n", "The download limit is 15 minutes"},
		{"folder missing", "missing", "", "", "sync-setup\nsync-config\nsync\n", "Blog content folder not found"},
		{"path traversal", "notes/../blog-content", "", "", "", "relative folder within the vault"},
		{"absolute path", "/blog-content", "", "", "", "relative folder within the vault"},
		{"vault root", ".", "", "", "", "relative folder within the vault"},
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
			out, err := runScript(root, "build-obsidian.sh")
			success := tc.wantOutput == "generator ran"
			if (err == nil) != success || !strings.Contains(out, tc.wantOutput) || (!success && strings.Contains(out, "generator ran")) {
				t.Fatalf("unexpected build result: %q, %v", out, err)
			}
			if strings.Contains(out, "test-auth-token") || strings.Contains(out, "test-vault-password") {
				t.Fatal("build logs contain credentials")
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
