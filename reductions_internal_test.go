package ndctranslator

import (
	"errors"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
)

func equalValue(got, want any) bool {
	if g, ok := got.(decimal.Decimal); ok {
		w, isDecimal := want.(decimal.Decimal)
		return isDecimal && g.Equal(w)
	}
	return reflect.DeepEqual(got, want)
}

func TestReductions(t *testing.T) {
	tests := []struct {
		reduce  string
		in      []string
		want    any
		wantErr error
	}{
		{"first", []string{"b", "a"}, "b", nil},
		{"first", nil, "", nil},
		{"same", []string{"USD", "USD"}, "USD", nil},
		{"same", nil, "", nil},
		{"same", []string{"USD", "EUR"}, nil, ErrDifferentValues},
		{"sum", []string{"0.10", "0.20"}, decimal.RequireFromString("0.30"), nil},
		{"sum", []string{"1370.23", "1370.23"}, decimal.RequireFromString("2740.46"), nil},
		{"sum", []string{"12.50"}, decimal.RequireFromString("12.50"), nil},
		{"sum", nil, decimal.Zero, nil},
		{"list", []string{"SH2", "SH1"}, []string{"SH2", "SH1"}, nil},
		{"list", nil, []string{}, nil},
	}
	for _, tt := range tests {
		got, err := reductions[tt.reduce](tt.in)
		if !errors.Is(err, tt.wantErr) || (err == nil && !equalValue(got, tt.want)) {
			t.Errorf("%s(%v) = %v, %v; want %v, %v", tt.reduce, tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestSumRejectsWhatIsNotAnAmount(t *testing.T) {
	if _, err := reductions["sum"]([]string{"1.00", "abc"}); err == nil {
		t.Error("want an error for a value that is not an amount")
	}
}

func TestListDoesNotShareTheCapturedSlice(t *testing.T) {
	in := []string{"a", "b"}
	out, _ := reductions["list"](in)
	in[0] = "changed"
	if out.([]string)[0] != "a" {
		t.Error("the list must be a copy, the reader reuses what it captured")
	}
}
