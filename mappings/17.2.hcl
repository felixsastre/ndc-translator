# Mapping for NDC 17.2: which records to read from a response and what to take from them.
#
# Paths are steps separated by "/": "prefix:name", or "*" for any element. They end in "@attribute"
# for an attribute, or in nothing for the text of the element they reach. "." is the element itself.
#
# A record is an element that repeats (an offer) or that holds the whole answer (an order). Its
# "path" starts at the root. Each capture collects every value its path finds inside the record
# (paths relative to the record). Each output reduces the values of one capture to a single value:
#   first  the first value, or "" if there is none
#   same   the value they all share (an error if they differ), or "" if there is none
#   sum    their sum as a decimal amount (zero if there is none)
#   list   all of them, in the order they appear in the XML
# Example: output "total" "sum" { from = "amount" } makes the output total the sum of the capture amount.
version = "17.2"

namespaces = {
  n = "http://www.iata.org/IATA/EDIST/2017.2"
}

record "offer" {
  path  = "n:AirShoppingRS/n:OffersGroup/n:AirlineOffers/n:Offer"
  label = "id"

  capture "id"    { path = "@OfferID" }
  capture "owner" { path = "@Owner" }
  # The price element of every item of the offer, whatever its form; only this form is read.
  capture "amount" {
    path     = "n:OfferItem/n:TotalPriceDetail/n:TotalAmount/*"
    accept   = ["SimpleCurrencyPrice"]
    required = true
  }
  capture "currency" {
    path     = "n:OfferItem/n:TotalPriceDetail/n:TotalAmount/*/@Code"
    required = true
  }

  output "id" "first" { from = "id" }
  output "owner" "first" { from = "owner" }
  output "total" "sum" { from = "amount" }
  output "currency" "same" { from = "currency" }
}

# The answer to a booking. The passengers and the segments are listed after the order, so the
# record is the whole response, and the order is one of its parts.
record "order" {
  path  = "n:OrderViewRS/n:Response"
  label = "id"

  capture "id"        { path = "n:Order/@OrderID" }
  capture "owner"     { path = "n:Order/@Owner" }
  capture "total" {
    path     = "n:Order/n:TotalOrderPrice/n:SimpleCurrencyPrice"
    required = true
  }
  capture "currency" {
    path     = "n:Order/n:TotalOrderPrice/n:SimpleCurrencyPrice/@Code"
    required = true
  }
  capture "item"      { path = "n:Order/n:OrderItems/n:OrderItem/@OrderItemID" }
  capture "base"      { path = "n:Order/n:OrderItems/n:OrderItem/n:PriceDetail/n:BaseAmount" }
  capture "taxes"     { path = "n:Order/n:OrderItems/n:OrderItem/n:PriceDetail/n:Taxes/n:Total" }
  capture "passenger" { path = "n:DataLists/n:PassengerList/n:Passenger/@PassengerID" }
  capture "segment"   { path = "n:DataLists/n:FlightSegmentList/n:FlightSegment/@SegmentKey" }

  output "id" "first" { from = "id" }
  output "owner" "first" { from = "owner" }
  output "total" "sum" { from = "total" }
  output "currency" "same" { from = "currency" }
  output "items" "list" { from = "item" }
  output "base" "sum" { from = "base" }
  output "taxes" "sum" { from = "taxes" }
  output "passengers" "list" { from = "passenger" }
  output "segments" "list" { from = "segment" }
}
