package ndctranslator

import (
	"errors"

	"github.com/shopspring/decimal"
)

// Offer is an airline offer with its total price.
type Offer struct {
	ID, Owner, Currency string
	Total               decimal.Decimal
}

// UnsupportedPriceFormError reports an offer whose price is not in a supported form.
type UnsupportedPriceFormError struct{ OfferID string }

func (e *UnsupportedPriceFormError) Error() string {
	return "the price form of " + e.OfferID + " is not supported"
}

// MixedCurrenciesError reports an offer whose items are priced in different currencies.
type MixedCurrenciesError struct{ OfferID string }

func (e *MixedCurrenciesError) Error() string { return e.OfferID + " mixes currencies" }

// MissingPriceError reports an offer that has no price.
type MissingPriceError struct{ OfferID string }

func (e *MissingPriceError) Error() string { return e.OfferID + " has no price" }

// amountCapture is the capture of the "offer" record that holds the price. ReadOffers requires it
// and reports an offer that finds no value in it as having no price.
const amountCapture = "amount"

// The outputs of the "offer" record that ReadOffers relies on.
var offerOutputs = map[string]string{"id": "first", "owner": "first", "total": "sum", "currency": "same"}

// ReadOffers returns the offers of an AirShopping response, using the "offer" record of the mapping.
func ReadOffers(xmlData []byte, m *Mapping) ([]Offer, error) {
	if err := m.expect("offer", offerOutputs, amountCapture, "currency"); err != nil {
		return nil, err
	}
	records, err := Read(xmlData, m, "offer")
	if err != nil {
		var re *RecordError
		if errors.As(err, &re) {
			switch {
			case errors.Is(err, ErrNotAccepted):
				return nil, &UnsupportedPriceFormError{OfferID: re.ID}
			case errors.Is(err, ErrNoValue) && re.Capture == amountCapture:
				return nil, &MissingPriceError{OfferID: re.ID}
			case errors.Is(err, ErrDifferentValues):
				return nil, &MixedCurrenciesError{OfferID: re.ID}
			}
		}
		return nil, err
	}
	offers := make([]Offer, len(records))
	for i, rec := range records {
		offers[i] = Offer{
			ID:       rec["id"].(string),
			Owner:    rec["owner"].(string),
			Currency: rec["currency"].(string),
			Total:    rec["total"].(decimal.Decimal),
		}
	}
	return offers, nil
}
