package ndctranslator

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// The tokenizer reads XML that is already in memory. The names, attribute values and text it hands
// out are slices of the input, so reading allocates almost nothing. It supports what NDC messages
// use: elements, attributes with either kind of quote, namespaces, text with the predefined and
// numeric entities, CDATA sections, comments, processing instructions and a DOCTYPE or other declaration
// (which it skips without reading any entity declared in it). It requires a root element but not
// all of XML's well-formedness: it accepts several roots, repeated attributes and a raw < in an
// attribute value. It does not check that characters are valid XML
// characters, and it reads every document as UTF-8, failing if the declaration says otherwise.
// The tests hold it to the same results as encoding/xml.

const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

// qname is an element name: its namespace URI and its local part.
type qname struct{ space, local []byte }

// attr is an attribute as written: its name with any prefix, and its value with entities not decoded.
type attr struct{ name, value []byte }

// local is the name without its prefix.
func (a attr) local() []byte {
	_, local := splitName(a.name)
	return local
}

// splitName separates the prefix of a name. A colon only counts as a separator when it is neither
// the first nor the last byte, as in encoding/xml.
func splitName(raw []byte) (prefix, local []byte) {
	if i := bytes.IndexByte(raw, ':'); i >= 1 && i <= len(raw)-2 {
		return raw[:i], raw[i+1:]
	}
	return nil, raw
}

type tokenKind uint8

const (
	tokenStart tokenKind = iota + 1
	tokenEnd
	tokenText
	tokenEOF
)

// token is valid until the next call to next. For a start, name and attrs are set; for an end,
// name; for text, text, and entities says whether it may hold entities to resolve (false inside
// CDATA). Line breaks are still to be normalized in both cases, with appendText.
type token struct {
	kind     tokenKind
	name     qname
	attrs    []attr
	text     []byte
	entities bool
}

type nsDecl struct{ prefix, uri []byte }

type tokenizer struct {
	data        []byte
	pos         int
	names       [][]byte // the names of the open elements as written, to check the closing tags
	marks       []int    // len(scopes) before each open element declared its namespaces
	scopes      []nsDecl
	attrs       []attr // reused by every start tag
	selfClosing bool   // the element just opened is closed by the same tag
}

func newTokenizer(data []byte) *tokenizer {
	return &tokenizer{data: bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))}
}

func (t *tokenizer) errorf(format string, args ...any) error {
	return fmt.Errorf("xml: %s (byte %d)", fmt.Sprintf(format, args...), t.pos)
}

func (t *tokenizer) next() (token, error) {
	if t.selfClosing {
		t.selfClosing = false
		return t.closeTop(), nil
	}
	for {
		if t.pos >= len(t.data) {
			if n := len(t.names); n > 0 {
				return token{}, t.errorf("unexpected end of input, <%s> is not closed", t.names[n-1])
			}
			return token{kind: tokenEOF}, nil
		}
		if t.data[t.pos] != '<' {
			end := len(t.data)
			if i := bytes.IndexByte(t.data[t.pos:], '<'); i >= 0 {
				end = t.pos + i
			}
			text := t.data[t.pos:end]
			t.pos = end
			return token{kind: tokenText, text: text, entities: true}, nil
		}
		rest := t.data[t.pos:]
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			i := bytes.Index(rest[4:], []byte("-->"))
			if i < 0 {
				return token{}, t.errorf("comment is not closed")
			}
			t.pos += 4 + i + 3
		case bytes.HasPrefix(rest, []byte("<![CDATA[")):
			i := bytes.Index(rest[9:], []byte("]]>"))
			if i < 0 {
				return token{}, t.errorf("CDATA section is not closed")
			}
			t.pos += 9 + i + 3
			return token{kind: tokenText, text: rest[9 : 9+i]}, nil
		case bytes.HasPrefix(rest, []byte("<?")):
			i := bytes.Index(rest[2:], []byte("?>"))
			if i < 0 {
				return token{}, t.errorf("processing instruction is not closed")
			}
			if err := t.checkDeclaration(rest[2 : 2+i]); err != nil {
				return token{}, err
			}
			t.pos += 2 + i + 2
		case bytes.HasPrefix(rest, []byte("<!")):
			if err := t.skipDirective(); err != nil {
				return token{}, err
			}
		case bytes.HasPrefix(rest, []byte("</")):
			return t.endTag()
		default:
			return t.startTag()
		}
	}
}

// checkDeclaration fails if an XML declaration names an encoding other than UTF-8.
func (t *tokenizer) checkDeclaration(pi []byte) error {
	if !bytes.HasPrefix(pi, []byte("xml")) || len(pi) < 4 || !isSpace(pi[3]) {
		return nil
	}
	i := indexAttribute(pi, "encoding")
	if i < 0 {
		return nil
	}
	rest := bytes.TrimLeft(pi[i+8:], " \t\r\n")
	if len(rest) == 0 || rest[0] != '=' {
		return nil
	}
	rest = bytes.TrimLeft(rest[1:], " \t\r\n")
	if len(rest) < 2 || (rest[0] != '"' && rest[0] != '\'') {
		return nil
	}
	end := bytes.IndexByte(rest[1:], rest[0])
	if end < 0 {
		return nil
	}
	if enc := rest[1 : 1+end]; len(enc) > 0 && !bytes.EqualFold(enc, []byte("utf-8")) {
		return t.errorf("encoding %q is not supported, only UTF-8", enc)
	}
	return nil
}

// indexAttribute finds a pseudo-attribute of a declaration by its whole name, which has to follow a space.
func indexAttribute(decl []byte, name string) int {
	for from := 0; ; {
		i := bytes.Index(decl[from:], []byte(name))
		if i < 0 {
			return -1
		}
		i += from
		if i > 0 && isSpace(decl[i-1]) {
			return i
		}
		from = i + len(name)
	}
}

// skipDirective moves past a <!DOCTYPE ...> or any other <!...> declaration. Nothing declared in it
// is read. It ends at the first > outside quotes and comments that is not closing a nested <,
// the same rule encoding/xml follows, which ignores the first byte after "<!".
func (t *tokenizer) skipDirective() error {
	rest := t.data[t.pos:]
	depth := 0
	var quote byte
	for i := 3; i < len(rest); i++ {
		c := rest[i]
		if quote == 0 && c == '>' && depth == 0 {
			t.pos += i + 1
			return nil
		}
		switch {
		case c == quote:
			quote = 0
		case quote != 0:
		case c == '\'' || c == '"':
			quote = c
		case c == '>':
			depth--
		case c == '<':
			if !bytes.HasPrefix(rest[i:], []byte("<!--")) {
				depth++
				continue
			}
			j := bytes.Index(rest[i+4:], []byte("-->"))
			if j < 0 {
				return t.errorf("comment is not closed")
			}
			i += 4 + j + 2
		}
	}
	return t.errorf("declaration is not closed")
}

func (t *tokenizer) startTag() (token, error) {
	data := t.data
	i := t.pos + 1
	nameStart := i
	for i < len(data) && !isSpace(data[i]) && data[i] != '>' && data[i] != '/' {
		i++
	}
	if i == nameStart {
		return token{}, t.errorf("expected an element name after <")
	}
	rawName := data[nameStart:i]

	t.attrs = t.attrs[:0]
	for {
		for i < len(data) && isSpace(data[i]) {
			i++
		}
		if i >= len(data) {
			return token{}, t.errorf("unexpected end of input inside <%s>", rawName)
		}
		if data[i] == '>' {
			i++
			break
		}
		if data[i] == '/' {
			if i+1 < len(data) && data[i+1] == '>' {
				t.selfClosing = true
				i += 2
				break
			}
			return token{}, t.errorf("expected > after / in <%s>", rawName)
		}
		start := i
		for i < len(data) && data[i] != '=' && !isSpace(data[i]) && data[i] != '>' && data[i] != '/' {
			i++
		}
		if i == start {
			return token{}, t.errorf("expected an attribute name in <%s>", rawName)
		}
		name := data[start:i]
		for i < len(data) && isSpace(data[i]) {
			i++
		}
		if i >= len(data) || data[i] != '=' {
			return token{}, t.errorf("attribute %s of <%s> has no value", name, rawName)
		}
		i++
		for i < len(data) && isSpace(data[i]) {
			i++
		}
		if i >= len(data) || (data[i] != '"' && data[i] != '\'') {
			return token{}, t.errorf("value of attribute %s of <%s> is not quoted", name, rawName)
		}
		quote := data[i]
		i++
		end := bytes.IndexByte(data[i:], quote)
		if end < 0 {
			return token{}, t.errorf("value of attribute %s of <%s> is not closed", name, rawName)
		}
		t.attrs = append(t.attrs, attr{name: name, value: data[i : i+end]})
		i += end + 1
	}
	t.pos = i

	mark := len(t.scopes)
	for _, a := range t.attrs {
		prefix, local := splitName(a.name)
		var decl nsDecl
		var err error
		switch {
		case string(a.name) == "xmlns":
			decl, err = t.declaration(nil, a.value)
		case string(prefix) == "xmlns":
			decl, err = t.declaration(local, a.value)
		default:
			continue
		}
		if err != nil {
			return token{}, err
		}
		t.scopes = append(t.scopes, decl)
	}
	t.names = append(t.names, rawName)
	t.marks = append(t.marks, mark)
	return token{kind: tokenStart, name: t.resolve(rawName), attrs: t.attrs}, nil
}

// declaration reads a namespace declaration; the URI is decoded only if it holds entities.
func (t *tokenizer) declaration(prefix, uri []byte) (nsDecl, error) {
	if bytes.IndexByte(uri, '&') >= 0 {
		decoded, err := appendUnescaped(nil, uri)
		if err != nil {
			return nsDecl{}, t.errorf("%v", err)
		}
		uri = decoded
	}
	return nsDecl{prefix: prefix, uri: uri}, nil
}

func (t *tokenizer) endTag() (token, error) {
	data := t.data
	i := t.pos + 2
	nameStart := i
	for i < len(data) && !isSpace(data[i]) && data[i] != '>' {
		i++
	}
	rawName := data[nameStart:i]
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	if i >= len(data) || data[i] != '>' {
		return token{}, t.errorf("expected > to end </%s>", rawName)
	}
	t.pos = i + 1
	n := len(t.names)
	if n == 0 {
		return token{}, t.errorf("</%s> closes an element that is not open", rawName)
	}
	if !bytes.Equal(t.names[n-1], rawName) {
		return token{}, t.errorf("<%s> is closed by </%s>", t.names[n-1], rawName)
	}
	return t.closeTop(), nil
}

// closeTop ends the innermost open element and forgets the namespaces it declared.
func (t *tokenizer) closeTop() token {
	n := len(t.names)
	name := t.resolve(t.names[n-1])
	t.scopes = t.scopes[:t.marks[n-1]]
	t.names = t.names[:n-1]
	t.marks = t.marks[:n-1]
	return token{kind: tokenEnd, name: name}
}

// resolve gives the namespace of an element name as written, in the scope of the open elements.
// An unknown prefix is kept as the namespace, as encoding/xml does.
func (t *tokenizer) resolve(raw []byte) qname {
	prefix, local := splitName(raw)
	hasPrefix := prefix != nil
	if hasPrefix && string(prefix) == "xml" {
		return qname{space: []byte(xmlNamespace), local: local}
	}
	for j := len(t.scopes) - 1; j >= 0; j-- {
		if bytes.Equal(t.scopes[j].prefix, prefix) {
			return qname{space: t.scopes[j].uri, local: local}
		}
	}
	if hasPrefix {
		return qname{space: prefix, local: local}
	}
	return qname{local: local}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

var errEntity = errors.New("invalid character reference or entity")

// appendUnescaped adds an attribute value to dst with entities and line breaks resolved.
func appendUnescaped(dst, s []byte) ([]byte, error) { return appendText(dst, s, true) }

// appendText adds s to dst the way encoding/xml reads text: \r\n and \r become \n and, if entities
// is true, the five predefined entities and numeric references are resolved.
func appendText(dst, s []byte, entities bool) ([]byte, error) {
	for len(s) > 0 {
		i := firstEscape(s, entities)
		if i < 0 {
			return append(dst, s...), nil
		}
		dst = append(dst, s[:i]...)
		if s[i] == '\r' {
			dst = append(dst, '\n')
			s = s[i+1:]
			if len(s) > 0 && s[0] == '\n' {
				s = s[1:]
			}
			continue
		}
		s = s[i:]
		end := bytes.IndexByte(s, ';')
		if end < 0 {
			return dst, errEntity
		}
		ref := s[1:end]
		switch string(ref) {
		case "amp":
			dst = append(dst, '&')
		case "lt":
			dst = append(dst, '<')
		case "gt":
			dst = append(dst, '>')
		case "quot":
			dst = append(dst, '"')
		case "apos":
			dst = append(dst, '\'')
		default:
			r, ok := characterReference(ref)
			if !ok {
				return dst, errEntity
			}
			dst = utf8.AppendRune(dst, r)
		}
		s = s[end+1:]
	}
	return dst, nil
}

func firstEscape(s []byte, entities bool) int {
	if bytes.IndexByte(s, '\r') < 0 && (!entities || bytes.IndexByte(s, '&') < 0) {
		return -1
	}
	for i, c := range s {
		if c == '\r' || (entities && c == '&') {
			return i
		}
	}
	return -1
}

// characterReference reads the inside of &#65; or &#x41; and checks the result is an XML character.
func characterReference(ref []byte) (rune, bool) {
	if len(ref) < 2 || ref[0] != '#' {
		return 0, false
	}
	digits, base := string(ref[1:]), 10
	if ref[1] == 'x' {
		digits, base = string(ref[2:]), 16
	}
	n, err := strconv.ParseUint(digits, base, 32)
	if err != nil {
		return 0, false
	}
	r := rune(n)
	valid := r == 0x9 || r == 0xA || r == 0xD || (r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) || (r >= 0x10000 && r <= 0x10FFFF)
	return r, valid
}
