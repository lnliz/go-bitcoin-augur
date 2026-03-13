package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	augur "github.com/lnliz/go-bitcoin-augur"
)

func main() {
	log.Println("Starting Augur Reference application")

	cfg := loadConfig()

	bitcoinClient := NewBitcoinRpcClient(cfg.BitcoinRpc)
	persist := NewMempoolPersistence(cfg.Persistence)

	feeEstimator, err := augur.NewFeeEstimator()
	if err != nil {
		log.Fatalf("Error creating fee estimator: %v", err)
	}

	mempoolCollector := NewMempoolCollector(bitcoinClient, persist, feeEstimator)

	handler := NewHandler(mempoolCollector, cfg.BaseURL)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting HTTP server on %s", addr)
		log.Printf("HTTP server started at http://%s/", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	mempoolCollector.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down application")
	mempoolCollector.Stop()
	if err := server.Close(); err != nil {
		log.Printf("Error closing HTTP server: %v", err)
	}
	log.Println("Application shutdown completed")
}
