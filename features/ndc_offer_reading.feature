# Assumption: the price of an offer is the sum of the total amounts of its items, one item per passenger.
# Assumption: all the items of an offer share one currency; a mix of currencies is rejected, never converted.
# Assumption: only the simple price form (an amount and a currency code) is supported. Other forms are reported as unsupported, never guessed.
# Assumption: in 21.3 the price of an offer is its total price, which the airline states once for the whole offer.
# Assumption: an offer always has at least one price. One without it is malformed and is reported, never read as free.
# Assumption: an empty body, or a body whose root element is not the one this kind of message has, is not a response with zero offers. It is rejected, so an airline's error page is never mistaken for "no offers".
# Assumption: an amount always has a currency, and an empty price counts as no price. Both are reported.
# Assumption: a response that carries an error and no offers has zero offers; it is not a failure of reading.

Feature: Reading the offers of an NDC 17.2 shopping response
  An airline answers a shopping request with offers. The reader turns the XML of that
  response into plain offers: an id, the airline that owns it and a total price.
  Reading must be fast, because a distributor reads many responses per second.

  Scenario Outline: The offers of a real response are read with their total price
    Given the <trip> shopping response for <passengers> passenger(s)
    When the offers are read
    Then there are <offers> offers
    And the first offer is OFFER1 from C9 priced <first_total> USD

    Examples:
      | trip       | passengers | offers | first_total |
      | one-way    | 1          | 90     | 1370.23     |
      | one-way    | 2          | 90     | 2740.46     |
      | round-trip | 1          | 60     | 1370.23     |
      | round-trip | 2          | 60     | 2740.46     |

  Scenario: A 21.3 response is read through its own paths
    Given the 21.3 shopping response
    When the offers are read
    Then there are 3 offers
    And the offers are priced 1370.23, 2740.46 and 998.00 USD

  Scenario: Amounts keep their cents
    Given an offer with two items priced 0.10 USD and 0.20 USD
    When the offers are read
    Then the offer is priced exactly 0.30 USD

  Scenario: A price in an unsupported form is reported with the offer it belongs to
    Given an offer OFFER7 from C9 whose price is given as an encoded price
    When the offers are read
    Then reading stops and reports that the price form of OFFER7 is not supported

  Scenario: Items in different currencies are not added together
    Given an offer OFFER3 with items priced 100.00 USD and 90.00 EUR
    When the offers are read
    Then reading stops and reports that OFFER3 mixes currencies

  Scenario: An offer without a price is reported
    Given an offer OFFER5 from C9 with no priced items
    When the offers are read
    Then reading stops and reports that OFFER5 has no price

  Scenario Outline: A body that is not an AirShopping response is rejected
    Given the airline answers with <body>
    When the offers are read
    Then reading stops and reports that the response is not an AirShopping response

    Examples:
      | body               |
      | an empty body      |
      | an HTML error page |
      | an order response  |

  Scenario: An offer priced without a currency is reported
    Given an offer OFFER6 from C9 priced 100.00 with no currency
    When the offers are read
    Then reading stops and reports that OFFER6 has no currency

  Scenario: An offer whose price is empty is reported as having no price
    Given an offer OFFER7 from C9 whose price is empty
    When the offers are read
    Then reading stops and reports that OFFER7 has no price

  Scenario: A response with an error and no offers has zero offers
    Given a shopping response that carries an error and no offers
    When the offers are read
    Then there are 0 offers
