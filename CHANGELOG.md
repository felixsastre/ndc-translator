# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/). Edits to a `.feature` go under **Changed**, with the reason.

## [Unreleased]

### Changed
- `Read` rejects an empty body, or one whose root element is not the one the record starts at, with `ErrUnexpectedRoot`. Before, it returned no records and no error, which looked like a response with no offers.
- A required capture does not count an empty value as found. Amounts and totals must also have a currency: `currency` is required in the offer and order mappings.
- Line breaks inside CDATA are normalized to `\n`, as `encoding/xml` does.
- A mapping is rejected if it repeats an output name or writes a prefix on an attribute in a path.
- The encoding in an XML declaration is read as a whole pseudo-attribute, not as the end of another name.
- `features/ndc_offer_reading.feature` and `features/ndc_order_reading.feature`: new scenarios for an offer without a price, an order without a total, an amount or total with no currency, an empty price, and a body of another kind. Each of these used to be read without any warning, as a free offer, an empty currency or a response with no offers.
- `Read` uses a tokenizer that works on the response in memory instead of `encoding/xml`: about 6 times faster, with a twentieth of the memory, and the same results on every fixture. It reads UTF-8 only and does not check that characters are valid XML characters.

### Added
- `required = true` on a capture: the record fails when the capture finds no value. `ErrNoValue`, and `RecordError.Capture` naming the capture. `ReadOffers` reports an offer with no price as `*MissingPriceError`; `ReadOrder` reports an order with no total.
- Behavior specs: `features/ndc_offer_reading.feature` (offers of real 17.2 responses, exact amounts, unsupported price form, mixed currencies, response without offers, a 21.3 response) and `features/ndc_order_reading.feature` (order of real 17.2 responses, passengers and segments in the airline's order, items that add up to the total, response with an error).
- `Read(xml, mapping, record)`: a generic engine that reads one pass over the XML and returns the records a mapping defines. Records have captures (paths) and outputs, each reduced with `first`, `same`, `sum` or `list`. Errors are `*RecordError`, wrapping `ErrDifferentValues` or `ErrNotAccepted`.
- `LoadMapping(version)`: reads `mappings/<version>.hcl`, checks its paths, reductions and capture names. Mappings for 17.2 (offer, order) and 21.3 (offer).
- `ReadOffers(xml, mapping)`: offers with id, airline, currency and total as `decimal.Decimal`. Reports `UnsupportedPriceFormError` and `MixedCurrenciesError` with the offer id.
- `ReadOrder(xml, mapping)`: the order of an OrderView 17.2 response with id, airline, total, base amount, taxes, item ids, passenger ids and segment keys. Returns nil when the response has no order.
- Paths match elements by namespace URI, so the prefix a sender chooses does not matter.
- Tokenizer tests: token-by-token comparison with `encoding/xml`, malformed documents that must fail, and a fuzz test.
- Benchmarks of reading offers and orders, single and from several goroutines.

### Dependencies
- Added `hashicorp/hcl/v2`, `shopspring/decimal`, `cucumber/godog`.
