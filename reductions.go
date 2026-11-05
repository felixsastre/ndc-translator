package ndctranslator

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

var (
	// ErrDifferentValues is what "same" reports when the captured values are not all equal.
	ErrDifferentValues = errors.New("the values are not all the same")
	// ErrNoValue is reported when a capture marked required finds no value in a record.
	ErrNoValue = errors.New("no value was found")
	// ErrNotAccepted is reported when a capture finds an element whose name it does not accept.
	ErrNotAccepted = errors.New("the element is not accepted")
)

// A reduction turns the values a capture found in one record into the single value of an output.
type reduction func(values []string) (any, error)

// reductions is the vocabulary a mapping file can name in "reduce".
var reductions = map[string]reduction{
	"first": first,
	"same":  same,
	"sum":   sum,
	"list":  list,
}

// first gives the first value, or "" when there is none.
func first(values []string) (any, error) {
	if len(values) == 0 {
		return "", nil
	}
	return values[0], nil
}

// same gives the value every one of them shares, or "" when there is none; it fails when they differ.
func same(values []string) (any, error) {
	if len(values) == 0 {
		return "", nil
	}
	for _, v := range values[1:] {
		if v != values[0] {
			return nil, ErrDifferentValues
		}
	}
	return values[0], nil
}

// sum adds the values as decimal amounts; with no values it gives zero.
func sum(values []string) (any, error) {
	total := decimal.Zero
	for _, v := range values {
		amount, err := decimal.NewFromString(v)
		if err != nil {
			return nil, fmt.Errorf("%q is not an amount: %w", v, err)
		}
		total = total.Add(amount)
	}
	return total, nil
}

// list gives a copy of the values, in the order they appear in the XML.
func list(values []string) (any, error) {
	return append([]string{}, values...), nil
}
