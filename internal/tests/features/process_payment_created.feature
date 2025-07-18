Feature: process PaymentCreated event

  Background: the payments-read-model is up and running
    Given a running payments-read-model

  Scenario: a new payment is successfully added to the read model
    Given a PaymentCreated event:
    """json
    {
      "id": "d6d01cf2-628b-4742-8dc8-6b578fe1815a",
      "type": "PaymentCreated",
      "aggregateVersion": 0,
      "data": {
        "id": "0ae1733e-7538-4908-b90a-5721670cb093",
        "amount": 100,
        "currency": "USD",
        "direction": "outbound",
        "gateway": "dinopay",
        "customerId": "2432318c-4ff3-4ac0-b734-9b61779e2e46",
        "externalId": "asdfasdfasdf",
        "status": "pending",
        "debtor": {
          "institutionName": "Lemon Cash",
          "currency": "USD",
          "accountDetails": {
            "accountType": "cvu",
            "cuit": "23112223339",
            "routingInfo": {
              "cvuRoutingInfoType": "cvu",
              "cvu": "0003252627188236545234"
            }
          }
        },
        "beneficiary": {
          "institutionName": "LetsBit",
          "currency": "USD",
          "accountDetails": {
            "accountType": "cvu",
            "cuit": "23112223339",
            "routingInfo": {
              "cvuRoutingInfoType": "cvu",
              "cvu": "0004252627182736545234"
            }
          }
        },
        "createdAt": "2024-10-04T00:00:00Z",
        "updatedAt": "2024-10-04T00:00:00Z"
      }
    }
    """
    When the event is published
    Then the payments-read-model produces the following log:
    """
    payment saved
    """
    And the payment exist in the payments-read-model
