package main

import (
	"strings"
	"testing"
)

func TestExternalLinks(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"external", `<a href="https://example.com/?a=1&amp;b=2">link</a>`, `<a href="https://example.com/?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">link</a>`},
		{"http", `<a href="http://example.com">link</a>`, `<a href="http://example.com" target="_blank" rel="noopener noreferrer">link</a>`},
		{"protocol relative", `<a href="//example.com/path">link</a>`, `<a href="//example.com/path" target="_blank" rel="noopener noreferrer">link</a>`},
		{"subdomain", `<a href="https://archive.kahvipatel.com">link</a>`, `<a href="https://archive.kahvipatel.com" target="_blank" rel="noopener noreferrer">link</a>`},
		{"lookalike host", `<a href="https://kahvipatel.com.example.org">link</a>`, `<a href="https://kahvipatel.com.example.org" target="_blank" rel="noopener noreferrer">link</a>`},
		{"relative", `<a href="/writing/">link</a>`, `<a href="/writing/">link</a>`},
		{"fragment", `<a href="#section">link</a>`, `<a href="#section">link</a>`},
		{"canonical host", `<a href="https://kahvipatel.com/page">link</a>`, `<a href="https://kahvipatel.com/page">link</a>`},
		{"www host", `<a href="//WWW.KAHVIPATEL.COM./page">link</a>`, `<a href="//WWW.KAHVIPATEL.COM./page">link</a>`},
		{"mail", `<a href="mailto:hi@example.com">link</a>`, `<a href="mailto:hi@example.com">link</a>`},
		{"other scheme", `<a href="ftp://example.com/file">link</a>`, `<a href="ftp://example.com/file">link</a>`},
		{"invalid", `<a href="http://[bad">link</a>`, `<a href="http://[bad">link</a>`},
		{"no href", `<a id="section">link</a>`, `<a id="section">link</a>`},
		{"explicit target", `<a href="https://example.com" target="_self">link</a>`, `<a href="https://example.com" target="_self">link</a>`},
		{"blank target", `<a href="https://example.com" target="_blank">link</a>`, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`},
		{"existing rel", `<a href="https://example.com" rel="nofollow NOOPENER">link</a>`, `<a href="https://example.com" rel="nofollow NOOPENER noreferrer" target="_blank">link</a>`},
		{"complete attributes", `<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`, `<a href="https://example.com" target="_blank" rel="noopener noreferrer">link</a>`},
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
			`<a href="https://example.com" target="_blank" rel="noopener noreferrer">External</a>`,
			`<a href="/writing/">Internal</a>`,
			`<a href="https://example.org" target="_blank" rel="noopener noreferrer">https://example.org</a>`,
			`<a href="https://example.net" rel="author noopener noreferrer" target="_blank">raw link</a>`,
		} {
			if !strings.Contains(string(files[page]), want) {
				t.Errorf("%s lacks %q", page, want)
			}
		}
	}
}
