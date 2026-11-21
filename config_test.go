package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteDomain(t *testing.T) {
	for _, tc := range []struct {
		name, config, want string
	}{
		{"valid", "domain: blog.example.org\n", "blog.example.org"},
		{"single label", "domain: localhost\n", "localhost"},
		{"missing domain", "{}", ""},
		{"empty domain", "domain: ''\n", ""},
		{"URL", "domain: https://example.org\n", ""},
		{"path", "domain: example.org/blog\n", ""},
		{"port", "domain: example.org:8080\n", ""},
		{"empty label", "domain: example..org\n", ""},
		{"invalid label", "domain: -example.org\n", ""},
		{"long label", "domain: " + strings.Repeat("a", 64) + ".org\n", ""},
		{"invalid YAML", "domain: [\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			put(t, root, "site.yaml", tc.config)
			got, err := siteDomain(root)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("invalid config accepted: %q", got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("domain = %q, error = %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSiteConfigBuildAndReload(t *testing.T) {
	root := fixture(t)
	stamp, err := fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "site.yaml", "domain: new-blog.example.org\n")
	next, err := fingerprint(root)
	if err != nil || next == stamp {
		t.Fatalf("config change was not detected: %v", err)
	}
	files, err := build(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files["/"]), `<link rel="canonical" href="https://new-blog.example.org/">`) ||
		!strings.Contains(string(files["/sitemap.xml"]), "https://new-blog.example.org/") {
		t.Fatal("configured domain missing from canonical link or sitemap")
	}
	if err := os.Remove(filepath.Join(root, "site.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := build(root); err == nil || !strings.Contains(err.Error(), "site.yaml") {
		t.Fatalf("missing config accepted: %v", err)
	}
}
