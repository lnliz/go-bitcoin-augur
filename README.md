# go-bitcoin-augur

bitcoin fee estimation library

golang port of [block/bitcoin-augur](https://github.com/block/bitcoin-augur) 

live here: [fees.coinbin.org](https://fees.coinbin.org/)

### how to use

```go
import augur "github.com/lnliz/go-bitcoin-augur"

estimator, _ := augur.NewFeeEstimator()

// collect mempool snapshots over time from your bitcoin node
snapshot := augur.NewMempoolSnapshotFromTransactions(txs, blockHeight, time.Now())
snapshots = append(snapshots, snapshot)

// estimate fees (needs multiple snapshots for accuracy)
estimate, _ := estimator.CalculateEstimates(snapshots)
feeRate, ok := estimate.GetFeeRate(3, 0.95) // 3 blocks, 95% confidence
```

### Example

See [example/](example/) for a complete example server that collects mempool snapshots and serves fee estimates via HTTP.

A pre-built Docker image is available on [Docker Hub](https://hub.docker.com/r/lnliz/go-bitcoin-augur-example).

### Run example

#### run example server

```
BITCOIN_RPC_USERNAME=user  BITCOIN_RPC_PASSWORD=pwd  BITCOIN_RPC_URL=http://bitcoin-node:8332 go run .
```

now you can see fees here: http://127.0.0.1:8080/fees.json


#### build and run example server in docker
```
docker build -f example/Dockerfile -t go-bitcoin-augur-example .

docker run -v ./mempool_data:/mempool_data \ 
    -e AUGUR_DATA_DIR=/mempool_data -e BITCOIN_RPC_USERNAME=user \
    -e BITCOIN_RPC_PASSWORD=pwd -e BITCOIN_RPC_URL=http://bitcoin-node:8332 -p 0.0.0.0:8080:8080 go-bitcoin-augur-example
```
