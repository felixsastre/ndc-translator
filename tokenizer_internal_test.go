package ndctranslator

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stdTokens reads a document with encoding/xml, the reference, and lists its tokens in a
// plain form. Adjacent text is merged; comments, directives and processing instructions are dropped.
// A byte order mark at the start is not content, so it is removed first (encoding/xml returns it as text).
func stdTokens(data []byte) ([]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	var out []string
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, "T|"+text.String())
			text.Reset()
		}
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			flush()
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			flush()
			attrs := make([]string, len(t.Attr))
			for i, a := range t.Attr {
				attrs[i] = a.Name.Local + "=" + a.Value
			}
			out = append(out, "S|"+t.Name.Space+"|"+t.Name.Local+"|"+strings.Join(attrs, ","))
		case xml.EndElement:
			flush()
			out = append(out, "E|"+t.Name.Space+"|"+t.Name.Local)
		case xml.CharData:
			text.Write(t)
		}
	}
}

// ownTokens reads the same document with the tokenizer, in the same plain form.
func ownTokens(data []byte) ([]string, error) {
	var out []string
	var text []byte
	flush := func() {
		if len(text) > 0 {
			out = append(out, "T|"+string(text))
			text = text[:0]
		}
	}
	tk := newTokenizer(data)
	for {
		tok, err := tk.next()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case tokenEOF:
			flush()
			return out, nil
		case tokenStart:
			flush()
			attrs := make([]string, len(tok.attrs))
			for i, a := range tok.attrs {
				value, err := appendUnescaped(nil, a.value)
				if err != nil {
					return nil, err
				}
				attrs[i] = string(a.local()) + "=" + string(value)
			}
			out = append(out, "S|"+string(tok.name.space)+"|"+string(tok.name.local)+"|"+strings.Join(attrs, ","))
		case tokenEnd:
			flush()
			out = append(out, "E|"+string(tok.name.space)+"|"+string(tok.name.local))
		case tokenText:
			text, err = appendText(text, tok.text, tok.entities)
			if err != nil {
				return nil, err
			}
		}
	}
}

var wellFormed = []string{
	`<a/>`,
	`<a></a>`,
	`<a x="1" y='2'>text</a>`,
	`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a><b>1</b><b>2</b></a>`,
	"\xEF\xBB\xBF<a>bom</a>",
	`<a>fish &amp; chips &lt;3 &gt; &quot;q&quot; &apos;s&apos; &#65;&#x42;</a>`,
	`<a x="a &amp; b &#x41;"/>`,
	`<a>before<![CDATA[<raw> & text]]>after</a>`,
	"<a><![CDATA[x\r\ny]]></a>",
	"<a>t<![CDATA[line\rbreak]]>u</a>",
	`<a>x<!-- a comment -->y<?pi target?>z</a>`,
	`<!DOCTYPE a [<!ELEMENT a ANY>]><a>d</a>`,
	`<!DOCTYPE a><a/>`,
	"<a>\r\nline\r\nbreaks\rhere</a>",
	`<a x="1>2">gt in attribute</a>`,
	`<a  x = "1"   y='2' ></a >`,
	`<n:a xmlns:n="urn:n"><n:b/><c/></n:a>`,
	`<a xmlns="urn:d"><b/><c xmlns="urn:e"><d/></c><e/></a>`,
	`<a xmlns="urn:d"><b xmlns=""><c/></b></a>`,
	`<p:a xmlns:p="urn:1"><p:b xmlns:p="urn:2"><p:c/></p:b><p:d/></p:a>`,
	`<a><u:b/></a>`,
	`<a n:x="1" xmlns:n="urn:n"/>`,
	`<r><i v="1"/><i v="2"/><i v="3"/></r>`,
	`<a>é ü 日本語</a>`,
	"  \n<a/>\n  ",
}

func TestTokenizerMatchesEncodingXML(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.xml")
	inputs := append([]string{}, wellFormed...)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, string(data))
	}
	for _, in := range inputs {
		want, err := stdTokens([]byte(in))
		if err != nil {
			t.Fatalf("reference rejects %q: %v", in, err)
		}
		got, err := ownTokens([]byte(in))
		if err != nil {
			t.Errorf("tokenizer rejects %.80q: %v", in, err)
			continue
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("tokens differ for %.80q\n got: %q\nwant: %q", in, got, want)
		}
	}
}

func TestTokenizerRejectsWhatIsNotWellFormed(t *testing.T) {
	for _, in := range []string{
		`<a>`,
		`<a></b>`,
		`<a><b></a></b>`,
		`</a>`,
		`<a`,
		`<a b=1/>`,
		`<a b="1/>`,
		`<a b>`,
		`<a>&bogus;</a>`,
		`<a>&amp</a>`,
		`<a>&#xZZ;</a>`,
		`<a><!-- never closed </a>`,
		`<a><![CDATA[ never closed </a>`,
		`<? never closed`,
		`<?xml version="1.0" encoding="ISO-8859-1"?><a/>`,
		`<a><</a>`,
	} {
		if _, err := stdTokens([]byte(in)); err == nil {
			t.Fatalf("test input %q is accepted by the reference; remove it from this list", in)
		}
		if _, err := ownTokens([]byte(in)); err == nil {
			t.Errorf("tokenizer accepts %q, want an error", in)
		}
	}
}

// Whatever the reference accepts, the tokenizer reads the same way.
func FuzzTokenizerMatchesEncodingXML(f *testing.F) {
	for _, in := range wellFormed {
		f.Add([]byte(in))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		want, err := stdTokens(data)
		if err != nil {
			return
		}
		got, err := ownTokens(data)
		if err != nil {
			t.Fatalf("tokenizer rejects what the reference accepts: %v\ninput: %q", err, data)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("tokens differ\ninput: %q\n got: %q\nwant: %q", data, got, want)
		}
	})
}

func BenchmarkTokenizerOwn(b *testing.B) {
	data, _ := os.ReadFile("testdata/one_way_1pax.xml")
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		tk := newTokenizer(data)
		for {
			tok, err := tk.next()
			if err != nil || tok.kind == tokenEOF {
				break
			}
		}
	}
}

func BenchmarkTokenizerStd(b *testing.B) {
	data, _ := os.ReadFile("testdata/one_way_1pax.xml")
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			if _, err := d.Token(); err != nil {
				break
			}
		}
	}
}

// "encoding" has to be a pseudo-attribute of the declaration, not the end of another name.
func TestDeclarationEncodingIsAnAttributeNotASubstring(t *testing.T) {
	data := []byte(`<?xml version="1.0" myencoding="latin1"?><a/>`)
	if _, err := ownTokens(data); err != nil {
		t.Errorf("a pseudo-attribute that only ends in 'encoding' is not the encoding: %v", err)
	}
}
