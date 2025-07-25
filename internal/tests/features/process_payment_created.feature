Feature: process PaymentCreated event

  Background: the payments-read-model is up and running
    Given a running payments-read-model

  Scenario: a new payment is successfully added to the read model
    Given a PaymentCreated event:
    """
    data/payment_created.json
    """
    When the event is published
    Then the payments-read-model produces the following log:
    """
    payment saved
    """
    And the payment exist in the payments-read-model

  Scenario: a duplicate PaymentCreated event does not create a new payment record
    Given a PaymentCreated event:
    """
    data/payment_created.json
    """
    And the event is published
    And the payments-read-model produces the following log:
    """
    payment saved
    """
    When the same PaymentCreated event is published again
    Then the payments-read-model produces the following log:
    """
    failed saving payment
    """
    And only one payment with the given id exists in the payments-read-model
