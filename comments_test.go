package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripComments(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"inline", "Before %%private%% after.", "Before  after."},
		{"multiple", "%%one%%Visible%%two%%", "Visible"},
		{"empty", "A%%%%B", "AB"},
		{"multiline", "Before\n%%private\n\ntext%%\nAfter", "Before\n\n\n\nAfter"},
		{"CRLF", "A%%one\r\ntwo%%B", "A\r\nB"},
		{"inline code", "`%%literal%%` and %%private%%", "`%%literal%%` and "},
		{"multiline code span", "``one\n%%literal%%\ntwo`` %%private%%", "``one\n%%literal%%\ntwo`` "},
		{"unclosed marker in code", "`%%` and %%private%%", "`%%` and "},
		{"fenced code", "```go\n%%literal%%\n```\n%%private%%", "```go\n%%literal%%\n```\n"},
		{"fence info", "```%%\ncode\n```\n%%private%%", "```%%\ncode\n```\n"},
		{"indented code", "    %%literal%%\n\n%%private%%", "    %%literal%%\n\n"},
		{"nested code", "> ```\n> %%literal%%\n> ```\n\n- ```\n  %%also literal%%\n  ```\n\n%%private%%", "> ```\n> %%literal%%\n> ```\n\n- ```\n  %%also literal%%\n  ```\n\n"},
		{"code fence inside comment", "%%private\n```\nstill private\n%%\n`%%literal%%`", "\n\n\n\n`%%literal%%`"},
		{"comments around code", "%%first%%\n```\n%%literal%%\n```\n%%second%%", "\n```\n%%literal%%\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stripComments([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnclosedComment(t *testing.T) {
	for _, input := range []string{
		"First\n%%private",
		"%%first%%\n%%private",
		"`%%`\n%%private",
	} {
		_, err := stripComments([]byte(input))
		if err == nil || !strings.Contains(err.Error(), "unclosed %% comment at body line 2") {
			t.Errorf("stripComments(%q): %v", input, err)
		}
	}
}

func TestCommentsInBuild(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/writing/posts/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\n%%Secret introduction%%\n\nPublic %%private%% text.\n\n`%%example%%`\n")
	put(t, root, "content/resume.md", "---\nrender: raw\n---\n%%raw content%%\n")
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	page := string(files["/writing/first/"])
	if strings.Contains(page, "Secret") || strings.Contains(page, "private") ||
		!strings.Contains(page, `<meta name="description" content="Public text.">`) ||
		!strings.Contains(page, "<code>%%example%%</code>") {
		t.Errorf("unexpected output: %s", page)
	}
	if !bytes.Equal(files["/resume.md"], []byte("%%raw content%%\n")) {
		t.Errorf("raw content changed: %q", files["/resume.md"])
	}

	put(t, root, "content/writing/posts/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\n%%private\n")
	_, err = build(root)
	if err == nil || !strings.Contains(err.Error(), filepath.Join("writing", "posts", "first.md")) ||
		!strings.Contains(err.Error(), "unclosed %% comment") {
		t.Fatalf("unclosed comment should fail the build with its filename: %v", err)
	}
}
