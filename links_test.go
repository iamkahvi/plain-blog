package main

import (
	"strings"
	"testing"
)

const testExternalIcon = `<svg class="external-link-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M13 9v4H3V3h4"/></svg>`

func markedLink(link string, newTab bool) string {
	description := "external link"
	if newTab {
		description += ", opens in a new tab"
	}
	link = strings.Replace(link, ">", ` data-external-link-icon="">`, 1)
	return strings.Replace(link, "</a>", testExternalIcon+`<span class="visually-hidden"> (`+description+`)</span></a>`, 1)
}

func TestExternalLinks(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"external", `<a href="https://example.com/?a=1&amp;b=2">link</a>`, markedLink(`<a href="https://example.com/?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"http", `<a href="http://example.com">link</a>`, markedLink(`<a href="http://example.com" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"protocol relative", `<a href="//example.com/path">link</a>`, markedLink(`<a href="//example.com/path" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"subdomain", `<a href="https://archive.kahvipatel.com">link</a>`, markedLink(`<a href="https://archive.kahvipatel.com" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"lookalike host", `<a href="https://kahvipatel.com.example.org">link</a>`, markedLink(`<a href="https://kahvipatel.com.example.org" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"relative", `<a href="/writing/">link</a>`, `<a href="/writing/">link</a>`},
		{"fragment", `<a href="#section">link</a>`, `<a href="#section">link</a>`},
		{"canonical host", `<a href="https://kahvipatel.com/page">link</a>`, `<a href="https://kahvipatel.com/page">link</a>`},
		{"www host", `<a href="//WWW.KAHVIPATEL.COM./page">link</a>`, `<a href="//WWW.KAHVIPATEL.COM./page">link</a>`},
		{"mail", `<a href="mailto:hi@example.com">link</a>`, `<a href="mailto:hi@example.com">link</a>`},
		{"other scheme", `<a href="ftp://example.com/file">link</a>`, `<a href="ftp://example.com/file">link</a>`},
		{"invalid", `<a href="http://[bad">link</a>`, `<a href="http://[bad">link</a>`},
		{"no href", `<a id="section">link</a>`, `<a id="section">link</a>`},
		{"explicit target", `<a href="https://example.com" target="_self">link</a>`, markedLink(`<a href="https://example.com" target="_self">link</a>`, false)},
		{"blank target", `<a href="https://example.com" target="_blank">link</a>`, markedLink(`<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"existing rel", `<a href="https://example.com" rel="nofollow NOOPENER">link</a>`, markedLink(`<a href="https://example.com" rel="nofollow NOOPENER noreferrer" target="_blank">link</a>`, true)},
		{"complete attributes", `<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`, markedLink(`<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`, true)},
		{"inline content", `<a href="https://example.com"><strong>link</strong></a>`, markedLink(`<a href="https://example.com" target="_blank" rel="noopener noreferrer"><strong>link</strong></a>`, true)},
		{"duplicate href", `<a href="" href="https://example.com">link</a>`, `<a href="" href="https://example.com">link</a>`},
		{"unchanged html", `<DIV DATA-X="a&amp;b"><a href="/?a=1&amp;b=2">link</a><script>const x = '<a href="https://example.com">';</script></DIV>`, `<DIV DATA-X="a&amp;b"><a href="/?a=1&amp;b=2">link</a><script>const x = '<a href="https://example.com">';</script></DIV>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := externalLinks([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			again, err := externalLinks(got)
			if err != nil || string(again) != string(got) {
				t.Errorf("second pass changed output: %q, %v", again, err)
			}
		})
	}
}

func TestBuildExternalLinks(t *testing.T) {
	root := fixture(t)
	put(t, root, "content/about/about.md", "---\ntitle: about\n---\n"+`[External](https://example.com) [Internal](/writing/) https://example.org

<a href="https://example.net" rel="author">raw link</a>
`)
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"/", "/about/"} {
		for _, want := range []string{
			markedLink(`<a href="https://example.com" target="_blank" rel="noopener noreferrer">External</a>`, true),
			`<a href="/writing/">Internal</a>`,
			markedLink(`<a href="https://example.org" target="_blank" rel="noopener noreferrer">https://example.org</a>`, true),
			markedLink(`<a href="https://example.net" rel="author noopener noreferrer" target="_blank">raw link</a>`, true),
		} {
			if !strings.Contains(string(files[page]), want) {
				t.Errorf("%s lacks %q", page, want)
			}
		}
	}
}
