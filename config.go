package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var domainLabel = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func siteDomain(root string) (string, error) {
	file := filepath.Join(root, "site.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read site config: %w", err)
	}
	var config struct {
		Domain string `yaml:"domain"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parse %s: %w", file, err)
	}
	if len(config.Domain) == 0 || len(config.Domain) > 253 {
		return "", fmt.Errorf("%s: invalid domain %q", file, config.Domain)
	}
	for _, label := range strings.Split(config.Domain, ".") {
		if !domainLabel.MatchString(label) {
			return "", fmt.Errorf("%s: invalid domain %q", file, config.Domain)
		}
	}
	return config.Domain, nil
}
