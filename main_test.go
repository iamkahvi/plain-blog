package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	layout, err := os.ReadFile("layout.html")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "layout.html", string(layout))
	put(t, root, "style.css", "body {}")
	themeScript, err := os.ReadFile("public/color-theme.js")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "public/color-theme.js", string(themeScript))
	put(t, root, "content/about/about.md", "---\ntitle: about\n---\nHello **world**.\n")
	put(t, root, "content/projects/projects.md", "---\ntitle: projects\n---\n- https://example.com\n")
	put(t, root, "content/pronounce/pronounce.md", "---\ntitle: pronounce\n---\n<audio controls></audio>\n")
	put(t, root, "content/posts/first.md", "---\ntitle: 'First & best'\ndate: 2020-01-01\n---\n[Section](#ui-design)\n\n## UI Design\n")
	put(t, root, "content/posts/draft.md", "---\ntitle: Draft\ndate: 2024-01-01\npublished: false\n---\nPrivate\n")
	put(t, root, "content/newsletter/latest.md", "---\ntitle: Latest\ndate: 2026-04-16\n---\nNews\n")
	put(t, root, "content/newsletter/february.md", "---\ntitle: February 2026\ndate: 2026-03-13\nsubstack: 'https://example.com/p/february?one=1&two=2'\n---\nNews\n")
	put(t, root, "public/assets/photo.txt", "photo")
	return root
}

func TestBuild(t *testing.T) {
	root := fixture(t)
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/", "/about/", "/projects/", "/pronounce/", "/writing/", "/writing/first/", "/writing/latest/", "/writing/february/", "/style.css", "/color-theme.js", "/assets/photo.txt"} {
		if _, ok := files[url]; !ok {
			t.Errorf("missing %s", url)
		}
	}
	for _, url := range []string{"/writing/draft/", "/first/", "/newsletter/latest/"} {
		if _, ok := files[url]; ok {
			t.Errorf("unexpected output at %s", url)
		}
	}
	writing := string(files["/writing/"])
	if strings.Contains(writing, "Draft") || !strings.Contains(writing, "First &amp; best") || strings.Index(writing, "Latest") > strings.Index(writing, "First &amp; best") {
		t.Errorf("unexpected list: %s", writing)
	}
	if strings.Count(writing, "<h2>2026</h2>") != 1 || !strings.Contains(writing, "<h2>2020</h2>") ||
		!strings.Contains(writing, `<a href="https://example.com/p/february?one=1&amp;two=2" target="_blank" rel="noopener noreferrer">February 2026</a> - <time datetime="2026-03-13">03.13</time>`) ||
		!strings.Contains(writing, `<a href="/writing/first/">First &amp; best</a>`) ||
		!strings.Contains(writing, `<a href="/writing/latest/">Latest</a>`) || strings.Contains(writing, "<small>") {
		t.Errorf("unexpected year groups or list format: %s", writing)
	}
	home := string(files["/"])
	if !strings.Contains(home, "<h3>2026</h3>") || !strings.Contains(home, `<a href="https://example.com/p/february?one=1&amp;two=2" target="_blank" rel="noopener noreferrer">February 2026</a>`) {
		t.Error("home list does not link to Substack")
	}
	if !strings.Contains(string(files["/about/"]), "Hello <strong>world</strong>") || !strings.Contains(string(files["/pronounce/"]), "<audio controls>") {
		t.Error("Markdown or trusted HTML was not rendered")
	}
	if strings.Contains(string(files["/about/"]), `class="article-header"`) {
		t.Error("undated page has a dated article header")
	}
	projects := string(files["/projects/"])
	if !strings.Contains(projects, "<h1>projects</h1>") || !strings.Contains(projects, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`) {
		t.Error("projects page was not rendered from Markdown")
	}
	post := string(files["/writing/first/"])
	if !strings.Contains(post, "<title>First &amp; best | kahvi patel</title>") ||
		!strings.Contains(post, "<h1>First &amp; best</h1>") ||
		!strings.Contains(post, `<a href="#ui-design">Section</a>`) ||
		!strings.Contains(post, `<h2 id="ui-design">UI Design</h2>`) {
		t.Error("content case or heading links were not preserved")
	}
	if !strings.Contains(post, "<header class=\"article-header\">\n        <h1>First &amp; best</h1>\n        <time datetime=\"2020-01-01\">january 1, 2020</time>\n        </header>\n        <p><a href=\"#ui-design\">Section</a></p>") {
		t.Error("article header is incorrect")
	}
	if !strings.Contains(post, `<a href="/">home</a> · <a href="/writing/">writing</a> · <a href="/projects/">projects</a> · <a href="/about/">about</a> <span id="color-theme-separator" aria-hidden="true" hidden>·</span> <button id="color-theme-toggle" type="button" hidden>change theme</button>`) ||
		!strings.Contains(post, `<script src="/color-theme.js"></script>`) {
		t.Error("theme control or existing navigation is missing")
	}
	newsletter := string(files["/writing/latest/"])
	if !strings.Contains(newsletter, `<time datetime="2026-04-16">april 16, 2026</time>`) || strings.Contains(newsletter, " · newsletter") {
		t.Error("article date is incorrect")
	}
	if strings.Contains(string(files["/"]), "__reload") {
		t.Error("static page contains development script")
	}
	themeScript, err := os.ReadFile(filepath.Join(root, "public", "color-theme.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(files["/color-theme.js"]) != string(themeScript) {
		t.Error("theme script was not copied from public/ unchanged")
	}
	put(t, root, "dist/stale.txt", "old")
	if err := writeSite(root, files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/stale.txt")); !os.IsNotExist(err) {
		t.Error("stale output remains")
	}
	if data, err := os.ReadFile(filepath.Join(root, "dist/writing/latest/index.html")); err != nil || !strings.Contains(string(data), "News") {
		t.Errorf("invalid output: %v", err)
	}
}

func TestMarkdown(t *testing.T) {
	root := t.TempDir()
	put(t, root, "article.md", "---\ntitle: Markdown\ndate: 2024-01-01\n---\n"+`## Heading [LINK](https://example.com/UPPER) &amp; `+"`CODE`"+`

<h3>RAW <em>HEADING</em> <code>CODE</code></h3>

**bold**, *italic*, [link](https://example.com), and ![photo](/photo.jpg).

- First
  - Nested

> A quote

| Name | Value |
| --- | --- |
| one | two |

`+"```go\nif a < b {}\n```"+`

A footnote.[^1]

[^1]: Footnote text with *emphasis*.

<div><iframe src="https://example.com"></iframe></div>
`)
	a, err := readArticle(filepath.Join(root, "article.md"), "/article/", "Post")
	if err != nil {
		t.Fatal(err)
	}
	body := string(a.Body)
	for _, want := range []string{
		`<h2 id="heading-linkhttpsexamplecomupper-amp-code">Heading <a href="https://example.com/UPPER">LINK</a> &amp; <code>CODE</code></h2>`,
		`<h3>RAW <em>HEADING</em> <code>CODE</code></h3>`, "<strong>bold</strong>", "<em>italic</em>",
		`<a href="https://example.com">link</a>`, `<img src="/photo.jpg" alt="photo">`,
		"<li>First\n<ul>\n<li>Nested</li>", "<blockquote>", "<table>", "<td>two</td>",
		`<code class="language-go">if a &lt; b {}`, `href="#fn:1"`,
		`id="fn:1"`, "Footnote text with <em>emphasis</em>", `href="#fnref:1"`,
		`<iframe src="https://example.com"></iframe>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Markdown output lacks %q:\n%s", want, body)
		}
	}
}

func TestBuildRejectsInvalidContentAndConflicts(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/posts/first.md", "---\ntitle: First\ndate: invalid\n---\nBody")
	if _, err := build(root); err == nil {
		t.Fatal("invalid date accepted")
	}
	put(t, root, "content/posts/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\nBody")
	put(t, root, "content/newsletter/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\nBody")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("duplicate post and newsletter slug not detected: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "content/newsletter/first.md")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "public/writing/index.html", "conflict")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("output conflict not detected: %v", err)
	}
}

func TestFingerprint(t *testing.T) {
	root := fixture(t)
	before, err := fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "content/posts/new.md", "new")
	after, err := fingerprint(root)
	if err != nil || before == after {
		t.Fatalf("addition was not detected: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "content/posts/new.md")); err != nil {
		t.Fatal(err)
	}
	last, err := fingerprint(root)
	if err != nil || last == after {
		t.Fatalf("deletion was not detected: %v", err)
	}
}
