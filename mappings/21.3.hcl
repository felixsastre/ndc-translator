# Mapping for NDC 21.3. Syntax and reductions: see 17.2.hcl.
version = "21.3"

namespaces = {
  m = "http://www.iata.org/IATA/2015/EASD/00/IATA_OffersAndOrdersMessage"
  c = "http://www.iata.org/IATA/2015/EASD/00/IATA_OffersAndOrdersCommonTypes"
}

record "offer" {
  path  = "m:IATA_AirShoppingRS/m:Response/c:OffersGroup/c:CarrierOffers/c:Offer"
  label = "id"

  # In 21.3 the id and the airline are child elements, and the offer states one total price.
  capture "id"       { path = "c:OfferID" }
  capture "owner"    { path = "c:OwnerCode" }
  capture "amount" {
    path     = "c:TotalPrice/c:TotalAmount"
    required = true
  }
  capture "currency" {
    path     = "c:TotalPrice/c:TotalAmount/@CurCode"
    required = true
  }

  output "id" "first" { from = "id" }
  output "owner" "first" { from = "owner" }
  output "total" "sum" { from = "amount" }
  output "currency" "same" { from = "currency" }
}
