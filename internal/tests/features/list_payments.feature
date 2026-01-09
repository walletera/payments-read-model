Feature: list payments

  Background: the payments-read-model is up and running
    Given a running payments-read-model
    And a list of payment created events is published and processed successfully by the payments-read-model:
    """
    data/payment_created_events_list.json
    """

  Scenario Outline: a list of payments is successfully retrieved based on the provided filters
    When the payments-read-model receives a GET request on endpoint /payments with filters <filters>
    Then the returned payments ids match <expectedPaymentIds>

    Examples:
      | filters                     | expectedPaymentIds                                                                                                      |
      | ?status=confirmed           | ["0ae1733e-7538-4908-b90a-5721670cb000","0ae1733e-7538-4908-b90a-5721670cb001", "0ae1733e-7538-4908-b90a-5721670cb002"] |
      | ?status=rejected&amount=101 | ["0ae1733e-7538-4908-b90a-5721670cb004"]                                                                                |
      | ?externalId=EXTERNAL-ID-03  | ["0ae1733e-7538-4908-b90a-5721670cb003"]                                                                                |