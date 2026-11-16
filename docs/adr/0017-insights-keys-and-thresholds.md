# ADR 0017: Insights with stable keys and integer permille thresholds

* Status: Accepted
* Date: 2026-11-15

## Context and Problem Statement

Automatic analyses (anomalies, subscriptions, trends) must be dismissable and acceptable across runs without storing the insights themselves, and thresholds must not use floats.

## Decision Outcome

An insight is recomputed on every run from the transactions; its key is a hash of the kind and subject (the transaction id for anomalies), so a dismissal stored in `dismissed_insights` survives recomputation. Thresholds are the config keys `insights.anomaly_factor_permille`, `insights.subscription_tolerance_permille`, `insights.subscription_min_occurrences`, `insights.trend_permille`, `insights.trend_min_amount` as integers. Accepting a subscription creates a recurrence through the recurring service. Insights work in a single currency.

### Consequences

* **Positive:** no insight table to migrate; deterministic keys make `--dismiss/--accept` scriptable and testable.
* **Negative:** changing the key derivation resurrects dismissed insights.
