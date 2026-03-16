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