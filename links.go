package main

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const externalLinkIcon = `<svg class="external-link-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M13 9v4H3V3h4"/></svg>`

// externalLinks marks external web links and adds new-tab attributes by default.
func externalLinks(page []byte) ([]byte, error) {
	var out bytes.Buffer
	var anchors []string
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
		switch typ {
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			if token.Data != "a" {
				continue
			}
			var suffix string
			changed := false
			if externalWebLink(&token) {
				changed = newTabLink(&token)
				marked, newTab, foundTarget := false, false, false
				for _, attr := range token.Attr {
					if attr.Key == "data-external-link-icon" {
						marked = true
					}
					if attr.Key == "target" && !foundTarget {
						newTab = strings.EqualFold(attr.Val, "_blank")
						foundTarget = true
					}
				}
				if !marked && typ == html.StartTagToken {
					token.Attr = append(token.Attr, html.Attribute{Key: "data-external-link-icon"})
					changed = true
					description := "external link"
					if newTab {
						description += ", opens in a new tab"
					}
					suffix = externalLinkIcon + `<span class="visually-hidden"> (` + description + `)</span>`
				}
			}
			if typ == html.StartTagToken {
				anchors = append(anchors, suffix)
			}
			if changed {
				out.Truncate(start)
				out.WriteString(token.String())
			}
		case html.EndTagToken:
			endTag := string(out.Bytes()[start:])
			if token := z.Token(); token.Data == "a" && len(anchors) > 0 {
				suffix := anchors[len(anchors)-1]
				anchors = anchors[:len(anchors)-1]
				if suffix != "" {
					out.Truncate(start)
					out.WriteString(suffix)
					out.WriteString(endTag)
				}
			}
		}
	}
}

func externalWebLink(token *html.Token) bool {
	for _, attr := range token.Attr {
		if attr.Key == "href" {
			u, err := url.Parse(strings.TrimSpace(attr.Val))
			if err != nil || u.Host == "" || (u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
				return false
			}
			host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
			return host != "kahvipatel.com" && host != "www.kahvipatel.com"
		}
	}
	return false
}

func newTabLink(token *html.Token) bool {
	target, rel := -1, -1
	for i, attr := range token.Attr {
		switch attr.Key {
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
