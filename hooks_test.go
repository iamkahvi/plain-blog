package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hookGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgSign=false"}, args...)...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func hookFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join(".githooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, ".githooks/pre-commit", string(data))
	put(t, root, "scripts/source-hash.sh", "#!/bin/sh\nexit 0\n")
	put(t, root, "scripts/build-linux.sh", "#!/bin/sh\nset -eu\necho BUILT >&2\nmkdir -p bin\ncp main.go bin/blog-linux-amd64\nchmod 755 bin/blog-linux-amd64\n")
	put(t, root, "main.go", "initial source\n")
	put(t, root, "main_test.go", "initial test\n")
	put(t, root, "go.mod", "module example\n")
	put(t, root, "go.sum", "dependency\n")
	put(t, root, "bin/blog-linux-amd64", "initial source\n")
	for _, file := range []string{".githooks/pre-commit", "scripts/source-hash.sh", "scripts/build-linux.sh", "bin/blog-linux-amd64"} {
		if err := os.Chmod(filepath.Join(root, file), 0755); err != nil {
			t.Fatal(err)
		}
	}
	hookGit(t, root, "init", "--quiet")
	hookGit(t, root, "add", ".")
	hookGit(t, root, "commit", "--quiet", "-m", "initial")
	hookGit(t, root, "config", "core.hooksPath", ".githooks")
	return root
}

func TestPreCommitBuildsStagedSource(t *testing.T) {
	root := hookFixture(t)
	put(t, root, "main.go", "staged source\n")
	hookGit(t, root, "add", "main.go")
	put(t, root, "main.go", "unstaged source\n")
	hookGit(t, root, "commit", "--quiet", "-m", "change source")
	if got := hookGit(t, root, "show", "HEAD:bin/blog-linux-amd64"); got != "staged source\n" {
		t.Fatalf("committed executable used the wrong source: %q", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(data) != "unstaged source\n" {
		t.Fatalf("unstaged source changed: %q, %v", data, err)
	}
	if got := hookGit(t, root, "status", "--short", "--", "bin/blog-linux-amd64"); got != "" {
		t.Fatalf("executable is unexpectedly modified: %q", got)
	}
}

func TestPreCommitPreservesUnstagedExecutable(t *testing.T) {
	root := hookFixture(t)
	put(t, root, "bin/blog-linux-amd64", "local executable changes\n")
	put(t, root, "main.go", "staged source\n")
	hookGit(t, root, "add", "main.go")
	hookGit(t, root, "commit", "--quiet", "-m", "change source")
	if got := hookGit(t, root, "show", "HEAD:bin/blog-linux-amd64"); got != "staged source\n" {
		t.Fatalf("committed executable used the wrong source: %q", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "bin/blog-linux-amd64"))
	if err != nil || string(data) != "local executable changes\n" {
		t.Fatalf("unstaged executable changed: %q, %v", data, err)
	}
}

func TestPreCommitSkipsTestOnlyChanges(t *testing.T) {
	root := hookFixture(t)
	put(t, root, "main_test.go", "updated test\n")
	hookGit(t, root, "add", "main_test.go")
	if out := hookGit(t, root, "commit", "--quiet", "-m", "change test"); strings.Contains(out, "BUILT") {
		t.Fatalf("test-only commit rebuilt the executable: %s", out)
	}
}

func TestPreCommitRejectsPathspec(t *testing.T) {
	root := hookFixture(t)
	put(t, root, "main.go", "pathspec source\n")
	cmd := exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgSign=false", "commit", "--quiet", "-m", "change source", "--only", "main.go")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "commit without file paths or --only") {
		t.Fatalf("pathspec commit was not rejected: %q, %v", out, err)
	}
	if got := hookGit(t, root, "status", "--short", "--", "bin/blog-linux-amd64"); got != "" {
		t.Fatalf("executable changed after a rejected commit: %q", got)
	}
}

func TestPreCommitWithAll(t *testing.T) {
	root := hookFixture(t)
	put(t, root, "main.go", "updated source\n")
	hookGit(t, root, "commit", "--quiet", "--all", "-m", "change source")
	if got := hookGit(t, root, "show", "HEAD:bin/blog-linux-amd64"); got != "updated source\n" {
		t.Fatalf("commit --all used the wrong executable: %q", got)
	}
	if got := hookGit(t, root, "status", "--short", "--", "bin/blog-linux-amd64"); got != "" {
		t.Fatalf("executable is unexpectedly modified after commit --all: %q", got)
	}
}
