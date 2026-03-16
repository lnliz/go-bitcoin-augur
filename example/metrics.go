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
	mempoolCollector *MempoolCollector

	feeRateDesc *prometheus.Desc
}

func NewFeeMetricsCollector(mempoolCollector *MempoolCollector) *FeeMetricsCollector {
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
			confStr := strconv.FormatFloat(confidence, 'f', 2, 64)
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

		path := ""
		status := sw.status
		if r.URL.Path == "/" || r.URL.Path == "/fees" || r.URL.Path == "/fees.json" {
			path = r.URL.Path
		} else {
			status = 404
			path = ""
		}
		m.requestDuration.WithLabelValues(path, r.Method, strconv.Itoa(status)).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func SetupMetricsServer(addr string, mempoolCollector *MempoolCollector) (*http.Server, *HTTPMetrics) {
	reg := prometheus.NewRegistry()

	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(NewFeeMetricsCollector(mempoolCollector))

	httpMetrics := NewHTTPMetrics(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}, httpMetrics
}
