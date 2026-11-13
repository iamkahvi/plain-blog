package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	stdhtml "html"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

type article struct {
	Title, Description, Date, URL string
	Published                     *bool
	Body                          template.HTML
	Kind                          string
}

type page struct {
	Title, Kind string
	Article     article
	Articles    []article
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func readArticle(file, url, kind string) (article, error) {
	var a article
	data, err := os.ReadFile(file)
	if err != nil {
		return a, err
	}
	parts := bytes.SplitN(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("---\n"), 3)
	if len(parts) != 3 || len(parts[0]) != 0 {
		return a, fmt.Errorf("%s: missing YAML front matter", file)
	}
	if err := yaml.Unmarshal(parts[1], &a); err != nil {
		return a, fmt.Errorf("%s: %w", file, err)
	}
	if a.Title == "" {
		return a, fmt.Errorf("%s: missing title", file)
	}
	if kind != "page" {
		if _, err := time.Parse("2006-01-02", a.Date); err != nil {
			return a, fmt.Errorf("%s: invalid date %q", file, a.Date)
		}
	}
	var body bytes.Buffer
	if err := markdown.Convert(parts[2], &body); err != nil {
		return a, fmt.Errorf("%s: %w", file, err)
	}
	lower, err := lowerHeadings(body.Bytes())
	if err != nil {
		return a, fmt.Errorf("%s: %w", file, err)
	}
	a.Body, a.URL, a.Kind = template.HTML(lower), url, kind // Content is trusted source HTML.
	return a, nil
}

func lowerHeadings(body []byte) (string, error) {
	z := xhtml.NewTokenizer(bytes.NewReader(body))
	var out bytes.Buffer
	heading, code := 0, 0
	for {
		token := z.Next()
		switch token {
		case xhtml.ErrorToken:
			if z.Err() == io.EOF {
				return out.String(), nil
			}
			return "", z.Err()
		case xhtml.StartTagToken, xhtml.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			delta := 1
			if token == xhtml.EndTagToken {
				delta = -1
			}
			if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
				if delta > 0 || heading > 0 {
					heading += delta
				}
				if heading == 0 {
					code = 0
				}
			}
			if heading > 0 && (tag == "code" || tag == "pre" || tag == "script" || tag == "style") && (delta > 0 || code > 0) {
				code += delta
			}
			out.Write(z.Raw())
		case xhtml.TextToken:
			if heading > 0 && code == 0 {
				out.WriteString(stdhtml.EscapeString(strings.ToLower(string(z.Text()))))
			} else {
				out.Write(z.Raw())
			}
		default:
			out.Write(z.Raw())
		}
	}
}

// build returns URL paths and their contents. It does not modify the output directory.
func build(root string) (map[string][]byte, error) {
	layout, err := template.New("layout.html").Funcs(template.FuncMap{
		"lower": strings.ToLower,
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
		if url == "/__reload" {
			return fmt.Errorf("reserved output path: %s", url)
		}
		outputs[name] = url
		files[url] = data
		return nil
	}
	render := func(url string, p page) error {
		var out bytes.Buffer
		if err := layout.Execute(&out, p); err != nil {
			return err
		}
		return add(url, out.Bytes())
	}
	about, err := readArticle(filepath.Join(root, "content/about/about.md"), "/about/", "page")
	if err != nil {
		return nil, err
	}
	pronounce, err := readArticle(filepath.Join(root, "content/pronounce/pronounce.md"), "/pronounce/", "page")
	if err != nil {
		return nil, err
	}
	var articles []article
	for _, group := range []struct{ dir, prefix, kind string }{
		{"posts", "/writing/", "post"}, {"newsletter", "/writing/", "newsletter"},
	} {
		matches, err := filepath.Glob(filepath.Join(root, "content", group.dir, "*.md"))
		if err != nil {
			return nil, err
		}
		for _, file := range matches {
			slug := strings.TrimSuffix(filepath.Base(file), ".md")
			if slug == "" || slug == "." || slug == ".." {
				return nil, fmt.Errorf("%s: invalid filename", file)
			}
			a, err := readArticle(file, group.prefix+slug+"/", group.kind)
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
	for _, item := range []struct {
		url string
		p   page
	}{
		{"/", page{Title: "Home", Kind: "home", Article: about, Articles: recent}},
		{"/about/", page{Title: about.Title, Kind: "page", Article: about}},
		{"/pronounce/", page{Title: pronounce.Title, Kind: "page", Article: pronounce}},
		{"/writing/", page{Title: "Writing", Kind: "writing", Articles: articles}},
	} {
		if err := render(item.url, item.p); err != nil {
			return nil, err
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
	for _, name := range []string{"content", "public", "layout.html", "style.css"} {
		file := filepath.Join(root, name)
		if _, err := os.Stat(file); os.IsNotExist(err) && name == "public" {
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

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "build" && os.Args[1] != "dev") {
		log.Fatal("usage: go run . [build|dev]")
	}
	if os.Args[1] == "dev" {
		log.Fatal(serve(".", "127.0.0.1:8080"))
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
