package ndctranslator

import (
	"fmt"
	"strings"
)

// step is one element of a path: a name in a namespace, or any element ("*").
type step struct {
	space, local string
	any          bool
}

func (s step) matches(name qname) bool {
	return s.any || (s.local == string(name.local) && s.space == string(name.space))
}

// path is a simple path written in a mapping file: element steps separated by "/", each as
// "prefix:name", "name" (no namespace) or "*", optionally ending in "@attribute". The path "."
// is the element itself. Without an attribute a path reads the text of the element it reaches.
type path struct {
	steps []step
	attr  string
}

// parsePath reads a path, turning the prefixes into namespaces with the given map.
func parsePath(src string, namespaces map[string]string) (path, error) {
	if src == "." {
		return path{}, nil
	}
	var p path
	parts := strings.Split(src, "/")
	for i, part := range parts {
		switch {
		case part == "" || part == ".":
			return path{}, fmt.Errorf("path %q has an empty or misplaced segment", src)
		case strings.HasPrefix(part, "@"):
			if i != len(parts)-1 || len(part) == 1 {
				return path{}, fmt.Errorf("path %q: an attribute must be named and come last", src)
			}
			if strings.Contains(part, ":") {
				return path{}, fmt.Errorf("path %q: attributes match by local name, so they take no prefix", src)
			}
			p.attr = part[1:]
		case part == "*":
			p.steps = append(p.steps, step{any: true})
		default:
			s, err := parseStep(part, namespaces)
			if err != nil {
				return path{}, fmt.Errorf("path %q: %w", src, err)
			}
			p.steps = append(p.steps, s)
		}
	}
	return p, nil
}

func parseStep(part string, namespaces map[string]string) (step, error) {
	prefix, local, found := strings.Cut(part, ":")
	if !found {
		return step{local: part}, nil
	}
	if strings.Contains(local, ":") {
		return step{}, fmt.Errorf("%q has more than one colon", part)
	}
	space, ok := namespaces[prefix]
	if !ok {
		return step{}, fmt.Errorf("unknown namespace prefix %q", prefix)
	}
	return step{space: space, local: local}, nil
}

// reaches reports whether the open elements, from the start of the path, are exactly its steps.
func (p path) reaches(open []qname) bool {
	if len(open) != len(p.steps) {
		return false
	}
	for i, s := range p.steps {
		if !s.matches(open[i]) {
			return false
		}
	}
	return true
}
