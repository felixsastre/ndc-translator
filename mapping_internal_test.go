package ndctranslator

import (
	"strings"
	"testing"
)

const goodRecord = `
record "r" {
  path  = "R"
  label = "id"
  capture "id"  { path = "@id" }
  capture "amt" {
    path   = "P/*"
    accept = ["Simple"]
  }
  output "total" "sum" { from = "amt" }
}`

func parse(body string) (*Mapping, error) {
	return parseMapping([]byte("version = \"t\"\nnamespaces = { n = \"urn:n\" }\n"+body), "t.hcl")
}

func TestParseMappingAcceptsAGoodRecord(t *testing.T) {
	if _, err := parse(goodRecord); err != nil {
		t.Fatal(err)
	}
}

func TestParseMappingRejects(t *testing.T) {
	tests := map[string]string{
		"no records":                         ``,
		"unknown reduction":                  strings.Replace(goodRecord, `"sum"`, `"median"`, 1),
		"output from an unknown capture":     strings.Replace(goodRecord, `from = "amt"`, `from = "nope"`, 1),
		"label that is not a capture":        strings.Replace(goodRecord, `label = "id"`, `label = "nope"`, 1),
		"unknown namespace prefix":           strings.Replace(goodRecord, `path  = "R"`, `path  = "x:R"`, 1),
		"record path ending in an attribute": strings.Replace(goodRecord, `path  = "R"`, `path  = "R/@a"`, 1),
		"record path with no elements":       strings.Replace(goodRecord, `path  = "R"`, `path  = "."`, 1),
		"duplicate capture":                  strings.Replace(goodRecord, `capture "amt"`, `capture "id"`, 1),
		"duplicate output": strings.Replace(goodRecord, `output "total" "sum" { from = "amt" }`,
			"output \"total\" \"sum\" { from = \"amt\" }\n  output \"total\" \"first\" { from = \"id\" }", 1),
		"duplicate record": goodRecord + goodRecord,
	}
	for name, body := range tests {
		if _, err := parse(body); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
