package ndctranslator

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Order is the booking an airline answers with: its total, its items and what it covers.
type Order struct {
	ID, Owner, Currency string
	Total               decimal.Decimal
	BaseAmount, Taxes   decimal.Decimal // summed over the items
	Items               []string        // order item ids
	Passengers          []string        // passenger ids, in the order the airline lists them
	Segments            []string        // flight segment keys, in the order the airline lists them
}

// The outputs of the "order" record that ReadOrder relies on.
var orderOutputs = map[string]string{
	"id": "first", "owner": "first", "currency": "same", "total": "sum", "base": "sum", "taxes": "sum",
	"items": "list", "passengers": "list", "segments": "list",
}

// ReadOrder returns the order of an order response, using the "order" record of the mapping. It
// returns nil when the response carries no order, and an error if it carries more than one.
func ReadOrder(xmlData []byte, m *Mapping) (*Order, error) {
	if err := m.expect("order", orderOutputs, "total", "currency"); err != nil {
		return nil, err
	}
	records, err := Read(xmlData, m, "order")
	if err != nil {
		return nil, err
	}
	switch len(records) {
	case 0:
		return nil, nil
	case 1:
		rec := records[0]
		return &Order{
			ID:         rec["id"].(string),
			Owner:      rec["owner"].(string),
			Currency:   rec["currency"].(string),
			Total:      rec["total"].(decimal.Decimal),
			BaseAmount: rec["base"].(decimal.Decimal),
			Taxes:      rec["taxes"].(decimal.Decimal),
			Items:      rec["items"].([]string),
			Passengers: rec["passengers"].([]string),
			Segments:   rec["segments"].([]string),
		}, nil
	}
	return nil, fmt.Errorf("the response holds %d orders, want at most one", len(records))
}
