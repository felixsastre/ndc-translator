// Package ndctranslator reads NDC responses from airlines into plain Go values.
//
// NDC is the IATA standard that airlines use to offer and sell tickets directly. An airline
// answers a search with an XML message, and the standard comes in several versions that put the
// same information in different places. This package reads the offers of an AirShopping response
// and the order of an OrderView response, and gives the same result whatever the version.
//
// What differs between versions is kept in one HCL mapping file per version, in the mappings
// folder. A mapping lists the records to read, the path to each value inside a record and how to
// reduce several values to one. The format is described in REFERENCE.md. LoadMapping reads a
// mapping, and ReadOffers and ReadOrder use it:
//
//	mapping, err := ndctranslator.LoadMapping("17.2")
//	offers, err := ndctranslator.ReadOffers(response, mapping)
//
// Read is the engine under both. It goes through the XML once and returns the records that a
// mapping defines. A problem in the response comes back as an error that names the offer or the
// order: MissingPriceError, MixedCurrenciesError, UnsupportedPriceFormError, or a RecordError that
// wraps ErrNoValue. A body that is empty, or is not the expected kind of message, wraps
// ErrUnexpectedRoot.
//
// The files, by role:
//
//   - tokenizer.go reads XML held in memory, without copying it.
//   - path.go parses the paths of a mapping and matches them against the open elements.
//   - reductions.go holds the functions a mapping can name: first, same, sum and list.
//   - mapping.go loads and checks the mapping files.
//   - reader.go is the engine behind Read.
//   - offers.go and orders.go are the typed readers built on it.
package ndctranslator
