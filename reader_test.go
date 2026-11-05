package ndctranslator_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"testing/quick"

	"github.com/shopspring/decimal"

	nt "github.com/felixsastre/ndc-translator"
)

func mustMapping(t testing.TB) *nt.Mapping {
	t.Helper()
	m, err := nt.LoadMapping("17.2")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLoadMappingUnknownVersion(t *testing.T) {
	if _, err := nt.LoadMapping("1.0"); err == nil {
		t.Error("want an error for a version without a mapping file")
	}
}

func TestReadOffersMalformedXML(t *testing.T) {
	if _, err := nt.ReadOffers([]byte("<AirShoppingRS>"), mustMapping(t)); err == nil {
		t.Error("want an error for XML that does not parse")
	}
}

func TestReadOffersAmountThatIsNotANumber(t *testing.T) {
	xml := response(offer("OFFER1", "C9", item("SimpleCurrencyPrice", "abc", "USD")))
	if _, err := nt.ReadOffers(xml, mustMapping(t)); err == nil {
		t.Error("want an error for an amount that is not a number")
	}
}

// cents turns arbitrary numbers into amounts with two decimals, as the airline writes them.
func cents(values []uint16) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = decimal.New(int64(v), -2).StringFixed(2)
	}
	return out
}

func offerWith(id string, amounts []string) string {
	items := make([]string, len(amounts))
	for i, a := range amounts {
		items[i] = item("SimpleCurrencyPrice", a, "USD")
	}
	return offer(id, "C9", items...)
}

// Property: an offer's total is the sum of its item amounts, whatever their order.
func TestTotalIsTheSumOfItemsInAnyOrder(t *testing.T) {
	m := mustMapping(t)
	prop := func(values []uint16) bool {
		if len(values) == 0 {
			return true
		}
		amounts := cents(values)
		want := decimal.Zero
		for _, a := range amounts {
			want = want.Add(decimal.RequireFromString(a))
		}
		reversed := make([]string, len(amounts))
		for i, a := range amounts {
			reversed[len(amounts)-1-i] = a
		}
		for _, order := range [][]string{amounts, reversed} {
			offers, err := nt.ReadOffers(response(offerWith("O", order)), m)
			if err != nil || len(offers) != 1 || !offers[0].Total.Equal(want) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Error(err)
	}
}

// Property: the number of offers read equals the number of offers in the XML.
func TestEveryOfferInTheXMLIsRead(t *testing.T) {
	m := mustMapping(t)
	prop := func(n uint8) bool {
		offers := make([]string, n)
		for i := range offers {
			offers[i] = offerWith("OFFER"+strconv.Itoa(i), []string{"1.00"})
		}
		got, err := nt.ReadOffers(response(offers...), m)
		return err == nil && len(got) == int(n)
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Error(err)
	}
}

func BenchmarkReadOffers(b *testing.B) {
	m := mustMapping(b)
	for _, name := range []string{"round_trip_1pax", "one_way_1pax", "round_trip_2pax", "one_way_2pax"} {
		data := mustRead(b, fmt.Sprintf("testdata/%s.xml", name))
		b.Run(fmt.Sprintf("%s_%dKB", name, len(data)/1024), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if _, err := nt.ReadOffers(data, m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkReadOffersParallel reads the largest response from many goroutines at once; run it
// with -cpu 1,4,8 to see how throughput grows with the cores.
func BenchmarkReadOffersParallel(b *testing.B) {
	m := mustMapping(b)
	data := mustRead(b, "testdata/one_way_2pax.xml")
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := nt.ReadOffers(data, m); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

const prefixed = `<q:AirShoppingRS xmlns:q="%s"><q:OffersGroup><q:AirlineOffers>` +
	`<q:Offer OfferID="OFFER1" Owner="C9"><q:OfferItem><q:TotalPriceDetail><q:TotalAmount>` +
	`<q:SimpleCurrencyPrice Code="USD">10.00</q:SimpleCurrencyPrice>` +
	`</q:TotalAmount></q:TotalPriceDetail></q:OfferItem></q:Offer>` +
	`</q:AirlineOffers></q:OffersGroup></q:AirShoppingRS>`

// The namespace is what counts, not the prefix the sender chose for it.
func TestReadOffersDoesNotDependOnThePrefix(t *testing.T) {
	xml := []byte(fmt.Sprintf(prefixed, namespace))
	offers, err := nt.ReadOffers(xml, mustMapping(t))
	if err != nil || len(offers) != 1 || offers[0].ID != "OFFER1" {
		t.Errorf("got %+v, %v; want OFFER1", offers, err)
	}
}

func TestReadOffersRejectsAResponseOfAnotherNamespace(t *testing.T) {
	xml := []byte(fmt.Sprintf(prefixed, "http://example.com/other"))
	if _, err := nt.ReadOffers(xml, mustMapping(t)); !errors.Is(err, nt.ErrUnexpectedRoot) {
		t.Errorf("got %v, want ErrUnexpectedRoot", err)
	}
}

func TestReadOrderRejectsAResponseWithTwoOrders(t *testing.T) {
	order := `<Response><Order OrderID="A" Owner="C9"><TotalOrderPrice><SimpleCurrencyPrice Code="USD">1.00` +
		`</SimpleCurrencyPrice></TotalOrderPrice></Order></Response>`
	xml := []byte(`<OrderViewRS xmlns="` + namespace + `">` + order + order + `</OrderViewRS>`)
	if _, err := nt.ReadOrder(xml, mustMapping(t)); err == nil {
		t.Error("want an error for a response that holds two orders")
	}
}

func BenchmarkReadOrder(b *testing.B) {
	m := mustMapping(b)
	for _, name := range []string{"order_one_way_1pax", "order_round_trip_2pax"} {
		data := mustRead(b, fmt.Sprintf("testdata/%s.xml", name))
		b.Run(fmt.Sprintf("%s_%dKB", name, len(data)/1024), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				if _, err := nt.ReadOrder(data, m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
