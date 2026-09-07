package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	persist, err := NewMempoolPersistence(cfg.Persistence)
	if err != nil {
		return err
	}
	estimator, err := augur.NewFeeEstimator()
	if err != nil {
		return err
	}
	collector := NewMempoolCollector(NewBitcoinRpcClient(cfg.BitcoinRpc), persist, estimator)
	metricsServer, metrics := SetupMetricsServer(cfg.MetricsAddr, collector)
	mux := http.NewServeMux()
	NewHandler(collector, cfg.BaseURL).RegisterRoutes(mux)
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:           metrics.Middleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	metricsListener, err := net.Listen("tcp", metricsServer.Addr)
	if err != nil {
		return err
	}
	defer metricsListener.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, 2)
	go func() { failures <- server.Serve(listener) }()
	go func() { failures <- metricsServer.Serve(metricsListener) }()
	log.Printf("Serving HTTP on %s and metrics on %s", server.Addr, metricsServer.Addr)
	collector.Start()
	select {
	case <-ctx.Done():
	case err = <-failures:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}
	collector.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range []*http.Server{server, metricsServer} {
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			srv.Close()
			err = errors.Join(err, shutdownErr)
		}
	}
	return err
}
