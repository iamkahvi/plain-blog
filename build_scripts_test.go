package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func scriptFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"source-hash.sh", "build-artifacts.sh", "build-linux.sh"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, filepath.Join("scripts", name), string(data))
		if err := os.Chmod(filepath.Join(root, "scripts", name), 0755); err != nil {
			t.Fatal(err)
		}
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

func TestBuildArtifactsScript(t *testing.T) {
	root := scriptFixture(t)
	hash, err := runScript(root, "source-hash.sh")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "blog-linux-amd64")
	put(t, root, "bin/blog-linux-amd64", "#!/bin/sh\nif [ \"$1\" = source-hash ]; then\n  printf '%s\\n' '"+strings.TrimSpace(hash)+"'\nelse\n  printf '%s\\n' \"$1\"\nfi\n")
	if err := os.Chmod(binary, 0755); err != nil {
		t.Fatal(err)
	}
	out, err := runScript(root, "build-artifacts.sh")
	if err != nil || strings.TrimSpace(out) != "build-artifacts" {
		t.Fatalf("matching executable was not run: %q, %v", out, err)
	}
	put(t, root, "main.go", "package main\n// changed\n")
	out, err = runScript(root, "build-artifacts.sh")
	if err == nil || !strings.Contains(out, "does not match the Go source") {
		t.Fatalf("stale executable was accepted: %q, %v", out, err)
	}
}
