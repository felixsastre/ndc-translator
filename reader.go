package ndctranslator

import (
	"errors"
	"fmt"
	"strings"
)

// Record is what one record of a response gives: each output by name, with the type its
// reduction produces (string for first and same, decimal.Decimal for sum, []string for list).
type Record map[string]any

// ErrUnexpectedRoot is reported when the response is empty or its root element is not the one the
// record starts at, so an error page is never read as a response with no records.
var ErrUnexpectedRoot = errors.New("the response is not the expected kind of message")

// RecordError reports a record that could not be read. ID is the label of the record, if the
// mapping names one. Capture or Output names the part of the mapping that failed, when there is one.
type RecordError struct {
	Record, ID, Capture, Output string
	Err                         error
}

func (e *RecordError) Error() string {
	msg := fmt.Sprintf("%s %s", e.Record, e.ID)
	switch {
	case e.Capture != "":
		msg += ", capture " + e.Capture
	case e.Output != "":
		msg += ", output " + e.Output
	}
	return msg + ": " + e.Err.Error()
}

func (e *RecordError) Unwrap() error { return e.Err }

// Read returns every record of the given kind found in a response. It reads the XML once, token by
// token, and keeps only the values the mapping captures.
func Read(xmlData []byte, m *Mapping, record string) ([]Record, error) {
	spec, ok := m.records[record]
	if !ok {
		return nil, fmt.Errorf("the mapping has no record %q", record)
	}
	r := reader{spec: spec}
	tokens := newTokenizer(xmlData)
	for {
		tok, err := tokens.next()
		if err != nil {
			return nil, fmt.Errorf("reading the response: %w", err)
		}
		switch tok.kind {
		case tokenEOF:
			if !r.sawRoot {
				return nil, fmt.Errorf("%w: the response is empty", ErrUnexpectedRoot)
			}
			return r.records, nil
		case tokenStart:
			r.start(tok)
		case tokenText:
			r.text(tok)
		case tokenEnd:
			r.end()
		}
		if r.err != nil {
			return nil, r.err
		}
	}
}

// reader is the state of one pass over a response.
type reader struct {
	spec       *recordSpec
	sawRoot    bool
	open       []qname // the elements open now, from the root
	current    *recordState
	collectors []collector
	spare      [][]byte // buffers of finished collectors, to reuse
	records    []Record
	err        error
}

// recordState is the record being read.
type recordState struct {
	depth  int        // len(open) while the record element is open
	values [][]string // what each capture has found so far
	fail   error      // the first element a capture refused, reported when the record closes
}

// collector gathers the text directly inside one element, for one capture.
type collector struct {
	capture, depth int
	buf            []byte
}

func (r *reader) start(tok token) {
	r.open = append(r.open, tok.name)
	if len(r.open) == 1 {
		r.sawRoot = true
		if want := r.spec.path.steps[0]; !want.matches(tok.name) {
			r.err = fmt.Errorf("%w: found <%s>, want <%s>", ErrUnexpectedRoot, tok.name.local, want.local)
			return
		}
	}
	if r.current == nil {
		if r.spec.path.reaches(r.open) {
			r.current = &recordState{depth: len(r.open), values: make([][]string, len(r.spec.captures))}
			r.look(tok, nil)
		}
		return
	}
	r.look(tok, r.open[r.current.depth:])
}

// look checks an element inside the record (its path relative to the record) against every capture.
func (r *reader) look(tok token, inside []qname) {
	for i, c := range r.spec.captures {
		if !c.path.reaches(inside) {
			continue
		}
		if len(c.accept) > 0 && !accepts(c.accept, tok.name.local) {
			if r.current.fail == nil {
				r.current.fail = fmt.Errorf("%w: %s in capture %s", ErrNotAccepted, tok.name.local, c.name)
			}
			continue
		}
		if c.path.attr == "" {
			r.startText(i)
			continue
		}
		for _, a := range tok.attrs {
			if string(a.local()) != c.path.attr {
				continue
			}
			value, err := attributeValue(a.value)
			if err != nil {
				r.err = err
				return
			}
			r.current.values[i] = append(r.current.values[i], value)
			break
		}
	}
}

// hasValue reports whether a capture found a value that is not empty.
func hasValue(values []string) bool {
	for _, v := range values {
		if v != "" {
			return true
		}
	}
	return false
}

func accepts(names []string, local []byte) bool {
	for _, name := range names {
		if name == string(local) {
			return true
		}
	}
	return false
}

// attributeValue reads the value of an attribute, resolving entities only if it has any.
func attributeValue(raw []byte) (string, error) {
	if firstEscape(raw, true) < 0 {
		return string(raw), nil
	}
	decoded, err := appendUnescaped(nil, raw)
	return string(decoded), err
}

func (r *reader) startText(capture int) {
	var buf []byte
	if n := len(r.spare); n > 0 {
		buf, r.spare = r.spare[n-1][:0], r.spare[:n-1]
	}
	r.collectors = append(r.collectors, collector{capture: capture, depth: len(r.open), buf: buf})
}

func (r *reader) text(tok token) {
	for i := range r.collectors {
		c := &r.collectors[i]
		if c.depth != len(r.open) {
			continue
		}
		var err error
		if c.buf, err = appendText(c.buf, tok.text, tok.entities); err != nil {
			r.err = err
			return
		}
	}
}

func (r *reader) end() {
	depth := len(r.open)
	for n := len(r.collectors); n > 0 && r.collectors[n-1].depth == depth; n-- {
		c := r.collectors[n-1]
		r.current.values[c.capture] = append(r.current.values[c.capture], strings.TrimSpace(string(c.buf)))
		r.spare = append(r.spare, c.buf)
		r.collectors = r.collectors[:n-1]
	}
	if r.current != nil && r.current.depth == depth {
		r.closeRecord()
	}
	r.open = r.open[:depth-1]
}

func (r *reader) closeRecord() {
	state := r.current
	r.current = nil
	id := ""
	if r.spec.label >= 0 && len(state.values[r.spec.label]) > 0 {
		id = state.values[r.spec.label][0]
	}
	if state.fail != nil {
		r.err = &RecordError{Record: r.spec.name, ID: id, Err: state.fail}
		return
	}
	for i, c := range r.spec.captures {
		if c.required && !hasValue(state.values[i]) {
			r.err = &RecordError{Record: r.spec.name, ID: id, Capture: c.name, Err: ErrNoValue}
			return
		}
	}
	out := make(Record, len(r.spec.outputs))
	for _, o := range r.spec.outputs {
		value, err := o.reduce(state.values[o.from])
		if err != nil {
			r.err = &RecordError{Record: r.spec.name, ID: id, Output: o.name, Err: err}
			return
		}
		out[o.name] = value
	}
	r.records = append(r.records, out)
}
