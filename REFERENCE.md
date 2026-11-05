# Technical reference

For developers who want to know how `ndc-translator` works inside. The [README](README.md) explains what it is and what it is for.

The library reads NDC responses into plain values: the offers of an AirShopping response and the order of an OrderView response. What each NDC version looks like is described in one HCL file per version, so supporting a new version means writing a new file and leaving the reader alone.

## Using it

```go
m, err := ndctranslator.LoadMapping("17.2")

offers, err := ndctranslator.ReadOffers(xml, m) // []Offer: ID, Owner, Currency, Total
order, err := ndctranslator.ReadOrder(xml, m)   // *Order, or nil if the response has no order
```

`ReadOffers` returns `*UnsupportedPriceFormError` when an offer is priced in a form the mapping does not accept, `*MixedCurrenciesError` when its items use different currencies, and `*MissingPriceError` when it has no price at all. All three carry the offer id. A required value that is missing or empty, such as an amount without a currency, comes back as a `*RecordError` wrapping `ErrNoValue`, and so does an order with no total. A body that is empty or whose root element is not the one the record starts at, such as an HTML error page or another kind of message, returns an error wrapping `ErrUnexpectedRoot`. It is never read as a response with no records.

`Read(xml, m, "offer")` is the engine underneath. It returns a `[]Record` for any record the mapping defines, and `ReadOffers` and `ReadOrder` are thin readers over it.

## How a mapping works

A mapping file such as `mappings/17.2.hcl` defines records. A record is the part of a response to read: an offer, which repeats, or the whole response when it holds a single order. Each record has captures and outputs.

- A capture collects every value its path finds inside the record.
- An output reduces the values of one capture to a single value.

Here is an offer from a two-passenger response, abridged. It has one item per passenger:

```xml
<Offer OfferID="OFFER1" Owner="C9">
  <OfferItem>
    ... <SimpleCurrencyPrice Code="USD">1370.23</SimpleCurrencyPrice> ...
  </OfferItem>
  <OfferItem>
    ... <SimpleCurrencyPrice Code="USD">1370.23</SimpleCurrencyPrice> ...
  </OfferItem>
</Offer>
```

and the part of the mapping that reads it:

```hcl
record "offer" {
  path  = "n:AirShoppingRS/n:OffersGroup/n:AirlineOffers/n:Offer"
  label = "id"

  capture "id" { path = "@OfferID" }
  capture "amount" {
    path     = "n:OfferItem/n:TotalPriceDetail/n:TotalAmount/*"
    accept   = ["SimpleCurrencyPrice"]
    required = true
  }
  capture "currency" { path = "n:OfferItem/n:TotalPriceDetail/n:TotalAmount/*/@Code" }

  output "id"       "first" { from = "id" }
  output "total"    "sum"   { from = "amount" }
  output "currency" "same"  { from = "currency" }
}
```

The `amount` capture finds `1370.23` twice and `sum` turns them into `2740.46`. The `currency` capture finds `USD` twice and `same` returns `USD`. If one item were priced in EUR, `same` would fail and the offer would be reported as mixing currencies.

### Paths

A path is a list of steps separated by `/`. A step is `prefix:name`, a name with no prefix for an element with no namespace, or `*` for any element. A path can end in `@attribute`. Without an attribute it reads the text of the element it reaches, and `.` is the element itself.

Attributes are matched by local name, so `@Code` also matches `x:Code`, and an attribute in a path takes no prefix. The record path starts at the root of the response, and the root element has to be its first step. Capture paths start at the record. Elements are matched by namespace URI, so the prefix a sender chooses does not matter.

### Reductions

| Name | Gives | With no values | Fails when |
|---|---|---|---|
| `first` | the first value, as a string | `""` | never |
| `same` | the value they all share, as a string | `""` | the values differ |
| `sum` | their sum, as a decimal | zero | a value is not an amount |
| `list` | all of them, in the order they appear in the XML | an empty list | never |

Real examples from the 17.2 order of a round trip with two travelers:

- `list` of the passenger ids gives `[SH2 SH1]`, in the order the airline lists them.
- `sum` of the base amounts of the four order items gives `2718.00`, and `sum` of their taxes gives `3153.00`. Together they equal the order total, `5871.00`.

A single value summed gives that value as a decimal, which is how the order total is read.

### accept

A capture can list the element names it accepts. If its path reaches an element with another name, the record fails. The offer mapping uses it on the price element, so an offer priced in a form other than `SimpleCurrencyPrice` is reported and not read as zero.

### required

A capture marked `required = true` fails the record when it finds no value, and an empty value does not count. Without it, a capture that finds nothing gives the empty value of its reduction: `sum` gives zero and `same` gives `""`. That is wrong for money, because an offer with no price items would read as free and an amount with no currency would carry an empty one. The offer mapping marks `amount` and `currency` as required, and the order mapping does the same for `total` and `currency`.

## Adding a version or a message

1. Write `mappings/<version>.hcl`. Copy the closest existing file and change the namespaces and paths.
2. Write a record for each message to read. Name it the way the typed reader expects: `ReadOffers` needs a record `offer` with outputs `id`, `owner`, `currency` and `total`, and its captures `amount` and `currency` marked required. `ReadOrder` needs a record `order` with its captures `total` and `currency` marked required. `LoadMapping` checks the paths, the reductions and the capture names, and the readers check the outputs they need.
3. Add a scenario to a feature file and a fixture to `testdata/`.

A new kind of value that none of the four reductions covers is a new function in `reductions.go`, added to the `reductions` map.

## Limits

- The XML is read once, so a record can only use what is inside it. The passengers and segments of an order are listed after the order, so the `order` record is the whole response.
- Paths have no conditions or functions. Logic beyond the reductions is Go code.
- A text capture reads the text directly inside an element, not the text of the elements nested in it.
- An id that points at another list in the response, such as a service reference, is read as a value. The engine does not follow it.

## Run

```
go test -race ./...
go test -run '^$' -bench . -benchmem          # cost of reading one response
go test -run '^$' -bench Parallel -cpu 1,4,8  # throughput by number of cores
```

## The tokenizer

`Read` does not use `encoding/xml`. It uses a tokenizer in `tokenizer.go` that works on the response already in memory and hands out slices of it, so reading a 315 KB response makes 12 allocations instead of 53 thousand. It supports what NDC messages use: elements, attributes with either kind of quote, namespaces, text with the predefined and numeric entities, CDATA sections, comments, processing instructions and a DOCTYPE.

What it does not do:

- It reads every document as UTF-8 and fails if the XML declaration names another encoding.
- It does not check that characters are valid XML characters, and it does not enforce all of XML's well-formedness rules: it accepts several root elements, repeated attributes and a raw `<` in an attribute value. It does require a root element. Because of this it can accept a document `encoding/xml` rejects.
- It skips a DOCTYPE without reading it. An entity declared there is not expanded, and a reference to it is an error. Nothing is fetched from the network.

The tests hold it to the same results as `encoding/xml`: a token-by-token comparison on every fixture and on a list of hand-written documents (line breaks, CDATA, comments, nested namespaces), a list of malformed documents that must fail, and a fuzz test that checks that whatever `encoding/xml` accepts, the tokenizer reads the same way. The fuzz test keeps the inputs that once found a difference in `testdata/fuzz/`, and `go test` replays them. To fuzz for a while:

```
go test -run '^$' -fuzz FuzzTokenizerMatchesEncodingXML -fuzztime 60s
```

## Measurements

Intel i5-8257U, one core, the 315 KB one-way 17.2 response with 90 offers. Times from different days can differ by a third on this machine, so only rows measured together are comparable.

| What runs | Time | Memory | Allocations |
|---|---|---|---|
| `encoding/xml` reading tokens, extracting nothing | 9.2 ms | 1.7 MB | 53k |
| The module's tokenizer, extracting nothing | 1.2 ms | 864 B | 12 |
| `ReadOffers` on `encoding/xml` (the previous version) | 9.5 ms | 1.8 MB | 55k |
| `ReadOffers` | 1.6 ms | 74 KB | 1.8k |

Two results drove the design. Reading the XML once, without building a tree, took about half the time and a third of the memory of the first version, which built the tree and queried it with XPath. Replacing `encoding/xml` made it about six times faster, because most of that parser's time went to allocating tokens. For reference, Python with `lxml` parsing the same response and reading the offers with XPath took 5.2 ms, but it builds the whole tree and checks well-formedness more strictly.

Bigger responses take proportionally longer, and reading from several goroutines scales with the physical cores (4 here). Reductions are named functions and not HCL expressions, so a mapping never holds logic; the engine looks the function up once when the file loads.
