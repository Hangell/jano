// Package routepattern validates and describes Jano route patterns.
// It is internal so pattern metadata can evolve without becoming public API.
package routepattern

import (
	"fmt"
	"strings"
)

// Segment is a literal, single-segment parameter or terminal catch-all.
type Segment struct {
	Value               string
	Parameter, CatchAll bool
}

// ValidMethod reports whether a method is a non-empty HTTP token.
func ValidMethod(method string) bool {
	if method == "" {
		return false
	}
	for _, c := range method {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

// Parse validates a route and returns its segments and structural signature.
// Parameters occupy whole segments; catch-alls must be terminal.
func Parse(path string) ([]Segment, string, error) {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
		return nil, "", fmt.Errorf("jano: invalid route path %q", path)
	}
	parts := strings.Split(path, "/")
	segments := make([]Segment, len(parts))
	var signature strings.Builder
	names := make(map[string]bool)
	for i, part := range parts {
		s := Segment{Value: part}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			s.Value = part[1 : len(part)-1]
			s.Parameter = true
			if strings.HasSuffix(s.Value, "...") {
				s.CatchAll = true
				s.Value = strings.TrimSuffix(s.Value, "...")
				if i != len(parts)-1 {
					return nil, "", fmt.Errorf("jano: catch-all must be the final segment in %q", path)
				}
			}
			if s.Value == "" || strings.ContainsAny(s.Value, "{}") || names[s.Value] {
				return nil, "", fmt.Errorf("jano: invalid or repeated parameter %q in %q", s.Value, path)
			}
			names[s.Value] = true
		} else if strings.ContainsAny(part, "{}") {
			return nil, "", fmt.Errorf("jano: parameters must occupy an entire segment in %q", path)
		}
		segments[i] = s
		if s.CatchAll {
			signature.WriteString("C;")
		} else if s.Parameter {
			signature.WriteString("P;")
		} else {
			fmt.Fprintf(&signature, "S%d:%s;", len(part), part)
		}
	}
	return segments, signature.String(), nil
}
