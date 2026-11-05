package ndctranslator

import (
	"embed"
	"fmt"
	"slices"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

//go:embed mappings/*.hcl
var mappingFiles embed.FS

// Mapping holds the records of one NDC version: where each one is and what to take from it.
type Mapping struct {
	records map[string]*recordSpec
}

type recordSpec struct {
	name     string
	path     path
	label    int // the capture whose first value names the record in errors, or -1
	captures []captureSpec
	outputs  []outputSpec
}

type captureSpec struct {
	name     string
	path     path
	accept   []string
	required bool // the record fails if the capture finds no value
}

type outputSpec struct {
	name       string
	from       int // index of the capture it reduces
	reduceName string
	reduce     reduction
}

// mappingFile is the shape of a mappings/<version>.hcl file.
type mappingFile struct {
	Version    string            `hcl:"version"`
	Namespaces map[string]string `hcl:"namespaces"`
	Records    []struct {
		Name     string `hcl:"name,label"`
		Path     string `hcl:"path"`
		Label    string `hcl:"label,optional"`
		Captures []struct {
			Name     string   `hcl:"name,label"`
			Path     string   `hcl:"path"`
			Accept   []string `hcl:"accept,optional"`
			Required bool     `hcl:"required,optional"`
		} `hcl:"capture,block"`
		Outputs []struct {
			Name   string `hcl:"name,label"`
			Reduce string `hcl:"reduce,label"`
			From   string `hcl:"from"`
		} `hcl:"output,block"`
	} `hcl:"record,block"`
}

// LoadMapping reads the HCL file of a version (for example "17.2").
func LoadMapping(version string) (*Mapping, error) {
	src, err := mappingFiles.ReadFile("mappings/" + version + ".hcl")
	if err != nil {
		return nil, fmt.Errorf("no mapping for NDC version %s: %w", version, err)
	}
	m, err := parseMapping(src, version+".hcl")
	if err != nil {
		return nil, fmt.Errorf("mapping %s: %w", version, err)
	}
	return m, nil
}

func parseMapping(src []byte, filename string) (*Mapping, error) {
	file, diags := hclparse.NewParser().ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, diags
	}
	var cfg mappingFile
	if diags := gohcl.DecodeBody(file.Body, nil, &cfg); diags.HasErrors() {
		return nil, diags
	}
	if len(cfg.Records) == 0 {
		return nil, fmt.Errorf("the file defines no record")
	}
	m := &Mapping{records: map[string]*recordSpec{}}
	for _, rf := range cfg.Records {
		if _, duplicate := m.records[rf.Name]; duplicate {
			return nil, fmt.Errorf("record %q is defined twice", rf.Name)
		}
		spec, err := compileRecord(rf.Name, rf.Path, cfg.Namespaces)
		if err != nil {
			return nil, fmt.Errorf("record %q: %w", rf.Name, err)
		}
		index := map[string]int{}
		for _, cf := range rf.Captures {
			if _, duplicate := index[cf.Name]; duplicate {
				return nil, fmt.Errorf("record %q: capture %q is defined twice", rf.Name, cf.Name)
			}
			p, err := parsePath(cf.Path, cfg.Namespaces)
			if err != nil {
				return nil, fmt.Errorf("record %q, capture %q: %w", rf.Name, cf.Name, err)
			}
			index[cf.Name] = len(spec.captures)
			spec.captures = append(spec.captures, captureSpec{name: cf.Name, path: p, accept: cf.Accept, required: cf.Required})
		}
		if rf.Label != "" {
			i, ok := index[rf.Label]
			if !ok {
				return nil, fmt.Errorf("record %q: label %q is not a capture", rf.Name, rf.Label)
			}
			spec.label = i
		}
		outputs := map[string]bool{}
		for _, of := range rf.Outputs {
			if outputs[of.Name] {
				return nil, fmt.Errorf("record %q: output %q is defined twice", rf.Name, of.Name)
			}
			outputs[of.Name] = true
			from, ok := index[of.From]
			if !ok {
				return nil, fmt.Errorf("record %q, output %q: %q is not a capture", rf.Name, of.Name, of.From)
			}
			reduce, ok := reductions[of.Reduce]
			if !ok {
				return nil, fmt.Errorf("record %q, output %q: unknown reduction %q", rf.Name, of.Name, of.Reduce)
			}
			spec.outputs = append(spec.outputs, outputSpec{name: of.Name, from: from, reduceName: of.Reduce, reduce: reduce})
		}
		m.records[rf.Name] = spec
	}
	return m, nil
}

func compileRecord(name, src string, namespaces map[string]string) (*recordSpec, error) {
	p, err := parsePath(src, namespaces)
	if err != nil {
		return nil, err
	}
	if len(p.steps) == 0 || p.attr != "" {
		return nil, fmt.Errorf("path must be elements from the root")
	}
	return &recordSpec{name: name, path: p, label: -1}, nil
}

// expect checks that a record has the outputs a typed reader relies on, each with its reduction,
// and that the captures it names are required, so a missing value is an error and not a zero.
func (m *Mapping) expect(record string, outputs map[string]string, required ...string) error {
	spec, ok := m.records[record]
	if !ok {
		return fmt.Errorf("the mapping has no record %q", record)
	}
	for name, reduce := range outputs {
		if !slices.ContainsFunc(spec.outputs, func(o outputSpec) bool { return o.name == name && o.reduceName == reduce }) {
			return fmt.Errorf("record %q needs an output %q reduced with %q", record, name, reduce)
		}
	}
	for _, name := range required {
		if !slices.ContainsFunc(spec.captures, func(c captureSpec) bool { return c.name == name && c.required }) {
			return fmt.Errorf("record %q needs the capture %q marked required", record, name)
		}
	}
	return nil
}
