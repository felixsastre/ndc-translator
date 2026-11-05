# Assumption: the total of an order is the amount the airline states for the whole order; it is read, not recomputed.
# Assumption: the passengers and the flight segments of an order are listed at the end of the response, outside the order. A response holds one order.
# Assumption: the passengers and the segments keep the order in which the airline lists them.
# Assumption: a body whose root element is not an order response is rejected, never read as an empty order.
# Assumption: an order always states its total, and the total has a currency. One without it is malformed and is reported, never read as free.
# Assumption: in a settled order the base amounts and the taxes of its items add up to the order total. This holds in the sample responses; the standard does not require it.

Feature: Reading the order of an NDC 17.2 order response
  After a booking the airline answers with the order. The reader turns that XML into a plain order:
  its id, the airline, the total, its items, and the passengers and flight segments it covers.

  Scenario Outline: The order of a real response is read
    Given the <trip> order response for <travelers> traveler(s)
    When the order is read
    Then the order is <order> from C9 priced <total> USD
    And it has <items> items, <passengers> passengers and <segments> flight segments

    Examples:
      | trip       | travelers | order  | total   | items | passengers | segments |
      | one-way    | 1         | D16BT5 | 1177.38 | 1     | 1          | 1        |
      | one-way    | 2         | D16BR5 | 2354.76 | 2     | 2          | 1        |
      | round-trip | 1         | D16EV5 | 2935.50 | 2     | 1          | 3        |
      | round-trip | 2         | D16EX5 | 5871.00 | 4     | 2          | 3        |

  Scenario: Passengers are listed in the order the airline gives them
    Given the one-way order response for 2 traveler(s)
    When the order is read
    Then the passengers are SH2 and SH1

  Scenario: Flight segments are listed in the order the airline gives them
    Given the round-trip order response for 2 traveler(s)
    When the order is read
    Then the flight segments are SEG_CDGDXB_1, SEG_DXBFRA_2 and SEG_FRACDG_18

  Scenario: The items add up to the order total
    Given the round-trip order response for 2 traveler(s)
    When the order is read
    Then the base amounts of the items add up to 2718.00 USD
    And their taxes add up to 3153.00 USD
    And together they equal the order total

  Scenario: An order without a total is reported
    Given an order response where order D16XX1 states no total
    When the order is read
    Then reading stops and reports that D16XX1 has no total

  Scenario: A response that is not an order response is rejected
    Given the one-way shopping response for 1 passenger(s)
    When the order is read
    Then reading stops and reports that the response is not an order response

  Scenario: An order whose total has no currency is reported
    Given an order response where order D16XX2 states a total with no currency
    When the order is read
    Then reading stops and reports that D16XX2 has no currency

  Scenario: A response that carries an error has no order
    Given an order response that carries an error and no order
    When the order is read
    Then there is no order
