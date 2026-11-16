package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testAccount = "0123456789abcdef0123456789abcdef"

func testArtifactsConfig() artifactsConfig {
	return artifactsConfig{account: testAccount, namespace: "default", repo: "blog-content", branch: "main", repoToken: "private-repo-token"}
}

func TestArtifactsConfig(t *testing.T) {
	values := map[string]string{"ARTIFACTS_ACCOUNT_ID": testAccount, "ARTIFACTS_REPO": "blog-content", "ARTIFACTS_REPO_TOKEN": "secret"}
	getenv := func(name string) string { return values[name] }
	cfg, err := artifactsConfigFromEnv(getenv)
	if err != nil || cfg.branch != "main" || cfg.namespace != "default" {
		t.Fatalf("invalid defaults: %+v, %v", cfg, err)
	}
	delete(values, "ARTIFACTS_REPO_TOKEN")
	if _, err := artifactsConfigFromEnv(getenv); err == nil || !strings.Contains(err.Error(), "ARTIFACTS_REPO_TOKEN") {
		t.Fatalf("missing secret accepted: %v", err)
	}
	values["ARTIFACTS_REPO_TOKEN"] = "secret"
	values["ARTIFACTS_REPO"] = "../other"
	if _, err := artifactsConfigFromEnv(getenv); err == nil {
		t.Fatal("invalid repository accepted")
	}
	values["ARTIFACTS_REPO"] = "blog-content"
	values["ARTIFACTS_REPO_TOKEN"] = "secret\ninjected"
	if _, err := artifactsConfigFromEnv(getenv); err == nil {
		t.Fatal("invalid token accepted")
	}
}

func TestBuildArtifacts(t *testing.T) {
	root := fixture(t)
	source := filepath.Join(t.TempDir(), "source")
	if err := os.Rename(filepath.Join(root, "content"), source); err != nil {
		t.Fatal(err)
	}
	put(t, source, "home/home.md", "---\ntitle: Vault\npermalink: /\n---\nFrom the vault.\n")
	put(t, root, "content/home/home.md", "---\ntitle: Checkout\npermalink: /\n---\nFrom the checkout.\n")
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main", source},
		{"-C", source, "add", "."},
		{"-C", source, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--quiet", "-m", "fixture"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v: %s", err, out)
		}
	}
	t.Setenv("ARTIFACTS_REPO_TOKEN", "private-repo-token")
	t.Setenv("BLOG_CONTENT_DIR", filepath.Join(root, "content"))
	t.Setenv("GIT_TRACE_CURL", "1")
	var content string
	git := func(args, env []string) ([]byte, error) {
		if args[0] == "clone" {
			content = args[len(args)-1]
			joined := strings.Join(env, "\n")
			if strings.Contains(joined, "ARTIFACTS_REPO_TOKEN=") || !strings.Contains(joined, "GIT_CONFIG_VALUE_0=Authorization: Bearer private-repo-token") || !strings.Contains(joined, "GIT_TRACE_CURL=0") || !strings.Contains(joined, "GIT_CONFIG_VALUE_1=false") {
				t.Error("incorrect Git credentials or trace settings")
			}
			wantRemote := "https://" + testAccount + ".artifacts.cloudflare.net/git/default/blog-content.git"
			if args[len(args)-2] != wantRemote || strings.Contains(strings.Join(args, " "), "private-repo-token") {
				t.Error("incorrect remote or token present in arguments")
			}
			args = append([]string(nil), args...)
			args[len(args)-2] = source
		}
		return runGit(args, env)
	}
	if err := buildArtifacts(root, testArtifactsConfig(), git); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(root, "dist/index.html"))
	if err != nil || !strings.Contains(string(page), "From the vault.") || strings.Contains(string(page), "From the checkout.") {
		t.Fatalf("build did not use Artifacts content: %v", err)
	}
	if _, err := os.Stat(content); !os.IsNotExist(err) {
		t.Fatalf("temporary content was not removed: %v", err)
	}
}

func TestBuildArtifactsFailureCleansUp(t *testing.T) {
	root := fixture(t)
	put(t, root, "dist/previous.txt", "previous")
	var content string
	git := func(args, _ []string) ([]byte, error) {
		content = args[len(args)-1]
		if err := os.MkdirAll(content, 0700); err != nil {
			t.Fatal(err)
		}
		return nil, fmt.Errorf("private-repo-token")
	}
	err := buildArtifacts(root, testArtifactsConfig(), git)
	if err == nil || strings.Contains(err.Error(), "private-repo-token") {
		t.Fatalf("Git error exposed or ignored: %v", err)
	}
	if _, err := os.Stat(content); !os.IsNotExist(err) {
		t.Fatalf("temporary content was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/previous.txt")); err != nil {
		t.Fatalf("previous output changed: %v", err)
	}
}

func TestBuildArtifactsInvalidContentCleansUp(t *testing.T) {
	root := fixture(t)
	put(t, root, "dist/previous.txt", "previous")
	var content string
	git := func(args, _ []string) ([]byte, error) {
		if args[0] == "clone" {
			content = args[len(args)-1]
			put(t, content, "home/home.md", "invalid content")
			return nil, nil
		}
		return []byte("revision"), nil
	}
	if err := buildArtifacts(root, testArtifactsConfig(), git); err == nil {
		t.Fatal("invalid content accepted")
	}
	if _, err := os.Stat(content); !os.IsNotExist(err) {
		t.Fatalf("temporary content was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/previous.txt")); err != nil {
		t.Fatalf("previous output changed: %v", err)
	}
}
