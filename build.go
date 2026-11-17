package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// build returns URL paths and their contents. It does not modify the output directory.
func build(root string) (map[string][]byte, error) {
	return buildWithContent(root, contentDir(root))
}

func buildWithContent(root, content string) (map[string][]byte, error) {
	layout, err := template.New("layout.html").Funcs(template.FuncMap{
		"shortDate": func(value string) string { return value[5:7] + "." + value[8:10] },
		"date": func(value string) (string, error) {
			d, err := time.Parse("2006-01-02", value)
			return strings.ToLower(d.Format("January 2, 2006")), err
		},
	}).ParseFiles(filepath.Join(root, "templates", "layout.html"))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte)
	outputs := make(map[string]string)
	add := func(url string, data []byte) error {
		name := outputName(url)
		for existing, source := range outputs {
			if name == existing || strings.HasPrefix(name, existing+"/") || strings.HasPrefix(existing, name+"/") {
				return fmt.Errorf("output conflict: %s and %s", source, url)
			}
		}
		if url == "/__reload" || url == "/__reload/" {
			return fmt.Errorf("reserved output path: %s", url)
		}
		outputs[name] = url
		files[url] = data
		return nil
	}
	pages := make(map[string]article)
	matches, err := filepath.Glob(filepath.Join(content, "*", "*.md"))
	if err != nil {
		return nil, err
	}
	for _, file := range matches {
		name := filepath.Base(filepath.Dir(file))
		if filepath.Base(file) != name+".md" {
			continue
		}
		a, err := readArticle(file, "", "page")
		if err != nil {
			return nil, err
		}
		if a.Permalink == "" || a.Permalink[0] != '/' || !strings.HasSuffix(a.Permalink, "/") ||
			strings.ContainsAny(a.Permalink, "\\?#%\x00\r\n\t") || (a.Permalink != "/" && path.Clean(a.Permalink) != strings.TrimSuffix(a.Permalink, "/")) {
			return nil, fmt.Errorf("%s: invalid permalink %q", file, a.Permalink)
		}
		a.URL = a.Permalink
		pages[name] = a
	}
	home, ok := pages["home"]
	if !ok || home.URL != "/" {
		return nil, fmt.Errorf("home/home.md must have permalink /")
	}
	var nav []navLink
	for _, name := range []string{"home", "writing", "book-shelf", "projects"} {
		a, ok := pages[name]
		if !ok {
			return nil, fmt.Errorf("missing navigation page %s/%s.md", name, name)
		}
		title := a.Title
		if name == "home" {
			title = "home"
		}
		nav = append(nav, navLink{title, a.URL})
	}
	render := func(url string, p page) error {
		p.SiteTitle, p.WritingURL, p.Nav = home.Title, pages["writing"].URL, nav
		var out bytes.Buffer
		if err := layout.Execute(&out, p); err != nil {
			return err
		}
		data, err := externalLinks(out.Bytes())
		if err != nil {
			return err
		}
		return add(url, data)
	}
	bookYears, err := readBooks(content)
	if err != nil {
		return nil, err
	}
	var articles []article
	for _, dir := range []string{"writing/posts", "writing/newsletter"} {
		matches, err := filepath.Glob(filepath.Join(content, dir, "*.md"))
		if err != nil {
			return nil, err
		}
		for _, file := range matches {
			slug := strings.TrimSuffix(filepath.Base(file), ".md")
			if slug == "" || slug == "." || slug == ".." {
				return nil, fmt.Errorf("%s: invalid filename", file)
			}
			a, err := readArticle(file, pages["writing"].URL+slug+"/", "article")
			if err != nil {
				return nil, err
			}
			if a.Published != nil && !*a.Published {
				continue
			}
			articles = append(articles, a)
			if err := render(a.URL, page{Title: a.Title, Kind: "article", Article: a}); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].Date == articles[j].Date {
			return articles[i].URL < articles[j].URL
		}
		return articles[i].Date > articles[j].Date
	})
	recent := articles
	if len(recent) > 5 {
		recent = recent[:5]
	}
	for name, a := range pages {
		p := page{Title: a.Title, Kind: "page", Article: a}
		switch name {
		case "home":
			p.Kind, p.Years = "home", groupByYear(recent)
		case "writing":
			p.Kind, p.Years = "writing", groupByYear(articles)
		case "book-shelf":
			p.Kind, p.BookYears = "books", bookYears
		}
		if err := render(a.URL, p); err != nil {
			return nil, err
		}
	}
	rawFiles, err := filepath.Glob(filepath.Join(content, "*.md"))
	if err != nil {
		return nil, err
	}
	for _, file := range rawFiles {
		a, body, err := readContent(file)
		if err != nil {
			return nil, err
		}
		if a.Render == "raw" {
			if err := add("/"+filepath.Base(file), body); err != nil {
				return nil, err
			}
		}
	}
	public := filepath.Join(root, "public")
	if _, err := os.Stat(public); err == nil {
		err = filepath.WalkDir(public, func(file string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("%s: not a regular file", file)
			}
			rel, err := filepath.Rel(public, file)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			return add("/"+filepath.ToSlash(rel), data)
		})
		if err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, ok := files["/style.css"]; !ok {
		return nil, fmt.Errorf("missing public/style.css")
	}
	return files, nil
}

func outputName(url string) string {
	name := strings.TrimPrefix(url, "/")
	if strings.HasSuffix(url, "/") {
		name += "index.html"
	}
	return name
}

func writeSite(root string, files map[string][]byte) error {
	out := filepath.Join(root, "dist")
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	for url, data := range files {
		file := filepath.Join(out, filepath.FromSlash(outputName(url)))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

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
			return true
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
