package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

func canonicalURL(domain, route string) string {
	return (&url.URL{Scheme: "https", Host: domain, Path: route}).String()
}

// articleDescription uses an explicit description or the first paragraph of the page.
func articleDescription(a article) string {
	if description := strings.TrimSpace(a.Description); description != "" {
		return description
	}
	z := html.NewTokenizer(strings.NewReader(string(a.Body)))
	var text strings.Builder
	inParagraph, ignored := false, 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				return ""
			}
			return excerpt(text.String())
		case html.StartTagToken:
			token := z.Token()
			if !inParagraph && token.Data == "p" {
				inParagraph = true
			} else if inParagraph && (token.Data == "script" || token.Data == "style") {
				ignored++
			} else if inParagraph && token.Data == "br" && ignored == 0 {
				text.WriteByte(' ')
			}
		case html.SelfClosingTagToken:
			if inParagraph && z.Token().Data == "br" {
				text.WriteByte(' ')
			}
		case html.EndTagToken:
			token := z.Token()
			if inParagraph && (token.Data == "script" || token.Data == "style") && ignored > 0 {
				ignored--
			} else if inParagraph && token.Data == "p" {
				if description := excerpt(text.String()); description != "" {
					return description
				}
				inParagraph = false
				text.Reset()
			}
		case html.TextToken:
			if inParagraph && ignored == 0 {
				text.Write(z.Text())
			}
		}
	}
}

func excerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	const limit = 160
	if len(runes) <= limit {
		return text
	}
	cut := limit - 1
	for cut > 0 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	if cut == 0 {
		cut = limit - 1
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

func makeSitemap(locations []string) ([]byte, error) {
	type entry struct {
		Location string `xml:"loc"`
	}
	type urlset struct {
		XMLName xml.Name `xml:"urlset"`
		Xmlns   string   `xml:"xmlns,attr"`
		URLs    []entry  `xml:"url"`
	}
	sort.Strings(locations)
	sitemap := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, location := range locations {
		sitemap.URLs = append(sitemap.URLs, entry{Location: location})
	}
	data, err := xml.MarshalIndent(sitemap, "", "  ")
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{[]byte(xml.Header), data, []byte("\n")}, nil), nil
}
