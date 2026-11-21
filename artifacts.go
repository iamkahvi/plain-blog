package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type artifactsConfig struct {
	account, namespace, repo, branch, repoToken string
}

var (
	accountPattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
	namePattern    = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9._-]*$`)
)

func artifactsConfigFromEnv(getenv func(string) string) (artifactsConfig, error) {
	cfg := artifactsConfig{
		account: getenv("ARTIFACTS_ACCOUNT_ID"), namespace: getenv("ARTIFACTS_NAMESPACE"),
		repo: getenv("ARTIFACTS_REPO"), branch: getenv("ARTIFACTS_BRANCH"), repoToken: getenv("ARTIFACTS_REPO_TOKEN"),
	}
	for _, item := range []struct{ name, value string }{
		{"ARTIFACTS_ACCOUNT_ID", cfg.account}, {"ARTIFACTS_REPO", cfg.repo}, {"ARTIFACTS_REPO_TOKEN", cfg.repoToken},
	} {
		if item.value == "" {
			return cfg, fmt.Errorf("set %s before the build", item.name)
		}
	}
	if cfg.namespace == "" {
		cfg.namespace = "default"
	}
	if cfg.branch == "" {
		cfg.branch = "main"
	}
	if !accountPattern.MatchString(cfg.account) || !namePattern.MatchString(cfg.namespace) || !namePattern.MatchString(cfg.repo) {
		return cfg, fmt.Errorf("invalid Artifacts account ID, namespace, or repository name")
	}
	if strings.ContainsAny(cfg.repoToken, "\r\n\x00") {
		return cfg, fmt.Errorf("invalid Artifacts repository token")
	}
	return cfg, nil
}

type gitRunner func(args, env []string) ([]byte, error)

func runGit(args, env []string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// gitEnvironment keeps credentials out of the command line and disables Git traces.
func gitEnvironment(repoToken string) []string {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "ARTIFACTS_REPO_TOKEN" || key == "ARTIFACTS_API_TOKEN" || key == "GIT_CONFIG_COUNT" || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") || strings.HasPrefix(key, "GIT_TRACE") || key == "GIT_CURL_VERBOSE" || key == "GIT_TERMINAL_PROMPT" {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Bearer "+repoToken,
		"GIT_CONFIG_KEY_1=http.followRedirects", "GIT_CONFIG_VALUE_1=false",
		"GIT_TERMINAL_PROMPT=0", "GIT_TRACE=0", "GIT_TRACE_CURL=0", "GIT_TRACE_PACKET=0", "GIT_CURL_VERBOSE=0",
	)
}

func buildArtifacts(root string, cfg artifactsConfig, git gitRunner) error {
	remote := fmt.Sprintf("https://%s.artifacts.cloudflare.net/git/%s/%s.git", strings.ToLower(cfg.account), cfg.namespace, cfg.repo)
	temp, err := os.MkdirTemp("", "blog-content-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	content := filepath.Join(temp, "content")
	env := gitEnvironment(cfg.repoToken)
	start := time.Now()
	if _, err := git([]string{"clone", "--quiet", "--depth=1", "--branch=" + cfg.branch, "--", remote, content}, env); err != nil {
		return fmt.Errorf("could not clone the Artifacts repository; check its branch and token")
	}
	log.Printf("cloned Artifacts content in %s", time.Since(start).Round(time.Millisecond))
	revision, err := git([]string{"-C", content, "rev-parse", "HEAD"}, env)
	if err != nil {
		return fmt.Errorf("could not read the Artifacts content revision")
	}
	log.Printf("Artifacts content revision: %s", strings.TrimSpace(string(revision)))
	start = time.Now()
	files, err := buildWithContent(root, content)
	if err != nil {
		return err
	}
	log.Printf("generated site in %s", time.Since(start).Round(time.Millisecond))
	start = time.Now()
	if err := writeSite(root, files); err != nil {
		return err
	}
	log.Printf("wrote %d files to dist/ in %s", len(files), time.Since(start).Round(time.Millisecond))
	return nil
}

func buildFromArtifacts(root string) error {
	cfg, err := artifactsConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	return buildArtifacts(root, cfg, runGit)
}
