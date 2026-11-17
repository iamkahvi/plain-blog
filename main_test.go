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
	layout, err := os.ReadFile("templates/layout.html")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "templates/layout.html", string(layout))
	put(t, root, "public/style.css", "body {}")
	for _, name := range []string{"public/theme-mode.js", "public/font-mode.js"} {
		script, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, name, string(script))
	}
	put(t, root, "content/home/home.md", "---\ntitle: Test Blog\npermalink: /\n---\nHello **world**.\n")
	put(t, root, "content/writing/writing.md", "---\ntitle: writing\npermalink: /writing/\n---\nWriting introduction.\n")
	put(t, root, "content/projects/projects.md", "---\ntitle: projects\npermalink: /projects/\n---\n- https://example.com\n")
	put(t, root, "content/pronounce/pronounce.md", "---\ntitle: pronounce\npermalink: /pronounce/\n---\n<audio controls></audio>\n")
	put(t, root, "content/book-shelf/book-shelf.md", "---\ntitle: book shelf\npermalink: /book-shelf/\n---\nBooks I have read.\n")
	put(t, root, "content/extra/extra.md", "---\ntitle: Extra\npermalink: /new-path/\n---\nExtra page.\n")
	put(t, root, "content/resume.md", "---\nrender: raw\n---\n# Resume\n")
	put(t, root, "content/book-shelf/books/older.md", "---\ntitle: Older Book\nauthor: First Author\ndate: 2020-01-01\n---\nOlder notes.\n")
	put(t, root, "content/book-shelf/books/newer.md", "---\ntitle: \"A Newer's Book\"\nauthor: Second Author\ndate: 2026-04-25\n---\n**Newer notes.**\n")
	put(t, root, "content/book-shelf/books/middle.md", "---\ntitle: Middle Book\nauthor: Third Author\ndate: 2026-02-02\n---\nMiddle notes.\n")
	put(t, root, "content/book-shelf/books/oldest.md", "---\ntitle: Oldest Book\nauthor: Fourth Author\ndate: 2018-12-31\n---\nOldest notes.\n")
	put(t, root, "content/writing/posts/first.md", "---\ntitle: 'First & best'\ndate: 2020-01-01\n---\n[Section](#ui-design)\n\n## UI Design\n")
	put(t, root, "content/writing/posts/draft.md", "---\ntitle: Draft\ndate: 2024-01-01\npublished: false\n---\nPrivate\n")
	put(t, root, "content/writing/newsletter/latest.md", "---\ntitle: Latest\ndate: 2026-04-16\n---\nNews\n")
	put(t, root, "content/writing/newsletter/february.md", "---\ntitle: February 2026\ndate: 2026-03-13\nsubstack: 'https://example.com/p/february?one=1&two=2'\n---\nNews\n")
	put(t, root, "public/assets/photo.txt", "photo")
	return root
}

func TestBuild(t *testing.T) {
	root := fixture(t)
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/", "/projects/", "/pronounce/", "/book-shelf/", "/writing/", "/new-path/", "/resume.md", "/writing/first/", "/writing/latest/", "/writing/february/", "/style.css", "/theme-mode.js", "/font-mode.js", "/assets/photo.txt"} {
		if _, ok := files[url]; !ok {
			t.Errorf("missing %s", url)
		}
	}
	for _, url := range []string{"/about/", "/extra/", "/writing/draft/", "/first/", "/newsletter/latest/"} {
		if _, ok := files[url]; ok {
			t.Errorf("unexpected output at %s", url)
		}
	}
	externalLink := markedLink(`<a href="https://example.com/p/february?one=1&amp;two=2" target="_blank" rel="noopener noreferrer">February 2026</a>`, true)
	writing := string(files["/writing/"])
	if strings.Contains(writing, "Draft") || !strings.Contains(writing, "First &amp; best") || strings.Index(writing, "Latest") > strings.Index(writing, "First &amp; best") {
		t.Errorf("unexpected list: %s", writing)
	}
	if strings.Count(writing, "<h2>2026</h2>") != 1 || !strings.Contains(writing, "<h2>2020</h2>") ||
		!strings.Contains(writing, externalLink+` - <time datetime="2026-03-13">03.13</time>`) ||
		!strings.Contains(writing, `<a href="/writing/first/">First &amp; best</a>`) ||
		!strings.Contains(writing, `<a href="/writing/latest/">Latest</a>`) || strings.Contains(writing, "<small>") {
		t.Errorf("unexpected year groups or list format: %s", writing)
	}
	home := string(files["/"])
	if !strings.Contains(home, "<title>Test Blog</title>") || !strings.Contains(home, "<h1>Test Blog</h1>") || !strings.Contains(home, externalLink) {
		t.Error("home list does not link to Substack with the external link icon")
	}
	if strings.Contains(home, "<h3>2026</h3>") || strings.Contains(home, "<h3>2020</h3>") || strings.Contains(home, "<h2>2026</h2>") || strings.Contains(home, "<h2>2020</h2>") || strings.Count(home, `<ul class="articles">`) != 1 || !strings.Contains(home, "First &amp; best") {
		t.Error("home should show recent entries in one list without year headings")
	}
	for _, page := range []string{home, writing} {
		if strings.Count(page, `class="external-link-icon"`) != 1 {
			t.Error("list should show the icon for the Substack link")
		}
	}
	if !strings.Contains(home, "Hello <strong>world</strong>") || !strings.Contains(writing, "Writing introduction.") || !strings.Contains(string(files["/pronounce/"]), "<audio controls>") || !strings.Contains(string(files["/new-path/"]), "Extra page.") {
		t.Error("Markdown, generic page, or trusted HTML was not rendered")
	}
	if string(files["/resume.md"]) != "# Resume\n" {
		t.Errorf("raw file includes front matter or changed body: %q", files["/resume.md"])
	}
	if strings.Contains(string(files["/new-path/"]), `class="article-header"`) {
		t.Error("undated page has a dated article header")
	}
	projects := string(files["/projects/"])
	if !strings.Contains(projects, "<h1>projects</h1>") || !strings.Contains(projects, markedLink(`<a href="https://example.com" target="_blank" rel="noopener noreferrer">https://example.com</a>`, true)) {
		t.Error("projects page was not rendered from Markdown")
	}
	post := string(files["/writing/first/"])
	if !strings.Contains(post, "<title>First &amp; best | Test Blog</title>") ||
		!strings.Contains(post, "<h1>First &amp; best</h1>") ||
		!strings.Contains(post, `<a href="#ui-design">Section</a>`) ||
		!strings.Contains(post, `<h2 id="ui-design">UI Design</h2>`) {
		t.Error("content case or heading links were not preserved")
	}
	if !strings.Contains(post, "<header class=\"article-header\">\n        <h1>First &amp; best</h1>\n        <time datetime=\"2020-01-01\">january 1, 2020</time>\n        </header>\n        <p><a href=\"#ui-design\">Section</a></p>") {
		t.Error("article header is incorrect")
	}
	if !strings.Contains(post, `<a href="/">home</a> · <a href="/writing/">writing</a> · <a href="/book-shelf/">book shelf</a> · <a href="/projects/">projects</a>`) {
		t.Error("existing navigation is missing")
	}
	if !strings.Contains(home, `<p><button id="theme-toggle" type="button" hidden>switch theme</button></p>`) ||
		strings.Contains(post, `id="theme-toggle"`) {
		t.Error("theme button should appear only on the home page")
	}
	for _, page := range []string{home, writing, post} {
		if !strings.Contains(page, `<script src="/theme-mode.js"></script>`) {
			t.Error("theme script is missing")
		}
		if !strings.Contains(page, `<script src="/font-mode.js"></script>`) ||
			strings.Index(page, `<script src="/font-mode.js"></script>`) > strings.Index(page, `<link rel="stylesheet"`) {
			t.Error("font script must appear before the stylesheet")
		}
	}
	newsletter := string(files["/writing/latest/"])
	if !strings.Contains(newsletter, `<time datetime="2026-04-16">april 16, 2026</time>`) || strings.Contains(newsletter, " · newsletter") {
		t.Error("article date is incorrect")
	}
	if strings.Contains(string(files["/"]), "__reload") {
		t.Error("static page contains development script")
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

func TestFontAssets(t *testing.T) {
	root := fixture(t)
	assets := []string{
		"/fonts/GeistSerif-Regular.woff2", "/fonts/OFL.txt", "/fonts/LibreBaskerville-OFL.txt",
		"/fonts/libre-baskerville-latin-wght-normal.woff2", "/fonts/libre-baskerville-latin-wght-italic.woff2",
		"/fonts/libre-baskerville-latin-ext-wght-normal.woff2", "/fonts/libre-baskerville-latin-ext-wght-italic.woff2",
	}
	for _, name := range append([]string{"/style.css"}, assets...) {
		data, err := os.ReadFile("public" + name)
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, "public"+name, string(data))
	}

	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	css := string(files["/style.css"])
	for _, text := range []string{
		`font: 1rem/1.6 "Geist Serif", "Geist Serif Fallback", Times, "Times New Roman", serif`,
		`font-family: "Geist Serif Fallback"`, `src: local("Times New Roman"), local("Times")`, `size-adjust: 110%`,
		`:root[data-font="baskerville"] body { font-family: "Libre Baskerville"`,
		`:root[data-font="times"] body { font-family: Times`,
	} {
		if !strings.Contains(css, text) {
			t.Errorf("stylesheet is missing %s", text)
		}
	}
	for _, url := range assets {
		if len(files[url]) == 0 {
			t.Errorf("missing font asset %s", url)
		}
		if strings.HasSuffix(url, ".woff2") && !strings.Contains(css, `url("`+url+`")`) {
			t.Errorf("stylesheet does not reference %s", url)
		}
	}
}

func TestLoadDevEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BLOG_CONTENT_DIR", "")
	if err := os.Unsetenv("BLOG_CONTENT_DIR"); err != nil {
		t.Fatal(err)
	}
	if err := loadDevEnv(root); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	put(t, root, ".env.local", "BLOG_CONTENT_DIR=/vault/content\n")
	if err := loadDevEnv(root); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("BLOG_CONTENT_DIR"); got != "/vault/content" {
		t.Fatalf("content directory = %q", got)
	}
	t.Setenv("BLOG_CONTENT_DIR", "/shell/content")
	if err := loadDevEnv(root); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("BLOG_CONTENT_DIR"); got != "/shell/content" {
		t.Fatalf("shell value was overwritten: %q", got)
	}
	put(t, root, ".env.local", "not a valid assignment\n")
	if err := loadDevEnv(root); err == nil {
		t.Fatal("invalid env file was accepted")
	}
}

func TestExternalContentDirectory(t *testing.T) {
	root := fixture(t)
	content := filepath.Join(t.TempDir(), "vault")
	if err := os.Rename(filepath.Join(root, "content"), content); err != nil {
		t.Fatal(err)
	}
	put(t, root, "content/home/home.md", "not the vault")
	t.Setenv("BLOG_CONTENT_DIR", content)
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["/"]), "Hello <strong>world</strong>") {
		t.Fatal("build did not use external content")
	}
	before, err := fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	put(t, content, "writing/posts/new.md", "new")
	after, err := fingerprint(root)
	if err != nil || before == after {
		t.Fatalf("external content change was not detected: %v", err)
	}
}

func TestBookID(t *testing.T) {
	for title, want := range map[string]string{
		"The Autobiography of Malcolm X ": "the-autobiography-of-malcolm-x-",
		"Daisy Jones & The Six":           "daisy-jones-&-the-six",
		"Cat's Cradle":                    "cats-cradle",
	} {
		if got := bookID(title); got != want {
			t.Errorf("bookID(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestBookShelf(t *testing.T) {
	root := fixture(t)
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	page := string(files["/book-shelf/"])
	for _, want := range []string{
		"<h1>book shelf</h1>", "Books I have read.", "<h2>2026</h2>", "<h2>2020</h2>",
		`id="a-newers-book"`,
		`<header class="book-header"><h3><a href="#a-newers-book">A Newer&#39;s Book</a></h3> <span>by Second Author · <time datetime="2026-04-25">04.25</time></span></header>`,
		"<strong>Newer notes.</strong>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("book shelf lacks %q", want)
		}
	}
	newer, middle, older, oldest := strings.Index(page, "A Newer&#39;s Book"), strings.Index(page, "Middle Book"), strings.Index(page, "Older Book"), strings.Index(page, "Oldest Book")
	if strings.Count(page, "<h2>2026</h2>") != 1 || newer < 0 || newer >= middle || middle >= older || older >= oldest {
		t.Error("books are not in descending date order")
	}
	if !strings.Contains(page, "<h2>2018</h2>") || strings.Contains(page, `datetime="2018-12-31"`) {
		t.Error("dates before 2019 should not be displayed")
	}
	put(t, root, "content/book-shelf/books/bad.md", "---\ntitle: Bad\ndate: 2026-01-01\n---\nNotes\n")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "missing author") {
		t.Fatalf("missing author accepted: %v", err)
	}
	put(t, root, "content/book-shelf/books/bad.md", "---\ntitle: Bad\nauthor: Author\ndate: invalid\n---\nNotes\n")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Fatalf("invalid date accepted: %v", err)
	}
	put(t, root, "content/book-shelf/books/bad.md", "---\ntitle: \"A Newers Book\"\nauthor: Author\ndate: 2026-01-01\n---\nNotes\n")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "duplicate book anchor") {
		t.Fatalf("duplicate anchor accepted: %v", err)
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
	put(t, root, "content/writing/posts/first.md", "---\ntitle: First\ndate: invalid\n---\nBody")
	if _, err := build(root); err == nil {
		t.Fatal("invalid date accepted")
	}
	put(t, root, "content/writing/posts/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\nBody")
	put(t, root, "content/writing/newsletter/first.md", "---\ntitle: First\ndate: 2020-01-01\n---\nBody")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("duplicate post and newsletter slug not detected: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "content/writing/newsletter/first.md")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "public/writing/index.html", "conflict")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("output conflict not detected: %v", err)
	}
}

func TestPagePermalinksAndNavigation(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/writing/writing.md", "---\ntitle: Essays\npermalink: /essays/\n---\nIntroduction.\n")
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/essays/", "/essays/first/"} {
		if _, ok := files[url]; !ok {
			t.Errorf("missing %s", url)
		}
	}
	for _, want := range []string{`<a href="/essays/">Essays</a>`, `<a href="/essays/">all writing →</a>`, `<a href="/essays/first/">`} {
		if !strings.Contains(string(files["/"]), want) {
			t.Errorf("home lacks %q", want)
		}
	}
	if !strings.Contains(string(files["/essays/"]), "<title>Essays | Test Blog</title>") || !strings.Contains(string(files["/essays/"]), "<h1>Essays</h1>") {
		t.Error("writing title does not come from content")
	}
}

func TestInvalidPageMetadata(t *testing.T) {
	for _, permalink := range []string{"", "relative/", "/no-slash", "/../escape/", "/a/../b/", "/%2e%2e/", "/query/?q=x", "/__reload/", "/writing/"} {
		t.Run(permalink, func(t *testing.T) {
			root := fixture(t)
			put(t, root, "content/extra/extra.md", "---\ntitle: Extra\npermalink: "+permalink+"\n---\nBody\n")
			if _, err := build(root); err == nil {
				t.Fatalf("accepted permalink %q", permalink)
			}
		})
	}
}

func TestRawContent(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/resume.md", "---\r\nrender: raw\r\n---\r\n\r\n# Resume\r\n")
	put(t, root, "content/private.md", "---\nrender: none\n---\nPrivate\n")
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(files["/resume.md"]) != "\r\n# Resume\r\n" {
		t.Errorf("raw body changed: %q", files["/resume.md"])
	}
	if _, ok := files["/private.md"]; ok {
		t.Error("unmarked root content was published")
	}
	put(t, root, "public/resume.md", "duplicate")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("duplicate raw output accepted: %v", err)
	}
}

func TestFingerprint(t *testing.T) {
	root := fixture(t)
	before, err := fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "content/writing/posts/new.md", "new")
	after, err := fingerprint(root)
	if err != nil || before == after {
		t.Fatalf("addition was not detected: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "content/writing/posts/new.md")); err != nil {
		t.Fatal(err)
	}
	last, err := fingerprint(root)
	if err != nil || last == after {
		t.Fatalf("deletion was not detected: %v", err)
	}
	for _, name := range []string{"templates/layout.html", "public/style.css"} {
		put(t, root, name, "changed")
		next, err := fingerprint(root)
		if err != nil || next == last {
			t.Fatalf("change to %s was not detected: %v", name, err)
		}
		last = next
	}
}

func TestBuildRequiresStylesheet(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "public/style.css")); err != nil {
		t.Fatal(err)
	}
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "missing public/style.css") {
		t.Fatalf("missing stylesheet accepted: %v", err)
	}
}
