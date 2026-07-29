package rulesrc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// This file is a deliberately small JSON navigation helper (~100 lines, stdlib
// only). It supports the subset of "dot path" syntax needed by fingerprints:
//
//	[]            iterate the top-level array
//	.assets[]     iterate the "assets" array inside the current object
//	versions[]    iterate the "versions" array (no leading dot allowed by convention)
//	.tag_name     read the "tag_name" field of the current object
//	tag_name      read the "tag_name" field (leading dot optional)
//	.size         read a number field (returned as string via asString)
//
// It deliberately does NOT implement a full JSONPath spec. That minimalism is
// itself a validation finding: see REPORT.md.

// jsonNode is the working type. Unmarshalling into any leaves us with
// map[string]any / []any / string / float64 / bool / nil.
type jsonNode = any

// jsonArray splits a path like ".assets[]" or "[]" into (field, iterate).
// "[]"            -> ("", true)
// ".assets[]"     -> ("assets", true)
// "versions[]"    -> ("versions", true)
// ".tag_name"     -> ("tag_name", false)
// "tag_name"      -> ("tag_name", false)
func parseArrayPath(path string) (field string, iterate bool) {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, ".")
	if strings.HasSuffix(p, "[]") {
		return strings.TrimSuffix(p, "[]"), true
	}
	return p, false
}

// iterArray iterates a JSON array located at the given path within node,
// calling fn for each element. path may be "[]" (root array) or ".field[]".
func iterArray(node jsonNode, path string, fn func(jsonNode) error) error {
	field, iterate := parseArrayPath(path)
	if !iterate {
		return fmt.Errorf("path %q is not an array path (must end with [])", path)
	}
	if field != "" {
		m, ok := node.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object to access .%s, got %T", field, node)
		}
		node = m[field]
	}
	arr, ok := node.([]any)
	if !ok {
		return fmt.Errorf("expected array at %q, got %T", path, node)
	}
	for _, el := range arr {
		if err := fn(el); err != nil {
			return err
		}
	}
	return nil
}

// getPath reads a scalar field at a (possibly dotted) path within an object.
// Returns the raw value and a convenience string form via asString.
func getPath(node jsonNode, path string) (jsonNode, bool) {
	p := strings.TrimPrefix(strings.TrimSpace(path), ".")
	if p == "" {
		return node, true
	}
	cur := node
	for _, seg := range strings.Split(p, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// asString renders any JSON scalar to a string for ReleaseAsset fields.
func asString(v jsonNode) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		// JSON numbers unmarshal to float64; render integers without a decimal.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// asInt renders a JSON number to int64 (size fields).
func asInt(v jsonNode) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	}
	return 0
}

// truthy reports whether a boolean field at path is true (for skip filters).
func truthy(node jsonNode, path string) bool {
	v, ok := getPath(node, path)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}
