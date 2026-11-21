package main

import (
	"encoding/xml"
	"html/template"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSEOMetadataAndSitemap(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/writing/posts/first.md", "---\ntitle: 'First & best'\ndate: 2020-01-01\ndescription: 'A <good> & \"useful\" post'\n---\nBody.\n")
	put(t, root, "content/writing/posts/café & tea.md", "---\ntitle: Tea\ndate: 2020-01-01\n---\nTea.\n")
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, description, canonical string
	}{
		{"/", "Hello world.", "https://test.example.org/"},
		{"/writing/", "Writing introduction.", "https://test.example.org/writing/"},
		{"/writing/first/", "A &lt;good&gt; &amp; &#34;useful&#34; post", "https://test.example.org/writing/first/"},
		{"/writing/café & tea/", "Tea.", "https://test.example.org/writing/caf%C3%A9%20&amp;%20tea/"},
	} {
		page := string(files[tc.path])
		if !strings.Contains(page, `<meta name="description" content="`+tc.description+`">`) {
			t.Errorf("%s: incorrect description: %s", tc.path, page)
		}
		if !strings.Contains(page, `<link rel="canonical" href="`+tc.canonical+`">`) {
			t.Errorf("%s: incorrect canonical: %s", tc.path, page)
		}
	}
	if page := string(files["/writing/february/"]); strings.Contains(page, `rel="canonical"`) || !strings.Contains(page, `<meta name="description"`) {
		t.Errorf("Substack page should have a description but no canonical: %s", page)
	}
	var sitemap struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(files["/sitemap.xml"], &sitemap); err != nil {
		t.Fatal(err)
	}
	if sitemap.XMLName.Space != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Errorf("wrong sitemap namespace: %q", sitemap.XMLName.Space)
	}
	locations := make(map[string]bool)
	for i, item := range sitemap.URLs {
		locations[item.Location] = true
		if i > 0 && sitemap.URLs[i-1].Location >= item.Location {
			t.Errorf("sitemap is not sorted: %q", item.Location)
		}
	}
	for _, location := range []string{"https://test.example.org/", "https://test.example.org/writing/first/", "https://test.example.org/writing/caf%C3%A9%20&%20tea/"} {
		if !locations[location] {
			t.Errorf("sitemap missing %s", location)
		}
	}
	for _, location := range []string{"https://test.example.org/writing/february/", "https://test.example.org/writing/draft/", "https://test.example.org/resume.md"} {
		if locations[location] {
			t.Errorf("sitemap unexpectedly includes %s", location)
		}
	}
}

func TestArticleDescription(t *testing.T) {
	for _, tc := range []struct {
		name, description, body, want string
	}{
		{"explicit", "  Written by the author  ", "<p>Other text.</p>", "Written by the author"},
		{"markup", "", `<p>Hello <strong>world</strong> &amp; <a href="/">friends</a>.</p><p>Second.</p>`, "Hello world & friends."},
		{"empty paragraph", "", `<p><img src="x"></p><p>Useful text.</p>`, "Useful text."},
		{"line breaks", "", `<p>First<br>second<br/>third.</p>`, "First second third."},
		{"script", "", `<p>Hello <script>bad()</script>world.</p>`, "Hello world."},
		{"no paragraph", "", `<h2>Heading</h2>`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := articleDescription(article{Description: tc.description, Body: template.HTML(tc.body)})
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
	for _, text := range []string{strings.Repeat("é", 200), strings.Repeat("long words ", 30)} {
		got := excerpt(text)
		if !utf8.ValidString(got) || utf8.RuneCountInString(got) > 160 || !strings.HasSuffix(got, "…") {
			t.Errorf("invalid excerpt: %q", got)
		}
	}
}

func TestSitemapConflict(t *testing.T) {
	root := fixture(t)
	put(t, root, "public/sitemap.xml", "existing sitemap")
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "output conflict") {
		t.Fatalf("sitemap conflict was not detected: %v", err)
	}
}
