package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// fingerprint detects file additions, edits, and deletions without a platform-specific watcher.
func fingerprint(root string) (string, error) {
	h := sha256.New()
	for _, file := range []string{contentDir(root), filepath.Join(root, "public"), filepath.Join(root, "templates", "layout.html")} {
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
