package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBitcoinRpcClientParseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}

		response := []map[string]any{
			{
				"result": map[string]any{
					"blocks": 850000,
				},
				"error": nil,
			},
			{
				"result": map[string]any{
					"tx1": map[string]any{
						"weight": 1000,
						"vsize":  250,
						"fees": map[string]any{
							"base": 0.00001,
						},
					},
					"tx2": map[string]any{
						"weight": 2000,
						"vsize":  500,
						"fees": map[string]any{
							"base": 0.00005,
						},
					},
				},
				"error": nil,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewBitcoinRpcClient(BitcoinRpcConfig{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	})

	height, txs, err := client.GetHeightAndMempoolTransactions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if height != 850000 {
		t.Errorf("expected height 850000, got %d", height)
	}

	if len(txs) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(txs))
	}
}

func TestBitcoinRpcClientHandlesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBitcoinRpcClient(BitcoinRpcConfig{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	})

	_, _, err := client.GetHeightAndMempoolTransactions()
	if err == nil {
		t.Error("expected error for failed request")
	}
}

func TestBitcoinRpcClientHandlesRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []map[string]any{
			{
				"result": nil,
				"error":  "some RPC error",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewBitcoinRpcClient(BitcoinRpcConfig{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	})

	_, _, err := client.GetHeightAndMempoolTransactions()
	if err == nil {
		t.Error("expected error for RPC error response")
	}
}

func TestBitcoinRpcClientConnectionError(t *testing.T) {
	client := NewBitcoinRpcClient(BitcoinRpcConfig{
		URL:      "http://localhost:99999",
		Username: "user",
		Password: "pass",
	})

	_, _, err := client.GetHeightAndMempoolTransactions()
	if err == nil {
		t.Error("expected error for connection failure")
	}
}
