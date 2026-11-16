package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
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
	}).ParseFiles(filepath.Join(root, "layout.html"))
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
	css, err := os.ReadFile(filepath.Join(root, "style.css"))
	if err != nil {
		return nil, err
	}
	if err := add("/style.css", css); err != nil {
		return nil, err
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

// fingerprint detects file additions, edits, and deletions without a platform-specific watcher.
func fingerprint(root string) (string, error) {
	h := sha256.New()
	for _, file := range []string{contentDir(root), filepath.Join(root, "public"), filepath.Join(root, "layout.html"), filepath.Join(root, "style.css")} {
		if _, err := os.Stat(file); os.IsNotExist(err) && file == filepath.Join(root, "public") {
			continue
		} else if err != nil {
			return "", err
		}
		if err := filepath.WalkDir(file, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano(), info.Mode())
			return nil
		}); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func serve(root, addr string) error {
	files, err := build(root)
	if err != nil {
		return err
	}
	stamp, err := fingerprint(root)
	if err != nil {
		return err
	}
	var mu sync.RWMutex
	version := 1
	go func() {
		for range time.Tick(400 * time.Millisecond) {
			next, err := fingerprint(root)
			if err != nil {
				log.Print(err)
				continue
			}
			if next == stamp {
				continue
			}
			stamp = next
			updated, err := build(root)
			if err != nil {
				log.Printf("build failed: %v", err)
				continue
			}
			mu.Lock()
			files = updated
			version++
			mu.Unlock()
			log.Print("site rebuilt")
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		currentFiles, currentVersion := files, version
		mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/__reload" {
			fmt.Fprint(w, currentVersion)
			return
		}
		url := path.Clean(r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/") && url != "/" {
			url += "/"
		}
		data, ok := currentFiles[url]
		if !ok && strings.HasSuffix(url, "/index.html") {
			url = strings.TrimSuffix(url, "index.html")
			data, ok = currentFiles[url]
		}
		if !ok && !strings.HasSuffix(url, "/") {
			if _, ok := currentFiles[url+"/"]; ok {
				http.Redirect(w, r, url+"/", http.StatusMovedPermanently)
				return
			}
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(url, "/") || strings.HasSuffix(url, ".html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			script := fmt.Sprintf(`<script>setInterval(async()=>{try{if(await(await fetch('/__reload',{cache:'no-store'})).text()!==%q)location.reload()}catch(e){}},500)</script>`, fmt.Sprint(currentVersion))
			data = bytes.Replace(data, []byte("</body>"), []byte(script+"</body>"), 1)
		} else if path.Ext(url) == ".m4a" {
			w.Header().Set("Content-Type", "audio/mp4")
		} else if typ := mime.TypeByExtension(path.Ext(url)); typ != "" {
			w.Header().Set("Content-Type", typ)
		}
		http.ServeContent(w, r, path.Base(url), time.Time{}, bytes.NewReader(data))
	})
	log.Printf("serving http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

// loadDevEnv loads optional local settings without overriding the shell environment.
func loadDevEnv(root string) error {
	err := godotenv.Load(filepath.Join(root, ".env.local"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "build" && os.Args[1] != "build-artifacts" && os.Args[1] != "dev") {
		log.Fatal("usage: go run . [build|build-artifacts|dev]")
	}
	if os.Args[1] == "dev" {
		if err := loadDevEnv("."); err != nil {
			log.Fatal(err)
		}
		log.Fatal(serve(".", "127.0.0.1:8080"))
	}
	if os.Args[1] == "build-artifacts" {
		if err := buildFromArtifacts("."); err != nil {
			log.Fatal(err)
		}
		return
	}
	files, err := build(".")
	if err == nil {
		err = writeSite(".", files)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d files to dist/", len(files))
}
