package ndctranslator

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

const listMapping = `
record "r" {
  path = "R"
  capture "v" { path = "I/@v" }
  output "values" "list" { from = "v" }
}`

// Property: list gives back the captured values, in the order they appear in the XML.
func TestListKeepsTheOrderOfTheXML(t *testing.T) {
	m, err := parse(listMapping)
	if err != nil {
		t.Fatal(err)
	}
	prop := func(numbers []uint8) bool {
		want := make([]string, len(numbers))
		var xml strings.Builder
		xml.WriteString("<R>")
		for i, n := range numbers {
			want[i] = "v" + strconv.Itoa(int(n))
			fmt.Fprintf(&xml, `<I v="%s"/>`, want[i])
		}
		xml.WriteString("</R>")
		records, err := Read([]byte(xml.String()), m, "r")
		return err == nil && len(records) == 1 && reflect.DeepEqual(records[0]["values"], want)
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Error(err)
	}
}

func TestReadUnknownRecord(t *testing.T) {
	m, _ := parse(listMapping)
	if _, err := Read([]byte("<R/>"), m, "other"); err == nil {
		t.Error("want an error for a record the mapping does not define")
	}
}

func TestReadRejectsAnElementThatIsNotAccepted(t *testing.T) {
	m, _ := parse(goodRecord)
	_, err := Read([]byte(`<R id="X"><P><Other>1</Other></P></R>`), m, "r")
	var re *RecordError
	if !errors.As(err, &re) || re.ID != "X" {
		t.Errorf("got %v, want a record error labelled X", err)
	}
}

const requiredMapping = `
record "r" {
  path  = "R"
  label = "id"

  capture "id" { path = "@id" }
  capture "v" {
    path     = "I/@v"
    required = true
  }

  output "values" "list" { from = "v" }
}`

func TestRequiredCaptureThatFindsNothingFailsTheRecord(t *testing.T) {
	m, err := parse(requiredMapping)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Read([]byte(`<R id="X"><I/></R>`), m, "r")
	var re *RecordError
	if !errors.As(err, &re) || !errors.Is(err, ErrNoValue) || re.ID != "X" || re.Capture != "v" {
		t.Errorf("got %v, want a record error for X, capture v, wrapping ErrNoValue", err)
	}
}

func TestRequiredCaptureThatFindsAValueIsRead(t *testing.T) {
	m, _ := parse(requiredMapping)
	records, err := Read([]byte(`<R id="X"><I v="a"/></R>`), m, "r")
	if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0]["values"], []string{"a"}) {
		t.Errorf("got %v, %v; want one record with [a]", records, err)
	}
}

func TestACaptureThatIsNotRequiredMayFindNothing(t *testing.T) {
	m, _ := parse(goodRecord)
	records, err := Read([]byte(`<R id="X"/>`), m, "r")
	if err != nil || len(records) != 1 {
		t.Errorf("got %v, %v; want one record", records, err)
	}
}

func TestExpectChecksThatTheCapturesAreRequired(t *testing.T) {
	notRequired, _ := parse(goodRecord)
	if err := notRequired.expect("r", map[string]string{"total": "sum"}, "amt"); err == nil {
		t.Error("want an error when a capture the reader relies on is not required")
	}
	required, _ := parse(requiredMapping)
	if err := required.expect("r", map[string]string{"values": "list"}, "v"); err != nil {
		t.Error(err)
	}
}

func TestReadRejectsABodyWithoutTheExpectedRoot(t *testing.T) {
	m, _ := parse(listMapping)
	for name, body := range map[string]string{
		"empty body":        "",
		"only whitespace":   "  \n ",
		"another root":      "<html><body>503</body></html>",
		"another namespace": `<R xmlns="urn:other"/>`,
	} {
		if _, err := Read([]byte(body), m, "r"); !errors.Is(err, ErrUnexpectedRoot) {
			t.Errorf("%s: got %v, want ErrUnexpectedRoot", name, err)
		}
	}
}

func TestRequiredCaptureThatFindsOnlyEmptyValuesFailsTheRecord(t *testing.T) {
	m, _ := parse(requiredMapping)
	_, err := Read([]byte(`<R id="X"><I v=""/></R>`), m, "r")
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("got %v, want ErrNoValue", err)
	}
}
