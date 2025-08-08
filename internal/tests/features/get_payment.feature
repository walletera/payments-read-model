Feature: Get Payment by Id

  Background: the payments-read-model is up and running
    Given a running payments-read-model
    And a list of payment created events is published and processed successfully by the payments-read-model:
    """
    data/payment_created_events_list.json
    """

  Scenario Outline: payments can be query by id
    When the payments-read-model receives a GET request on endpoint /payments/paymentId with paymentId <paymentId>
    Then the payments-read-model respond with status code <statusCode>
    And payment id <responsePaymentId>
    And external id <externalId>
    And status <status>

    Examples:
      | paymentId                            | statusCode | responsePaymentId                    | externalId     | status   |
      | 0ae1733e-7538-4908-b90a-5721670cb004 | 200        | 0ae1733e-7538-4908-b90a-5721670cb004 | EXTERNAL-ID-04 | rejected |
      | 39947bb3-47ec-4f91-9ddf-74cde04085c4 | 404        | -                                    | -              | -        |
      | xxxxxxxx-7538-4908-b90a-yyyyyyyyyyyy | 400        | -                                    | -              | -        |