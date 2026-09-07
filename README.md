# go-bitcoin-augur

A Go port of [Block's Augur](https://github.com/block/bitcoin-augur) Bitcoin fee estimation model. It combines the current mempool with observed transaction inflows to estimate fees at different time horizons and confidence levels.

- Prevents unavailable estimates from being averaged into valid fees.
- Uses long-term inflows exclusively from 144 blocks onward.
- Sorts targets before enforcing fee ordering.
- Rejects fractional block targets instead of truncating them.
- Fixes Poisson tail precision and removes the fixed search cutoff.
- Separates inflow observations across chain reorganizations.
- Requires spare block capacity before recommending a lower fee.
- Preserves small inflow changes in large snapshot weights.

The library uses only the Go standard library and performs no network or disk I/O. Go 1.26 or later is required. The separate [example server](example/) collects Bitcoin Core snapshots, persists them, and exposes HTTP endpoints and Prometheus metrics.

## Usage

```go
import (
    "time"

    augur "github.com/lnliz/go-bitcoin-augur"
)

estimator, err := augur.NewFeeEstimator()
if err != nil {
    return err
}

// Fees are integer satoshis; weights are Bitcoin weight units (WU).
snapshot, err := augur.NewMempoolSnapshotFromTransactions(txs, blockHeight, time.Now())
if err != nil {
    return err
}
history = append(history, snapshot)

estimate, err := estimator.CalculateEstimates(history)
if err != nil {
    return err
}
rate, available := estimate.GetFeeRate(3, 0.95)
// rate is sat/vB. Check available before using it.
```

Collect observations regularly, ideally covering 24 hours, with multiple snapshots between blocks. A single snapshot models the existing backlog with zero measured inflow; it cannot establish an arrival rate. Empty history returns no estimates. The result timestamp is the newest observation time, allowing callers to enforce freshness.

The standard horizons are 3, 6, 9, 12, 18, 24, 36, 48, 72, 96, and 144 ten-minute intervals; confidence levels are 5%, 20%, 50%, 80%, and 95%. The model uses a Poisson distribution for how many blocks arrive within each horizon. These are model-based estimates, not confirmation guarantees.

## Configuration and input contracts

Use `WithBlockTargets`, `WithProbabilities`, `WithShortTermWindow`, and `WithLongTermWindow` when constructing an estimator. Configuration is copied, sorted, and deduplicated; changing the supplied slices or exported default lists afterward does not affect estimators.

- Configured targets must be whole numbers from 1 through `MaxBlockTarget` (1008). `CalculateEstimatesForBlocks` accepts a specific target from 3 through 1008, or `nil` for the configured targets. Fractional targets are rejected instead of truncated.
- Probabilities must be finite and in `[0, 1]`. Zero requests no confidence and returns the modeled fee floor. One cannot provide a finite-time guarantee and has no available estimate.
- Windows must satisfy `0 < short-term window <= long-term window`; defaults are 30 minutes and 24 hours.
- Transactions require positive weights and nonnegative fees. Zero-fee transactions are omitted. Invalid transactions and bucket weight overflow return errors, including overflow when above-range buckets are combined.
- Snapshots require nonnegative heights, nonzero timestamps, and nonnegative bucket weights. Observation times must be distinct; unordered snapshots are accepted. Set the optional `BlockHash` from the observed chain tip to distinguish same-height reorganizations. Call `Validate` when reading snapshots from storage.

Estimators are safe to reuse concurrently. Input maps remain caller-owned and must not be modified during a calculation. Results are independent, caller-owned maps. `GetNearestBlockTarget` chooses the smaller target on an equal-distance tie.

The fee model uses `fee / (weight / 4)` without rounding virtual size to an integer, matching upstream. Logarithmic buckets cover approximately 0.10026–22026.47 sat/vB. Below-range buckets are excluded; above-range weight is retained in the highest bucket. An unavailable estimate is omitted from its probability map. The model does not simulate transaction dependencies, package selection, node relay policy, or miner-specific block assembly. See [the calculation notes](docs/calculations.md) for assumptions and validation.

## Run the example

```sh
cd example
BITCOIN_RPC_USERNAME=user BITCOIN_RPC_PASSWORD=pwd \
    BITCOIN_RPC_URL=http://bitcoin-node:8332 go run .
```

Open `http://127.0.0.1:8080/fees.json`. See [example/README.md](example/README.md) for configuration and endpoint behavior.

From the repository root, build and run a container:

```sh
docker build -f example/Dockerfile -t go-bitcoin-augur-example .
docker run --rm -v "$PWD/mempool_data:/mempool_data" \
    -e AUGUR_DATA_DIR=/mempool_data \
    -e BITCOIN_RPC_USERNAME=user -e BITCOIN_RPC_PASSWORD=pwd \
    -e BITCOIN_RPC_URL=http://bitcoin-node:8332 \
    -p 127.0.0.1:8080:8080 go-bitcoin-augur-example
```

## Development

The example has its own `go.mod`; testing the root module alone does not test it. CI checks both modules:

```sh
go test -race ./...
go vet ./...
(cd example && go test -race ./... && go vet ./...)
```

For numerical performance measurements, run `go test ./internal -bench . -benchmem`.

### Migration from the initial port

`NewMempoolSnapshotFromTransactions` now returns `(MempoolSnapshot, error)`. Handle its error before storing the snapshot. `FeeEstimatorOption` now applies to a private construction configuration, preventing options from mutating an existing estimator. Invalid configuration and snapshots that previously produced misleading estimates now return errors. Changing exported default slices no longer changes constructor defaults; use options instead.

Calculations intentionally correct several behaviors inherited from or differing from upstream: stable Poisson tails, explicit confidence endpoints, whole-number targets, bounded long-term weighting, preservation of unavailable projections, chronological inflow runs, exact integer inflow differences, positive spare capacity, and inclusion of the highest valid fee bucket. The Go fee floor remains approximately 0.1 sat/vB; the latest Kotlin library also offers configurable fee bounds, which this port does not expose.
