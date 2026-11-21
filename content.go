package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

type article struct {
	Title, Description, Date, URL, Substack, Author string
	Permalink, Render                               string
	Published                                       *bool
	Body                                            template.HTML
}

type yearGroup struct {
	Year     string
	Articles []article
}

type book struct {
	article
	ID string
}

type bookYear struct {
	Year  string
	Books []book
}

type navLink struct{ Title, URL string }

type page struct {
	Title, Kind, SiteTitle, WritingURL string
	Description, Canonical             string
	Nav                                []navLink
	Article                            article
	Years                              []yearGroup
	BookYears                          []bookYear
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func readContent(file string) (article, []byte, error) {
	var a article
	data, err := os.ReadFile(file)
	if err != nil {
		return a, nil, err
	}
	first := bytes.IndexByte(data, '\n')
	if first < 0 || string(bytes.TrimSuffix(data[:first], []byte("\r"))) != "---" {
		return a, nil, fmt.Errorf("%s: missing YAML front matter", file)
	}
	for start := first + 1; start < len(data); {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			break
		}
		end += start
		if string(bytes.TrimSuffix(data[start:end], []byte("\r"))) == "---" {
			if err := yaml.Unmarshal(data[first+1:start], &a); err != nil {
				return a, nil, fmt.Errorf("%s: %w", file, err)
			}
			return a, data[end+1:], nil
		}
		start = end + 1
	}
	return a, nil, fmt.Errorf("%s: missing YAML front matter", file)
}

func readArticle(file, url, kind string) (article, error) {
	a, body, err := readContent(file)
	if err != nil {
		return a, err
	}
	if a.Title == "" {
		return a, fmt.Errorf("%s: missing title", file)
	}
	if kind != "page" {
		if _, err := time.Parse("2006-01-02", a.Date); err != nil {
			return a, fmt.Errorf("%s: invalid date %q", file, a.Date)
		}
	}
	body, err = stripComments(body)
	if err != nil {
		return a, fmt.Errorf("%s: %w", file, err)
	}
	var rendered bytes.Buffer
	if err := markdown.Convert(body, &rendered); err != nil {
		return a, fmt.Errorf("%s: %w", file, err)
	}
	a.Body, a.URL = template.HTML(rendered.String()), url // Content is trusted source HTML.
	return a, nil
}

func groupByYear(articles []article) []yearGroup {
	var years []yearGroup
	for _, a := range articles {
		year := a.Date[:4]
		if len(years) == 0 || years[len(years)-1].Year != year {
			years = append(years, yearGroup{Year: year})
		}
		last := &years[len(years)-1]
		last.Articles = append(last.Articles, a)
	}
	return years
}

// contentDir lets a build use content outside the source checkout.
func contentDir(root string) string {
	if dir := os.Getenv("BLOG_CONTENT_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(root, "content")
}

var bookWhitespace = regexp.MustCompile(`[\s\p{Z}]+`)

// bookID preserves the anchors used by the original book shelf.
func bookID(title string) string {
	clean := strings.NewReplacer("!", "", "'", "", "’", "", ".", "", "(", "", ")", "", "*", "", ":", "").Replace(title)
	return strings.ToLower(bookWhitespace.ReplaceAllString(clean, "-"))
}

func readBooks(content string) ([]bookYear, error) {
	matches, err := filepath.Glob(filepath.Join(content, "book-shelf/books/*.md"))
	if err != nil {
		return nil, err
	}
	var books []book
	ids := make(map[string]string)
	for _, file := range matches {
		a, err := readArticle(file, "", "book")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(a.Author) == "" {
			return nil, fmt.Errorf("%s: missing author", file)
		}
		id := bookID(a.Title)
		if id == "" {
			return nil, fmt.Errorf("%s: empty book anchor", file)
		}
		if previous, ok := ids[id]; ok {
			return nil, fmt.Errorf("%s and %s: duplicate book anchor %q", previous, file, id)
		}
		ids[id] = file
		books = append(books, book{article: a, ID: id})
	}
	sort.Slice(books, func(i, j int) bool {
		if books[i].Date == books[j].Date {
			return books[i].Title < books[j].Title
		}
		return books[i].Date > books[j].Date
	})
	var years []bookYear
	for _, b := range books {
		year := b.Date[:4]
		if len(years) == 0 || years[len(years)-1].Year != year {
			years = append(years, bookYear{Year: year})
		}
		last := &years[len(years)-1]
		last.Books = append(last.Books, b)
	}
	return years, nil
}
