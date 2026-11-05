package ndctranslator_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/shopspring/decimal"

	nt "github.com/felixsastre/ndc-translator"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Strict:   true, // an undefined or misspelled step must fail the suite
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("feature scenarios failed")
	}
}

// world is the state of one scenario.
type world struct {
	version string
	xml     []byte
	offers  []nt.Offer
	order   *nt.Order
	err     error
}

func initializeScenario(sc *godog.ScenarioContext) {
	w := &world{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		*w = world{}
		return ctx, nil
	})

	sc.Step(`^the (one-way|round-trip) shopping response for (\d+) passenger\(s\)$`, w.realResponse)
	sc.Step(`^the 21\.3 shopping response$`, w.response213)
	sc.Step(`^the (one-way|round-trip) order response for (\d+) traveler\(s\)$`, w.realOrderResponse)
	sc.Step(`^an order response that carries an error and no order$`, w.orderError)
	sc.Step(`^the order is read$`, w.readOrder)
	sc.Step(`^the order is (\S+) from (\S+) priced (\S+) (\S+)$`, w.orderIs)
	sc.Step(`^it has (\d+) items, (\d+) passengers and (\d+) flight segments$`, w.orderHas)
	sc.Step(`^the passengers are (\S+) and (\S+)$`, w.passengersAre)
	sc.Step(`^the flight segments are (\S+), (\S+) and (\S+)$`, w.segmentsAre)
	sc.Step(`^the base amounts of the items add up to (\S+) (\S+)$`, w.baseAddsUp)
	sc.Step(`^their taxes add up to (\S+) (\S+)$`, w.taxesAddUp)
	sc.Step(`^together they equal the order total$`, w.togetherEqualTotal)
	sc.Step(`^there is no order$`, w.noOrder)
	sc.Step(`^an offer (\S+) from (\S+) with no priced items$`, w.noPricedItems)
	sc.Step(`^reading stops and reports that (\S+) has no price$`, w.noPrice)
	sc.Step(`^an order response where order (\S+) states no total$`, w.orderWithoutTotal)
	sc.Step(`^reading stops and reports that (\S+) has no total$`, w.noTotal)
	sc.Step(`^the airline answers with (an empty body|an HTML error page|an order response)$`, w.otherBody)
	sc.Step(`^reading stops and reports that the response is not an AirShopping response$`, w.notExpectedKind)
	sc.Step(`^reading stops and reports that the response is not an order response$`, w.notExpectedKind)
	sc.Step(`^an offer (\S+) from (\S+) priced (\S+) with no currency$`, w.noCurrency)
	sc.Step(`^an offer (\S+) from (\S+) whose price is empty$`, w.emptyPrice)
	sc.Step(`^an order response where order (\S+) states a total with no currency$`, w.orderNoCurrency)
	sc.Step(`^reading stops and reports that (\S+) has no currency$`, w.currencyMissing)
	sc.Step(`^an offer with two items priced (\S+) USD and (\S+) USD$`, w.twoItems)
	sc.Step(`^an offer (\S+) from (\S+) whose price is given as an encoded price$`, w.encodedPrice)
	sc.Step(`^an offer (\S+) with items priced (\S+) USD and (\S+) EUR$`, w.mixedCurrencies)
	sc.Step(`^a shopping response that carries an error and no offers$`, w.errorOnly)
	sc.Step(`^the offers are read$`, w.read)
	sc.Step(`^there are (\d+) offers$`, w.countIs)
	sc.Step(`^the offers are priced (\S+), (\S+) and (\S+) USD$`, w.offersPriced)
	sc.Step(`^the first offer is (\S+) from (\S+) priced (\S+) (\S+)$`, w.firstOffer)
	sc.Step(`^the offer is priced exactly (\S+) (\S+)$`, w.exactTotal)
	sc.Step(`^reading stops and reports that the price form of (\S+) is not supported$`, w.unsupported)
	sc.Step(`^reading stops and reports that (\S+) mixes currencies$`, w.mixed)
}

// load reads a fixture as the response the scenario works with.
func (w *world) load(path string) error {
	data, err := os.ReadFile(path)
	w.xml = data
	return err
}

func (w *world) realResponse(trip, passengers string) error {
	return w.load(fmt.Sprintf("testdata/%s_%spax.xml", strings.ReplaceAll(trip, "-", "_"), passengers))
}

func (w *world) response213() error {
	w.version = "21.3"
	return w.load("testdata/shopping_213.xml")
}

func (w *world) twoItems(a, b string) error {
	w.xml = response(offer("OFFER1", "C9", item("SimpleCurrencyPrice", a, "USD"), item("SimpleCurrencyPrice", b, "USD")))
	return nil
}

func (w *world) encodedPrice(id, owner string) error {
	w.xml = response(offer(id, owner, item("EncodedCurrencyPrice", "10.00", "USD")))
	return nil
}

func (w *world) mixedCurrencies(id, usd, eur string) error {
	w.xml = response(offer(id, "C9", item("SimpleCurrencyPrice", usd, "USD"), item("SimpleCurrencyPrice", eur, "EUR")))
	return nil
}

func (w *world) errorOnly() error {
	w.xml = errorResponse()
	return nil
}

func (w *world) read() error {
	if w.version == "" {
		w.version = "17.2"
	}
	m, err := nt.LoadMapping(w.version)
	if err != nil {
		return err
	}
	w.offers, w.err = nt.ReadOffers(w.xml, m)
	return nil
}

func (w *world) countIs(n int) error {
	if w.err != nil || len(w.offers) != n {
		return fmt.Errorf("got %d offers (error %v), want %d", len(w.offers), w.err, n)
	}
	return nil
}

func (w *world) offersPriced(a, b, c string) error {
	want := []string{a, b, c}
	if w.err != nil || len(w.offers) != len(want) {
		return fmt.Errorf("got %d offers (error %v), want %d", len(w.offers), w.err, len(want))
	}
	for i, total := range want {
		if got := w.offers[i]; !got.Total.Equal(decimal.RequireFromString(total)) || got.Currency != "USD" {
			return fmt.Errorf("offer %d is priced %s %s, want %s USD", i+1, got.Total, got.Currency, total)
		}
	}
	return nil
}

func (w *world) firstOffer(id, owner, total, currency string) error {
	if w.err != nil || len(w.offers) == 0 {
		return fmt.Errorf("no offers read (error %v)", w.err)
	}
	got, want := w.offers[0], decimal.RequireFromString(total)
	if got.ID != id || got.Owner != owner || !got.Total.Equal(want) || got.Currency != currency {
		return fmt.Errorf("first offer is %+v, want %s from %s priced %s %s", got, id, owner, total, currency)
	}
	return nil
}

func (w *world) exactTotal(total, currency string) error {
	if w.err != nil || len(w.offers) != 1 {
		return fmt.Errorf("want one offer, got %d (error %v)", len(w.offers), w.err)
	}
	got := w.offers[0]
	if !got.Total.Equal(decimal.RequireFromString(total)) || got.Currency != currency {
		return fmt.Errorf("offer is priced %s %s, want %s %s", got.Total, got.Currency, total, currency)
	}
	return nil
}

func (w *world) unsupported(id string) error {
	var e *nt.UnsupportedPriceFormError
	if !errors.As(w.err, &e) || e.OfferID != id {
		return fmt.Errorf("error is %v, want an unsupported price form for %s", w.err, id)
	}
	return nil
}

func (w *world) mixed(id string) error {
	var e *nt.MixedCurrenciesError
	if !errors.As(w.err, &e) || e.OfferID != id {
		return fmt.Errorf("error is %v, want mixed currencies for %s", w.err, id)
	}
	return nil
}

func (w *world) realOrderResponse(trip, travelers string) error {
	return w.load(fmt.Sprintf("testdata/order_%s_%spax.xml", strings.ReplaceAll(trip, "-", "_"), travelers))
}

func (w *world) orderError() error {
	w.xml = orderErrorResponse()
	return nil
}

func (w *world) readOrder() error {
	m, err := nt.LoadMapping("17.2")
	if err != nil {
		return err
	}
	w.order, w.err = nt.ReadOrder(w.xml, m)
	return nil
}

// requireOrder is the order a Then step looks at, or an error saying why there is none.
func (w *world) requireOrder() (*nt.Order, error) {
	if w.order == nil {
		return nil, fmt.Errorf("no order was read (error %v)", w.err)
	}
	return w.order, nil
}

func (w *world) orderIs(id, owner, total, currency string) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	if o.ID != id || o.Owner != owner || !o.Total.Equal(decimal.RequireFromString(total)) || o.Currency != currency {
		return fmt.Errorf("order is %s from %s priced %s %s, want %s from %s priced %s %s",
			o.ID, o.Owner, o.Total, o.Currency, id, owner, total, currency)
	}
	return nil
}

func (w *world) orderHas(items, passengers, segments int) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	if len(o.Items) != items || len(o.Passengers) != passengers || len(o.Segments) != segments {
		return fmt.Errorf("order has %d items, %d passengers, %d segments; want %d, %d, %d",
			len(o.Items), len(o.Passengers), len(o.Segments), items, passengers, segments)
	}
	return nil
}

func (w *world) passengersAre(a, b string) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	return sameList("passengers", o.Passengers, []string{a, b})
}

func (w *world) segmentsAre(a, b, c string) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	return sameList("flight segments", o.Segments, []string{a, b, c})
}

func sameList(what string, got, want []string) error {
	if !slices.Equal(got, want) {
		return fmt.Errorf("%s are %v, want %v", what, got, want)
	}
	return nil
}

func (w *world) baseAddsUp(total, currency string) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	return sameAmount("base amounts", o.BaseAmount, total, o.Currency, currency)
}

func (w *world) taxesAddUp(total, currency string) error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	return sameAmount("taxes", o.Taxes, total, o.Currency, currency)
}

func sameAmount(what string, got decimal.Decimal, want, gotCurrency, wantCurrency string) error {
	if !got.Equal(decimal.RequireFromString(want)) || gotCurrency != wantCurrency {
		return fmt.Errorf("%s add up to %s %s, want %s %s", what, got, gotCurrency, want, wantCurrency)
	}
	return nil
}

func (w *world) togetherEqualTotal() error {
	o, err := w.requireOrder()
	if err != nil {
		return err
	}
	if sum := o.BaseAmount.Add(o.Taxes); !sum.Equal(o.Total) {
		return fmt.Errorf("base and taxes add up to %s, the order total is %s", sum, o.Total)
	}
	return nil
}

func (w *world) noOrder() error {
	if w.order != nil {
		return fmt.Errorf("an order was read: %+v", *w.order)
	}
	return nil
}

func (w *world) noPricedItems(id, owner string) error {
	w.xml = response(offer(id, owner))
	return nil
}

func (w *world) noPrice(id string) error {
	var e *nt.MissingPriceError
	if !errors.As(w.err, &e) || e.OfferID != id {
		return fmt.Errorf("error is %v, want a missing price for %s", w.err, id)
	}
	return nil
}

func (w *world) orderWithoutTotal(id string) error {
	w.xml = orderWithoutTotalResponse(id)
	return nil
}

// missingCapture checks that reading failed because a required capture of the record found nothing.
func (w *world) missingCapture(id, capture string) error {
	var e *nt.RecordError
	if !errors.As(w.err, &e) || !errors.Is(w.err, nt.ErrNoValue) || e.ID != id || e.Capture != capture {
		return fmt.Errorf("error is %v, want no %s for %s", w.err, capture, id)
	}
	return nil
}

func (w *world) noTotal(id string) error { return w.missingCapture(id, "total") }

func (w *world) otherBody(kind string) error {
	switch kind {
	case "an empty body":
		w.xml = nil
	case "an HTML error page":
		w.xml = []byte("<html><body><h1>503 Service Unavailable</h1></body></html>")
	default:
		return w.load("testdata/order_one_way_1pax.xml")
	}
	return nil
}

func (w *world) notExpectedKind() error {
	if !errors.Is(w.err, nt.ErrUnexpectedRoot) {
		return fmt.Errorf("error is %v, want a response of another kind", w.err)
	}
	return nil
}

func (w *world) noCurrency(id, owner, amount string) error {
	w.xml = response(offer(id, owner, item("SimpleCurrencyPrice", amount, "")))
	return nil
}

func (w *world) emptyPrice(id, owner string) error {
	w.xml = response(offer(id, owner, item("SimpleCurrencyPrice", "", "USD")))
	return nil
}

func (w *world) orderNoCurrency(id string) error {
	w.xml = orderNoCurrencyResponse(id)
	return nil
}

func (w *world) currencyMissing(id string) error { return w.missingCapture(id, "currency") }
