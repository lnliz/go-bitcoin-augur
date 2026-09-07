package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type BitcoinRpcClient struct {
	config BitcoinRpcConfig
	client *http.Client
}

func NewBitcoinRpcClient(cfg BitcoinRpcConfig) *BitcoinRpcClient {
	return &BitcoinRpcClient{config: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type blockchainInfoResult struct {
	Blocks               *int   `json:"blocks"`
	BestBlockHash        string `json:"bestblockhash"`
	InitialBlockDownload bool   `json:"initialblockdownload"`
}

// satoshiAmount parses Bitcoin Core's BTC amounts without floating-point rounding.
type satoshiAmount int64

func (a *satoshiAmount) UnmarshalJSON(data []byte) error {
	// Bounding the token also avoids pathological exponents in untrusted responses.
	if len(data) == 0 || len(data) > 32 || data[0] == '"' {
		return fmt.Errorf("invalid BTC amount %q", data)
	}
	if i := bytes.IndexAny(data, "eE"); i >= 0 {
		exponent, err := strconv.Atoi(string(data[i+1:]))
		if err != nil || exponent < -32 || exponent > 32 {
			return fmt.Errorf("invalid BTC exponent")
		}
	}
	amount, ok := new(big.Rat).SetString(string(data))
	if !ok || amount.Sign() < 0 {
		return fmt.Errorf("invalid BTC amount %q", data)
	}
	amount.Mul(amount, big.NewRat(100_000_000, 1))
	if !amount.IsInt() || !amount.Num().IsInt64() || amount.Num().Int64() > 21_000_000*100_000_000 {
		return fmt.Errorf("BTC amount is outside the valid satoshi range: %q", data)
	}
	*a = satoshiAmount(amount.Num().Int64())
	return nil
}

type mempoolEntry struct {
	Weight int64 `json:"weight"`
	Fees   struct {
		Base *satoshiAmount `json:"base"`
	} `json:"fees"`
}

func (c *BitcoinRpcClient) call(ctx context.Context, method string, params []any, result any) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: "1.0", ID: method, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.config.Username, c.config.Password)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP status %d", method, resp.StatusCode)
	}
	var envelope struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: unexpected trailing response data", method)
	}
	if envelope.ID != method {
		return fmt.Errorf("%s: unexpected response ID %q", method, envelope.ID)
	}
	if len(envelope.Error) > 0 && !bytes.Equal(envelope.Error, []byte("null")) {
		return fmt.Errorf("%s RPC error: %s", method, envelope.Error)
	}
	if len(envelope.Result) == 0 || bytes.Equal(envelope.Result, []byte("null")) {
		return fmt.Errorf("%s: missing result", method)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}

func (c *BitcoinRpcClient) getTip(ctx context.Context) (blockchainInfoResult, error) {
	var tip blockchainInfoResult
	if err := c.call(ctx, "getblockchaininfo", []any{}, &tip); err != nil {
		return tip, err
	}
	if tip.Blocks == nil || *tip.Blocks < 0 || tip.BestBlockHash == "" {
		return tip, fmt.Errorf("incomplete blockchain info")
	}
	if tip.InitialBlockDownload {
		return tip, fmt.Errorf("Bitcoin node is still synchronizing")
	}
	return tip, nil
}

type mempoolObservation struct {
	BlockHeight  int
	BlockHash    string
	Transactions []augur.MempoolTransaction
}

// GetMempool brackets the mempool read with tip checks. A JSON-RPC batch is not
// an atomic snapshot; height alone does not detect a same-height reorganization.
func (c *BitcoinRpcClient) GetMempool(ctx context.Context) (mempoolObservation, error) {
	before, err := c.getTip(ctx)
	if err != nil {
		return mempoolObservation{}, err
	}
	var entries map[string]mempoolEntry
	if err := c.call(ctx, "getrawmempool", []any{true}, &entries); err != nil {
		return mempoolObservation{}, err
	}
	after, err := c.getTip(ctx)
	if err != nil {
		return mempoolObservation{}, err
	}
	if before.BestBlockHash != after.BestBlockHash || *before.Blocks != *after.Blocks {
		return mempoolObservation{}, fmt.Errorf("chain tip changed during mempool collection; retry on next poll")
	}
	transactions := make([]augur.MempoolTransaction, 0, len(entries))
	for txid, entry := range entries {
		if entry.Weight <= 0 || entry.Weight > 4_000_000 || entry.Fees.Base == nil {
			return mempoolObservation{}, fmt.Errorf("invalid mempool transaction %s", txid)
		}
		transactions = append(transactions, augur.MempoolTransaction{Weight: entry.Weight, Fee: int64(*entry.Fees.Base)})
	}
	return mempoolObservation{BlockHeight: *before.Blocks, BlockHash: before.BestBlockHash, Transactions: transactions}, nil
}
