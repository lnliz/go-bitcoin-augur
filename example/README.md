# example server using fee estimates

A pre-built Docker image is available on [Docker Hub](https://hub.docker.com/r/lnliz/go-bitcoin-augur-example).

### Configuration

Configuration can be set via environment variables or a YAML config file (set `AUGUR_CONFIG_FILE` to the path).

| Environment Variable | YAML Key | Default | Description |
|---|---|---|---|
| `BITCOIN_RPC_URL` | `bitcoinRpc.url` | `http://localhost:8332` | Bitcoin node RPC URL |
| `BITCOIN_RPC_USERNAME` | `bitcoinRpc.username` | | RPC username |
| `BITCOIN_RPC_PASSWORD` | `bitcoinRpc.password` | | RPC password |
| `AUGUR_DATA_DIR` | `persistence.dataDirectory` | `mempool_data` | Directory for mempool snapshot storage |
| `AUGUR_SERVER_HOST` | `server.host` | `0.0.0.0` | Server listen host |
| `AUGUR_SERVER_PORT` | `server.port` | `8080` | Server listen port |
| `AUGUR_BASE_URL` | `baseUrl` | | Base URL for the server |
| `METRICS_ADDR` | `metricsAddr` | `127.0.0.1:9876` | Prometheus metrics endpoint address |
| `AUGUR_CONFIG_FILE` | | | Path to YAML config file |

### Run

#### run example server

```
BITCOIN_RPC_USERNAME=user  BITCOIN_RPC_PASSWORD=pwd  BITCOIN_RPC_URL=http://bitcoin-node:8332 go run .
```

now you can see fees here: http://127.0.0.1:8080/fees.json


#### build and run example server in docker

in repo root directory
```
docker build -f example/Dockerfile -t go-bitcoin-augur-example .

docker run -v ./mempool_data:/mempool_data \
    -e AUGUR_DATA_DIR=/mempool_data -e BITCOIN_RPC_USERNAME=user \
    -e BITCOIN_RPC_PASSWORD=pwd -e BITCOIN_RPC_URL=http://bitcoin-node:8332 -p 0.0.0.0:8080:8080 go-bitcoin-augur-example
```

### Metrics

Prometheus metrics are exposed at `http://127.0.0.1:9876/metrics` (configurable via `METRICS_ADDR`).

| Metric | Labels | Description |
|---|---|---|
| `augur_fee_rate_sat_vbyte` | `block_target`, `confidence` | Fee rate estimates in sat/vB |
| `augur_http_request_duration_seconds` | `path`, `method`, `status` | HTTP request latency (p50/p90/p95/p99) |

Standard Go runtime (`go_*`) and process (`process_*`) metrics are also included.
### API behavior

All routes accept `GET` (and `HEAD`). `/fees` and `/fees.json` return the latest
published observation; `/fees.json` permits caching for 15 seconds.
`/fees/target/{blocks}` accepts whole-number targets from 1 through 1008 and uses
the same published snapshots. `/historical_fee?timestamp={unix_seconds}` reads
stored snapshots from the preceding 24 hours; timestamps must be nonnegative
and no later than now.

The server polls every 30 seconds. Current estimates become unavailable after
two minutes without a successful observation, returning HTTP 503 and omitting
fee gauges from metrics. Historical queries also require an observation within
two minutes before the requested time. Invalid parameters return 400; storage
or calculation failures return 500. The page clears its displayed fees when a
refresh fails.

### Collection and storage

The RPC client uses exact BTC-to-satoshi conversion, validates transaction
weights and fees, and has a 30-second timeout per request. It reads the chain tip,
then the verbose mempool, then the chain tip again. Observations spanning an
observed tip change are discarded and retried on the next poll; nodes reporting
initial block download are rejected. Each snapshot retains the observed block
hash so the estimator can distinguish successive tips at the same height.

Snapshots are written to UTC date directories through an atomic rename. Existing
JSON snapshots remain readable; older files without `blockHash` use height-based
inflow grouping. Invalid JSON, bucket keys, metadata, or weights produce explicit
errors instead of silently altering estimates. Repair or remove a reported
corrupt file before that time range can be estimated again. Files are retained
for historical queries; manage disk space and archival retention externally.

Configuration overlays defaults with YAML and then environment variables.
Unknown YAML fields, malformed files, and invalid settings stop startup.
An explicitly empty environment variable clears the corresponding YAML value.
`AUGUR_BASE_URL` may be an HTTP(S) URL or a path prefix when a reverse proxy strips
that prefix before forwarding requests. Both page links and fetches use it.

### Code boundaries and validation

`bitcoin_rpc.go` owns Bitcoin Core transport and observation validation;
`persistence.go` owns snapshot files; `collector.go` owns polling, historical
queries, and atomic publication of estimates and their input snapshots.
`handlers.go` and `metrics.go` consume estimates through small interfaces.
`index.html` is embedded in the binary; `main.go` owns startup and shutdown.

From `example/`, run:

```sh
go test -race ./...
go vet ./...
go build .
```

HTTP latency labels use registered route patterns (for example,
`GET /fees/target/`) and actual response statuses. Unmatched paths and unusual
methods are grouped to keep metric cardinality bounded.
