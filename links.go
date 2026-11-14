package main

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// externalLinks adds new-tab attributes to external links in a rendered page.
func externalLinks(page []byte) ([]byte, error) {
	var out bytes.Buffer
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		typ := z.Next()
		if typ == html.ErrorToken {
			if err := z.Err(); err != io.EOF {
				return nil, err
			}
			return out.Bytes(), nil
		}
		start := out.Len()
		out.Write(z.Raw())
		if typ == html.StartTagToken || typ == html.SelfClosingTagToken {
			token := z.Token()
			if token.Data == "a" && newTabLink(&token) {
				out.Truncate(start)
				out.WriteString(token.String())
			}
		}
	}
}

func newTabLink(token *html.Token) bool {
	href, target, rel := -1, -1, -1
	for i, attr := range token.Attr {
		switch attr.Key {
		case "href":
			if href == -1 {
				href = i
			}
		case "target":
			if target == -1 {
				target = i
			}
		case "rel":
			if rel == -1 {
				rel = i
			}
		}
	}
	if href == -1 {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(token.Attr[href].Val))
	if err != nil || u.Host == "" || (u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "kahvipatel.com" || host == "www.kahvipatel.com" {
		return false
	}
	if target != -1 && !strings.EqualFold(token.Attr[target].Val, "_blank") {
		return false
	}
	changed := false
	if target == -1 {
		token.Attr = append(token.Attr, html.Attribute{Key: "target", Val: "_blank"})
		changed = true
	}
	if rel == -1 {
		token.Attr = append(token.Attr, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
		return true
	}
	values := strings.Fields(token.Attr[rel].Val)
	for _, value := range []string{"noopener", "noreferrer"} {
		found := false
		for _, existing := range values {
			if strings.EqualFold(existing, value) {
				found = true
				break
			}
		}
		if !found {
			values = append(values, value)
			changed = true
		}
	}
	if changed {
		token.Attr[rel].Val = strings.Join(values, " ")
	}
	return changed
}
