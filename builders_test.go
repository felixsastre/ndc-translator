package ndctranslator_test

import (
	"os"
	"strings"
	"testing"
)

const namespace = "http://www.iata.org/IATA/EDIST/2017.2"

// item builds an offer item priced in the given form ("SimpleCurrencyPrice", ...). With an empty
// currency the price element has no Code attribute.
func item(form, amount, currency string) string {
	code := ""
	if currency != "" {
		code = ` Code="` + currency + `"`
	}
	return `<OfferItem OfferItemID="I"><TotalPriceDetail><TotalAmount>` +
		`<` + form + code + `>` + amount + `</` + form + `>` +
		`</TotalAmount></TotalPriceDetail></OfferItem>`
}

func offer(id, owner string, items ...string) string {
	return `<Offer OfferID="` + id + `" Owner="` + owner + `">` + strings.Join(items, "") + `</Offer>`
}

func response(offers ...string) []byte {
	return []byte(`<AirShoppingRS xmlns="` + namespace + `" Version="IATA2017.2">` +
		`<OffersGroup><AirlineOffers>` + strings.Join(offers, "") + `</AirlineOffers></OffersGroup>` +
		`</AirShoppingRS>`)
}

func errorResponse() []byte {
	return []byte(`<AirShoppingRS xmlns="` + namespace + `" Version="IATA2017.2">` +
		`<Errors><Error ShortText="No offers" Code="1"/></Errors></AirShoppingRS>`)
}

func mustRead(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func orderErrorResponse() []byte {
	return []byte(`<OrderViewRS xmlns="` + namespace + `" Version="IATA2017.2">` +
		`<Errors><Error ShortText="Order not found" Code="2"/></Errors></OrderViewRS>`)
}

// orderResponse wraps what goes inside the Response element of an order response.
func orderResponse(inside string) []byte {
	return []byte(`<OrderViewRS xmlns="` + namespace + `" Version="IATA2017.2"><Response>` +
		inside + `</Response></OrderViewRS>`)
}

func orderWithoutTotalResponse(id string) []byte {
	return orderResponse(`<Order OrderID="` + id + `" Owner="C9"><OrderItems/></Order>`)
}

func orderNoCurrencyResponse(id string) []byte {
	return orderResponse(`<Order OrderID="` + id + `" Owner="C9"><TotalOrderPrice>` +
		`<SimpleCurrencyPrice>100.00</SimpleCurrencyPrice></TotalOrderPrice><OrderItems/></Order>`)
}
