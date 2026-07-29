package rulesrc

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// This file collects candidate items from an HTML listing page using the
// already-available golang.org/x/net/html dependency (no new dep added).
//
// The supported selector grammar is intentionally tiny:
//
//	"a"        all <a> elements
//	"a.class"  <a> elements having class "class"
//	"tag"      all <tag> elements
//
// This covers Apache/nginx autoindex pages and most static directory listings.
// More complex selectors are deliberately out of scope (see REPORT.md).

// htmlItem is one picked element: its text and a named attribute (usually href).
type htmlItem struct {
	text string
	attr string // attribute value named by attrName (defaults to "href")
}

// pickHTML reads an HTML document and returns one item per element matching
// selector, carrying the element's text and the attribute named attrName
// (empty attrName defaults to "href").
func pickHTML(r io.Reader, selector, attrName string) ([]htmlItem, error) {
	tag, class := parseSelector(selector)
	if attrName == "" {
		attrName = "href"
	}
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	var items []htmlItem
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag && hasClass(n, class) {
			items = append(items, htmlItem{
				text: textOf(n),
				attr: attrOf(n, attrName),
			})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return items, nil
}

// parseSelector splits "a.class" -> ("a", "class"); "a" -> ("a", "").
func parseSelector(sel string) (tag, class string) {
	sel = strings.TrimSpace(sel)
	if i := strings.IndexByte(sel, '.'); i >= 0 {
		return sel[:i], sel[i+1:]
	}
	return sel, ""
}

func hasClass(n *html.Node, class string) bool {
	if class == "" {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textOf returns the trimmed concatenated text of an element's descendants.
func textOf(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return strings.TrimSpace(b.String())
}
