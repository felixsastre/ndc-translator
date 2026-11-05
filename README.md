# ndc-translator

Airlines sell tickets online through a standard called NDC. When you search for a flight, the airline answers with a long XML message, and the standard comes in several versions that organize the same information in different ways. This library reads those messages and gives back the same simple result whatever version arrived: the flight offers with their prices, or a booking with its passengers and flights.

It is a learning project written in Go. It is small, it is tested against real sample messages from IATA (the airlines' trade association), and it is built so that supporting a new version means adding a file and leaving the code alone.

## About this project

This is an experiment, made for research and out of curiosity about how airline distribution works. It is not prepared for production use, and it should not be used with real bookings, fares or traffic.

## Words used here

- NDC: New Distribution Capability, the IATA standard airlines use to offer and sell tickets directly.
- Offer: one option an airline proposes in answer to a search, such as a flight with its price.
- Order: a booking, with its total, its passengers and its flights.
- Version: NDC changes over time. This library reads versions 17.2 and 21.3.
- Mapping: a short file that says where each piece of information sits in one version.

## What it does

Given an airline's answer, it returns plain results. For one of the sample responses, a one-way search for two passengers, an offer comes back as:

```
Offer OFFER1, airline C9, total 2740.46 USD
```

and a booking from another sample as:

```
Order D16BR5, airline C9, total 2354.76 USD
2 passengers (SH2, SH1), 1 flight (SEG_CDGDXB_1)
```

In code it takes two lines:

```go
mapping, _ := ndctranslator.LoadMapping("17.2")
offers, err := ndctranslator.ReadOffers(airlineResponse, mapping)
```

## Why there are mapping files

The same information sits in a different place in each version. The airline's code, for example, is written one way in 17.2 and another way in 21.3. Instead of writing separate code for every version, each version gets a mapping file that lists where to look, and one piece of code reads them all. When a new version appears, someone writes a new file. The two that exist are `mappings/17.2.hcl` and `mappings/21.3.hcl`.

## It does not guess

When a message is wrong or incomplete, the library says so and does not fill the gap. It reports:

- an offer with no price, which is never shown as free;
- an amount with no currency;
- an empty body or an error page from the airline, which is never read as "no flights available";
- an offer priced in more than one currency, or in a price format it does not support, together with the offer's id.

## What is in the repository

- `features/` describes the behavior in plain English. Each scenario there is also an automated test.
- `mappings/` has one file per NDC version.
- `testdata/` has sample messages from IATA, plus one written by hand for version 21.3.
- The `.go` files are the code.
- `REFERENCE.md` has the technical detail for developers.

## What it supports today

| Message | Versions |
|---|---|
| Flight offers (AirShopping) | 17.2, 21.3 |
| A booking (OrderView) | 17.2 |

## Limits

This is a learning project and not a production system. It has not been tried on live airline traffic. The IATA samples are used under their MIT license. The version 21.3 sample was written by hand and checked against the 21.3 schema, so it may differ from what real airlines send.

## License

MIT, see `LICENSE`. The sample messages in `testdata/` come from IATA's own samples, which are also MIT licensed (`testdata/LICENSE`).

## Run it

You need Go installed. From this folder:

```
go test -race ./...
```

The tests read every scenario in `features/` and check it against the sample messages.
