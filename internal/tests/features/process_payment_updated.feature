Feature: process PaymentUpdated event

  Background: the payments-read-model is up and running
    Given a running payments-read-model

  Scenario: a payment in the read model is successfully updated
    Given a PaymentCreated event:
    """
    data/payment_created.json
    """
    And the event is published
    And the payments-read-model produces the following log:
    """
    payment saved
    """
    And a PaymentUpdated event:
    """
    data/payment_updated.json
    """
    When the event is published
    Then the payments-read-model produces the following log:
    """
    payment updated
    """
    And the payment in the payments-read-model has the expected new values in the updated fields