package main

import (
	augur "github.com/lnliz/go-bitcoin-augur"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsKeepActualStatusAndBoundRouteLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)
	mux := http.NewServeMux()
	NewHandler(&mockCollector{}, "").RegisterRoutes(mux)
	handler := metrics.Middleware(mux)
	for _, path := range []string{"/fees/target/3", "/fees/target/6", "/historical_fee", "/unknown-a", "/unknown-b"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]uint64{}
	for _, family := range families {
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			counts[labels["path"]+" "+labels["status"]] = metric.GetSummary().GetSampleCount()
		}
	}
	for key, want := range map[string]uint64{"GET /fees/target/ 503": 2, "GET /historical_fee 400": 1, "unmatched 404": 2} {
		if counts[key] != want {
			t.Errorf("%s: count=%d want=%d; got %v", key, counts[key], want, counts)
		}
	}
	if len(counts) != 3 {
		t.Fatalf("unexpected metric series: %v", counts)
	}
}

func TestStatusWriterKeepsFirstFinalStatus(t *testing.T) {
	for _, implicit := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		writer := &statusWriter{ResponseWriter: recorder, status: http.StatusOK}
		want := http.StatusCreated
		if implicit {
			writer.Write([]byte("body"))
			want = http.StatusOK
		} else {
			writer.WriteHeader(want)
		}
		writer.WriteHeader(http.StatusInternalServerError)
		if writer.status != want || recorder.Code != want {
			t.Fatalf("reported=%d actual=%d want=%d", writer.status, recorder.Code, want)
		}
	}
}

func TestFeeMetricsPreserveDistinctConfidences(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewFeeMetricsCollector(&mockCollector{latestEstimate: &augur.FeeEstimate{Timestamp: time.Now(), Estimates: map[int]augur.BlockTarget{3: {Probabilities: map[float64]float64{0.951: 4, 0.954: 5}}}}}))
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	rates := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "confidence" {
					rates[label.GetValue()] = metric.GetGauge().GetValue()
				}
			}
		}
	}
	if rates["0.951"] != 4 || rates["0.954"] != 5 || len(rates) != 2 {
		t.Fatalf("lost confidence: %v", rates)
	}
}
