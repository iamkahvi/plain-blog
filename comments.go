package main

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// stripComments removes Obsidian comments and preserves Markdown code examples.
func stripComments(source []byte) ([]byte, error) {
	marker := []byte("%%")
	for bytes.Contains(source, marker) {
		// Reparse after each comment because its contents can alter Markdown syntax.
		literal := make([]bool, len(source))
		doc := markdown.Parser().Parse(text.NewReader(source))
		mark := func(segment text.Segment) {
			for i := segment.Start; i < segment.Stop; i++ {
				literal[i] = true
			}
		}
		_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch n := node.(type) {
			case *ast.CodeSpan:
				for child := n.FirstChild(); child != nil; child = child.NextSibling() {
					if t, ok := child.(*ast.Text); ok {
						mark(t.Segment)
					}
				}
			case *ast.CodeBlock, *ast.FencedCodeBlock:
				for i := 0; i < node.Lines().Len(); i++ {
					mark(node.Lines().At(i))
				}
				if fenced, ok := node.(*ast.FencedCodeBlock); ok && fenced.Info != nil {
					mark(fenced.Info.Segment)
				}
			}
			return ast.WalkContinue, nil
		})

		start := -1
		for offset := 0; offset < len(source)-1; {
			i := bytes.Index(source[offset:], marker)
			if i < 0 {
				break
			}
			i += offset
			if !literal[i] && !literal[i+1] {
				start = i
				break
			}
			offset = i + 2
		}
		if start < 0 {
			return source, nil
		}
		end := bytes.Index(source[start+2:], marker)
		if end < 0 {
			return nil, fmt.Errorf("unclosed %%%% comment at body line %d", bytes.Count(source[:start], []byte("\n"))+1)
		}
		end += start + 4

		// Keep line breaks so comments do not join unrelated Markdown blocks.
		clean := make([]byte, 0, len(source)-(end-start))
		clean = append(clean, source[:start]...)
		for _, b := range source[start:end] {
			if b == '\r' || b == '\n' {
				clean = append(clean, b)
			}
		}
		source = append(clean, source[end:]...)
	}
	return source, nil
}
