Feature: process PaymentUpdated event

  Background: the payments-read-model is up and running
    Given a running payments-read-model

  Scenario: a payment in the read model is successfully updated
    Given a PaymentCreated event:
    """json
    {
      "id": "4ea4bab9-00bf-4aa5-8089-357f95c58a16",
      "type": "PaymentCreated",
      "aggregateVersion": 0,
      "data": {
        "id": "4a98475e-8316-4c07-bda4-38e1b9847d0b",
        "amount": 100,
        "currency": "USD",
        "direction": "outbound",
        "gateway": "dinopay",
        "customerId": "2432318c-4ff3-4ac0-b734-9b61779e2e46",
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
    And the event is published
    And the payments-read-model produces the following log:
    """
    payment saved
    """
    And a PaymentUpdated event:
    """json
    {
      "id": "65aec719-5a2c-4600-8511-cb6962efda21",
      "type": "PaymentUpdated",
      "aggregateVersion": 1,
      "data": {
        "paymentId": "4a98475e-8316-4c07-bda4-38e1b9847d0b",
        "externalId": "SOME-EXTERNAL-ID-1234",
        "status": "confirmed"
      }
    }
    """
    When the event is published
    Then the payments-read-model produces the following log:
    """
    payment updated
    """
    And the payment in the payments-read-model has the expected new values in the updated fields