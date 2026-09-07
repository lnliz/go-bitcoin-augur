package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

func newRPCServer(t *testing.T, result func(rpcRequest) any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "user" || password != "pass" {
			t.Error("incorrect RPC authentication")
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Method != http.MethodPost {
			t.Error("incorrect RPC request")
		}
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": request.ID, "result": result(request), "error": nil})
	}))
	t.Cleanup(server.Close)
	return server
}

func rpcClient(server *httptest.Server) *BitcoinRpcClient {
	return NewBitcoinRpcClient(BitcoinRpcConfig{URL: server.URL, Username: "user", Password: "pass"})
}

func TestBitcoinRpcClientParseResponse(t *testing.T) {
	server := newRPCServer(t, func(request rpcRequest) any {
		switch request.Method {
		case "getblockchaininfo":
			return map[string]any{"blocks": 850000, "bestblockhash": "tip"}
		case "getrawmempool":
			return json.RawMessage(`{"tx1":{"weight":1000,"fees":{"base":0.00001}},"tx2":{"weight":2000,"fees":{"base":0.00005}},"tx3":{"weight":400,"fees":{"base":1e-8}}}`)
		default:
			t.Errorf("unexpected method %s", request.Method)
			return nil
		}
	})
	observation, err := rpcClient(server).GetMempool(context.Background())
	height, txs := observation.BlockHeight, observation.Transactions
	if observation.BlockHash != "tip" {
		t.Errorf("lost tip identity: %q", observation.BlockHash)
	}
	if err != nil {
		t.Fatal(err)
	}
	if height != 850000 || len(txs) != 3 {
		t.Fatalf("height=%d txs=%v", height, txs)
	}
	sort.Slice(txs, func(i, j int) bool { return txs[i].Weight < txs[j].Weight })
	for i, expected := range []int64{1, 1000, 5000} {
		if txs[i].Fee != expected {
			t.Errorf("fee=%d, want %d", txs[i].Fee, expected)
		}
	}
}

func TestSatoshiAmount(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
	}{{"0", 0}, {"0.00000001", 1}, {"1e-8", 1}, {"0.00000029", 29}, {"20999999.99999999", 2099999999999999}, {"21000000", 2100000000000000}} {
		t.Run(tc.input, func(t *testing.T) {
			var amount satoshiAmount
			if err := json.Unmarshal([]byte(tc.input), &amount); err != nil || int64(amount) != tc.want {
				t.Fatalf("got %d, %v; want %d", amount, err, tc.want)
			}
		})
	}
	for _, input := range []string{`"1"`, "null", "-1", "0.000000001", "21000000.00000001", "1e1000000000"} {
		t.Run(input, func(t *testing.T) {
			var amount satoshiAmount
			if err := json.Unmarshal([]byte(input), &amount); err == nil {
				t.Fatalf("accepted %s", input)
			}
		})
	}
}

func TestBitcoinRpcClientRejectsMalformedResponses(t *testing.T) {
	for _, body := range []string{`[]`, `{}`, `null`, `{"id":"getblockchaininfo","result":null}`, `{"id":"wrong","result":{}}`, `{"id":"getblockchaininfo","error":{"code":-1,"message":"error"}}`, `{"id":"getblockchaininfo","result":{"blocks":0}}`, `{"id":"getblockchaininfo","result":{"blocks":0,"bestblockhash":"tip"}} {}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			if _, err := rpcClient(server).GetMempool(context.Background()); err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}

func TestBitcoinRpcClientRejectsChangedTip(t *testing.T) {
	for _, sameHeight := range []bool{false, true} {
		count := 0
		server := newRPCServer(t, func(request rpcRequest) any {
			if request.Method == "getrawmempool" {
				return map[string]any{}
			}
			count++
			height := 850000
			if count == 2 && !sameHeight {
				height++
			}
			return map[string]any{"blocks": height, "bestblockhash": []string{"before", "after"}[count-1]}
		})
		if _, err := rpcClient(server).GetMempool(context.Background()); err == nil {
			t.Fatal("accepted changed tip")
		}
	}
}

func TestBitcoinRpcClientRejectsInvalidTransactions(t *testing.T) {
	for _, entry := range []string{`{"weight":0,"fees":{"base":0}}`, `{"weight":4000001,"fees":{"base":0}}`, `{"weight":400}`, `{"weight":400,"fees":{"base":-1}}`, `{"weight":400,"fees":{"base":null}}`} {
		t.Run(entry, func(t *testing.T) {
			server := newRPCServer(t, func(request rpcRequest) any {
				if request.Method == "getblockchaininfo" {
					return map[string]any{"blocks": 850000, "bestblockhash": "tip"}
				}
				return map[string]any{"tx": json.RawMessage(entry)}
			})
			if _, err := rpcClient(server).GetMempool(context.Background()); err == nil {
				t.Fatal("accepted invalid transaction")
			}
		})
	}
}

func TestBitcoinRpcClientRejectsUnsyncedNode(t *testing.T) {
	server := newRPCServer(t, func(request rpcRequest) any {
		return map[string]any{"blocks": 850000, "bestblockhash": "tip", "initialblockdownload": true}
	})
	if _, err := rpcClient(server).GetMempool(context.Background()); err == nil {
		t.Fatal("accepted node in initial block download")
	}
}

func TestBitcoinRpcClientCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release }))
	defer server.Close()
	defer close(release)
	done := make(chan error, 1)
	go func() { _, err := rpcClient(server).GetMempool(ctx); done <- err }()
	<-entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RPC did not cancel")
	}
}

func TestBitcoinRpcClientHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	if _, err := rpcClient(server).GetMempool(context.Background()); err == nil {
		t.Fatal("accepted HTTP error")
	}
}
