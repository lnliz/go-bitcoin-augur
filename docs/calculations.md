# Calculation model and maintenance notes

The review used Kotlin upstream commit [`6f29f6a`](https://github.com/block/bitcoin-augur/tree/6f29f6abbf5ac20a78f93d788b84eef3db7e4a6a). Numerical equivalence to the original model is preserved for ordinary inputs except where the rules below explicitly fix boundary behavior. This is not a claim that fee predictions have been calibrated against live Bitcoin data.

## Responsibilities

- `mempool.go`: public input values, metadata validation, transaction-to-snapshot construction.
- `estimator.go`: configuration validation, input ordering and history filtering, orchestration. It owns the single chronological sort and rejects duplicate observation times.
- `fee_estimate.go`: public result values and lookup helpers.
- `internal/buckets.go` and `mempool_snapshot_buckets.go`: sparse logarithmic bucketing and dense arrays ordered from highest fee to lowest.
- `internal/inflow_calculator.go`: observed positive net growth per ten-minute interval.
- `internal/poisson.go`: block-arrival probabilities and confidence quantiles.
- `internal/fee_estimates_calculator.go`: capacity projection, short/long blending, and output bounds.
- `example/`: Bitcoin RPC, collection lifecycle, persistence, HTTP presentation, and metrics. These dependencies remain outside the library module.

Public boundaries reject invalid data; internal routines assume validated inputs and chronological snapshots or increasing targets as documented. Keep network behavior and persistence out of the calculation package.

## Bucketing and observed inflow

A transaction has integer weight `w > 0` in WU and fee `f >= 0` in satoshis. Its model fee rate is `4*f/w`; the logarithmic bucket is `roundToEven(100*ln(rate))`. Transactions above bucket 1000 are folded into that bucket. Zero fees are omitted. Sparse snapshots can retain below-range buckets, but simulation excludes buckets below -230. Invalid transactions and sums that overflow `int64`, including folded above-ceiling buckets, are errors. Dense snapshot weights remain integers; inflow endpoints are subtracted before conversion to floating point so small changes in large totals are preserved.

Inflow uses the first and last observations of each **contiguous run** at the same height and optional tip hash inside the window. Positive bucket deltas are summed and divided by the total observed time, then multiplied by 600 seconds. Intervals that cross a height change are excluded because mined transactions cannot be distinguished from departures. Declines in a bucket do not contribute negative inflow. Returning to a previous height after a reorg starts a new run; it must not merge with the old run.

This estimates net growth, not all arrivals. It cannot observe arrivals that disappear between snapshots. Height-only inputs cannot identify a same-height reorg; callers can supply the optional `BlockHash` to distinguish tips. The example checks the tip hash around each mempool read and persists it, separating successive observations when a replacement tip has the same height. Older stored snapshots without a hash remain usable.

## Block arrivals and capacity

For target `t`, block arrivals are modeled by `X ~ Poisson(t)`. At confidence `p`, use the largest integer `n` for which `P(X >= n) >= p`. A zero confidence requests the minimum modeled fee without making a confirmation claim. A confidence of one is unavailable: a Poisson process always has a nonzero chance of mining no blocks in finite time.

The implementation evaluates the smaller probability tail directly. Log-domain recurrence avoids cancellation near probability one and underflow for tiny positive probabilities. Quantile search expands its bracket rather than assuming all useful outcomes fit below `4*t`. Tests use independently computed high-precision reference values, including a target of 1008 and the smallest positive `float64` confidence.

Initial demand includes a half-interval short-term inflow buffer. Each projected block then receives `t/n` intervals of inflow before mining at most 4,000,000 WU from the highest fee buckets first.

The repeated simulation has an equivalent cumulative calculation. For any prefix of highest-fee buckets, let initial weight be `A`, constant per-block inflow be `B`, and block capacity be `C`. Its backlog obeys:

```
Q(0) = A
Q(k+1) = max(0, Q(k) + B - C)
Q(n) = max(0, A + n*(B - C))
```

The last identity holds because `A >= 0` and `B-C` is constant. Consequently, the first prefix without room for a new transaction is the first where initial weight plus `t` intervals of inflow reaches or exceeds `n*C`. Equality matters: an exactly full block has no room for even an infinitesimal candidate. One scan over the buckets replaces per-block allocations and loops. A deterministic randomized differential test compares this threshold to an independent reference that inserts a candidate transaction and mines block by block, including exact-capacity boundaries.

This is a divisible-weight approximation. It does not pack indivisible transactions, reserve space for a particular new transaction, or model ancestor packages, CPFP, replacement policy, block overhead, or miner selection differences. The continuous `weight/4` denominator also differs from rounded Bitcoin Core virtual size for weights not divisible by four.

## Combining projections

The long-term weight is `1 - (1 - min(t,144)/144)^2`; the remaining weight goes to the short-term projection. At and beyond 144 intervals, use the long-term projection alone. This prevents extrapolation into negative weights beyond one day.

Blend valid estimates in logarithmic bucket space. An unavailable projection with positive weight makes the blend unavailable; its sentinel must never be treated as a fee. As in upstream, fees are then made nonincreasing with increasing targets: a valid shorter-horizon estimate is also usable for a longer horizon. This means adding shorter targets can affect the reported longer-target estimates when this monotonicity correction applies.

Convert with `exp(bucket/100)` and include the highest modeled bucket. If the highest bucket has no spare capacity, the estimate is unavailable. Empty modeled demand yields the lowest bucket. Neither result should be interpreted as a guarantee of relay acceptance or confirmation.

## Verification

Regression tests cover malformed inputs, option ownership, target ordering, duplicate times, reorg runs, bucket overflow, extreme Poisson tails, probability endpoints, unavailable projections, fee bounds, concurrent estimator use, integer inflow precision, and independent candidate-admission simulation. Both modules run race-enabled tests and `go vet` in CI. Example tests use local HTTP servers and temporary directories; they do not require credentials or a Bitcoin node.
