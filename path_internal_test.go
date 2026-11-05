package ndctranslator

import (
	"reflect"
	"testing"
)

var testNamespaces = map[string]string{"n": "urn:n", "c": "urn:c"}

func TestParsePath(t *testing.T) {
	tests := []struct {
		src  string
		want path
	}{
		{".", path{}},
		{"@Code", path{attr: "Code"}},
		{"n:Offer", path{steps: []step{{space: "urn:n", local: "Offer"}}}},
		{"n:A/c:B/@id", path{steps: []step{{space: "urn:n", local: "A"}, {space: "urn:c", local: "B"}}, attr: "id"}},
		{"n:A/*", path{steps: []step{{space: "urn:n", local: "A"}, {any: true}}}},
		{"Plain", path{steps: []step{{local: "Plain"}}}},
	}
	for _, tt := range tests {
		got, err := parsePath(tt.src, testNamespaces)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parsePath(%q) = %+v, %v; want %+v", tt.src, got, err, tt.want)
		}
	}
}

func TestParsePathRejects(t *testing.T) {
	for _, src := range []string{"", "a//b", "a/", "/a", "x:A", "@", "@a/b", "a/./b", "@p:a", "n:a:b"} {
		if _, err := parsePath(src, testNamespaces); err == nil {
			t.Errorf("parsePath(%q) succeeded, want an error", src)
		}
	}
}

func TestReaches(t *testing.T) {
	p, _ := parsePath("n:A/*/n:C", testNamespaces)
	name := func(space, local string) qname { return qname{space: []byte(space), local: []byte(local)} }
	yes := []qname{name("urn:n", "A"), name("urn:zzz", "anything"), name("urn:n", "C")}
	if !p.reaches(yes) {
		t.Error("want a match: a wildcard accepts any element")
	}
	for _, no := range [][]qname{
		{name("urn:n", "A"), name("", "x")},
		{name("urn:other", "A"), name("", "x"), name("urn:n", "C")},
		{name("urn:n", "A"), name("", "x"), name("urn:n", "C"), name("urn:n", "D")},
	} {
		if p.reaches(no) {
			t.Errorf("unexpected match for %v", no)
		}
	}
}
