package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type FeeMetricsCollector struct {
	mempoolCollector feeEstimateProvider

	feeRateDesc *prometheus.Desc
}

func NewFeeMetricsCollector(mempoolCollector feeEstimateProvider) *FeeMetricsCollector {
	return &FeeMetricsCollector{
		mempoolCollector: mempoolCollector,
		feeRateDesc: prometheus.NewDesc(
			"augur_fee_rate_sat_vbyte",
			"Bitcoin fee rate estimate in sat/vB",
			[]string{"block_target", "confidence"},
			nil,
		),
	}
}

func (c *FeeMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.feeRateDesc
}

func (c *FeeMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	estimate := c.mempoolCollector.GetLatestFeeEstimate()
	if estimate == nil {
		return
	}

	for blocks, target := range estimate.Estimates {
		blockStr := strconv.Itoa(blocks)
		for confidence, feeRate := range target.Probabilities {
			confStr := formatProbability(confidence)
			ch <- prometheus.MustNewConstMetric(
				c.feeRateDesc,
				prometheus.GaugeValue,
				feeRate,
				blockStr,
				confStr,
			)
		}
	}
}

type HTTPMetrics struct {
	requestDuration *prometheus.SummaryVec
}

func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requestDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "augur_http_request_duration_seconds",
				Help: "HTTP request duration in seconds",
				Objectives: map[float64]float64{
					0.50: 0.05,
					0.90: 0.01,
					0.95: 0.005,
					0.99: 0.001,
				},
			},
			[]string{"path", "method", "status"},
		),
	}
	reg.MustRegister(m.requestDuration)
	return m
}

func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		// ServeMux patterns group dynamic targets and unknown routes without
		// inventing statuses or allowing arbitrary paths/methods to create series.
		path := r.Pattern
		if path == "" || sw.status == http.StatusNotFound {
			path = "unmatched"
		}
		method := r.Method
		switch method {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		default:
			method = "other"
		}
		m.requestDuration.WithLabelValues(path, method, strconv.Itoa(sw.status)).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.ResponseWriter.WriteHeader(code)
	if code >= 200 {
		w.status = code
		w.wroteHeader = true
	}
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func SetupMetricsServer(addr string, mempoolCollector feeEstimateProvider) (*http.Server, *HTTPMetrics) {
	reg := prometheus.NewRegistry()

	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(NewFeeMetricsCollector(mempoolCollector))

	httpMetrics := NewHTTPMetrics(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, httpMetrics
}
