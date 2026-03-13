package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type BitcoinRpcClient struct {
	config BitcoinRpcConfig
	client *http.Client
}

func NewBitcoinRpcClient(cfg BitcoinRpcConfig) *BitcoinRpcClient {
	return &BitcoinRpcClient{
		config: cfg,
		client: &http.Client{},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type blockchainInfoResult struct {
	Blocks int `json:"blocks"`
}

type blockchainInfoResponse struct {
	Result *blockchainInfoResult `json:"result"`
	Error  any                   `json:"error"`
}

type fees struct {
	Base float64 `json:"base"`
}

type mempoolEntry struct {
	Weight int  `json:"weight"`
	Vsize  int  `json:"vsize"`
	Fees   fees `json:"fees"`
}

type mempoolResponse struct {
	Result map[string]mempoolEntry `json:"result"`
	Error  any                     `json:"error"`
}

func (c *BitcoinRpcClient) GetHeightAndMempoolTransactions() (int, []augur.MempoolTransaction, error) {
	log.Println("Fetching blockchain height and mempool data")

	requests := []rpcRequest{
		{
			JSONRPC: "1.0",
			ID:      "blockchain-info",
			Method:  "getblockchaininfo",
			Params:  []any{},
		},
		{
			JSONRPC: "1.0",
			ID:      "mempool",
			Method:  "getrawmempool",
			Params:  []any{true},
		},
	}

	reqBody, err := json.Marshal(requests)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.URL, bytes.NewReader(reqBody))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(c.config.Username + ":" + c.config.Password))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("RPC request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read response: %w", err)
	}

	var results []json.RawMessage
	if err := json.Unmarshal(body, &results); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var blockchainResp blockchainInfoResponse
	if err := json.Unmarshal(results[0], &blockchainResp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal blockchain info: %w", err)
	}
	if blockchainResp.Error != nil {
		return 0, nil, fmt.Errorf("RPC error (blockchain): %v", blockchainResp.Error)
	}
	if blockchainResp.Result == nil {
		return 0, nil, fmt.Errorf("no blockchain info in response")
	}
	height := blockchainResp.Result.Blocks

	var mempoolResp mempoolResponse
	if err := json.Unmarshal(results[1], &mempoolResp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal mempool: %w", err)
	}
	if mempoolResp.Error != nil {
		return 0, nil, fmt.Errorf("RPC error (mempool): %v", mempoolResp.Error)
	}

	transactions := make([]augur.MempoolTransaction, 0, len(mempoolResp.Result))
	for _, entry := range mempoolResp.Result {
		transactions = append(transactions, augur.MempoolTransaction{
			Weight: int64(entry.Weight),
			Fee:    int64(math.Round(entry.Fees.Base * 100000000)),
		})
	}

	log.Printf("Fetched blockchain height: %d and %d mempool transactions", height, len(transactions))
	return height, transactions, nil
}
