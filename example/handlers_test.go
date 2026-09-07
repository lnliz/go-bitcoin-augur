package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type mockCollector struct {
	latestEstimate *augur.FeeEstimate
	historicalErr  error
	blockTargetErr error
}

func (m *mockCollector) GetLatestFeeEstimate() *augur.FeeEstimate {
	return m.latestEstimate
}

func (m *mockCollector) GetFeeEstimateForTimestamp(timestamp int64) (*augur.FeeEstimate, error) {
	if m.historicalErr != nil {
		return nil, m.historicalErr
	}
	return m.latestEstimate, nil
}

func (m *mockCollector) GetLatestFeeEstimateForBlockTarget(numBlocks float64) (*augur.FeeEstimate, error) {
	if m.blockTargetErr != nil {
		return nil, m.blockTargetErr
	}
	return m.latestEstimate, nil
}

func TestHandleFeesNoEstimate(t *testing.T) {
	collector := &mockCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/fees", nil)
	w := httptest.NewRecorder()

	handler.handleFees(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHandleFeesWithEstimate(t *testing.T) {
	collector := &mockCollector{}
	estimate := &augur.FeeEstimate{
		Timestamp: time.Date(2025, 3, 11, 12, 0, 0, 0, time.UTC),
		Estimates: map[int]augur.BlockTarget{
			3: {
				Blocks: 3,
				Probabilities: map[float64]float64{
					0.50: 5.1234,
					0.95: 10.5678,
				},
			},
		},
	}
	collector.latestEstimate = estimate

	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/fees", nil)
	w := httptest.NewRecorder()

	handler.handleFees(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeeEstimateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.MempoolUpdateTime != "2025-03-11T12:00:00.000Z" {
		t.Errorf("unexpected mempool_update_time: %s", response.MempoolUpdateTime)
	}

	if len(response.Estimates) != 1 {
		t.Errorf("expected 1 estimate, got %d", len(response.Estimates))
	}

	target3, ok := response.Estimates["3"]
	if !ok {
		t.Fatal("missing estimate for block target 3")
	}

	if len(target3.Probabilities) != 2 {
		t.Errorf("expected 2 probabilities, got %d", len(target3.Probabilities))
	}
}

func TestHandleHistoricalFeeMissingTimestamp(t *testing.T) {
	collector := &mockCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/historical_fee", nil)
	w := httptest.NewRecorder()

	handler.handleHistoricalFee(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleHistoricalFeeInvalidTimestamp(t *testing.T) {
	collector := &mockCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/historical_fee?timestamp=notanumber", nil)
	w := httptest.NewRecorder()

	handler.handleHistoricalFee(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleFeesTargetInvalidPath(t *testing.T) {
	collector := &mockCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/fees/target/invalid", nil)
	w := httptest.NewRecorder()

	handler.handleFeesTarget(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestTransformFeeEstimate(t *testing.T) {
	estimate := &augur.FeeEstimate{
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Estimates: map[int]augur.BlockTarget{
			6: {
				Blocks: 6,
				Probabilities: map[float64]float64{
					0.05: 1.23456789,
					0.50: 5.00001,
				},
			},
		},
	}

	response := transformFeeEstimate(estimate)

	if response.MempoolUpdateTime != "2025-01-15T10:30:00.000Z" {
		t.Errorf("unexpected timestamp format: %s", response.MempoolUpdateTime)
	}

	target6 := response.Estimates["6"]
	prob05 := target6.Probabilities["0.05"]
	if prob05.FeeRate != 1.2346 {
		t.Errorf("expected fee rate 1.2346, got %f", prob05.FeeRate)
	}

	prob50 := target6.Probabilities["0.50"]
	if prob50.FeeRate != 5.0 {
		t.Errorf("expected fee rate 5.0, got %f", prob50.FeeRate)
	}
}

func TestRoundTo4Decimals(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.23456789, 1.2346},
		{1.0, 1.0},
		{0.00001, 0.0},
		{99.99999, 100.0},
		{5.55555, 5.5556},
	}

	for _, tc := range tests {
		result := roundTo4Decimals(tc.input)
		if result != tc.expected {
			t.Errorf("roundTo4Decimals(%f) = %f, expected %f", tc.input, result, tc.expected)
		}
	}
}

func TestTargetRejectsInvalidNumbers(t *testing.T) {
	h := NewHandler(&mockCollector{}, "")
	for _, target := range []string{"NaN", "+Inf", "-Inf", "-1", "0", "0.5", "3.5", "1009", "1e100"} {
		t.Run(target, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.handleFeesTarget(w, httptest.NewRequest("GET", "/fees/target/"+target, nil))
			if w.Code != http.StatusBadRequest {
				t.Errorf("got status %d", w.Code)
			}
		})
	}
}

func TestTargetAcceptsOneAndTwoBlocks(t *testing.T) {
	estimate := &augur.FeeEstimate{Estimates: map[int]augur.BlockTarget{1: {Probabilities: map[float64]float64{0.5: 1}}}}
	h := NewHandler(&mockCollector{latestEstimate: estimate}, "")
	for _, target := range []string{"1", "2"} {
		w := httptest.NewRecorder()
		h.handleFeesTarget(w, httptest.NewRequest("GET", "/fees/target/"+target, nil))
		if w.Code != http.StatusOK {
			t.Errorf("target=%s status=%d", target, w.Code)
		}
	}
}

func TestNoCustomEstimateReturnsUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	NewHandler(&mockCollector{}, "").handleFeesTarget(w, httptest.NewRequest("GET", "/fees/target/3", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d", w.Code)
	}
}

func TestHistoricalRejectsOutOfRangeTimestamps(t *testing.T) {
	for _, timestamp := range []string{"-1", "9223372036854775807"} {
		w := httptest.NewRecorder()
		NewHandler(&mockCollector{}, "").handleHistoricalFee(w, httptest.NewRequest("GET", "/historical_fee?timestamp="+timestamp, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("timestamp=%s status=%d", timestamp, w.Code)
		}
	}
}

func TestRoutesRejectOtherMethods(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(&mockCollector{}, "").RegisterRoutes(mux)
	for _, path := range []string{"/", "/fees", "/fees.json", "/fees/target/3", "/historical_fee"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("path=%s status=%d", path, w.Code)
		}
	}
}

func TestIndexUsesAndEscapesBaseURL(t *testing.T) {
	for _, base := range []string{"/augur", "https://example.com/augur", `https://example.com/</script><script>alert(1)</script>`} {
		w := httptest.NewRecorder()
		NewHandler(&mockCollector{}, base).handleIndex(w, httptest.NewRequest("GET", "/", nil))
		body := w.Body.String()
		if w.Code != http.StatusOK || strings.Contains(body, `</script><script>alert(1)</script>`) {
			t.Fatalf("unsafe or failed template: %s", body)
		}
		if !strings.Contains(body, `const BASE = "`+strings.ReplaceAll(base, "<", `\u003c`)) && base == "/augur" {
			t.Fatal("base URL missing from JavaScript")
		}
		if base == "/augur" && !strings.Contains(body, `href="/augur/fees.json"`) {
			t.Fatal("base URL missing from link")
		}
	}
}

func TestJSONEncodingErrorReturns500(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Cache-Control", "public, max-age=15")
	writeJSON(w, map[string]float64{"fee": math.NaN()})
	if w.Code != http.StatusInternalServerError || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
}

func TestProbabilityFormattingPreservesDistinctConfidences(t *testing.T) {
	estimate := &augur.FeeEstimate{Estimates: map[int]augur.BlockTarget{3: {Probabilities: map[float64]float64{0: 1, 0.5: 2, 0.95: 3, 0.951: 4, 0.954: 5, 1: 6}}}}
	probabilities := transformFeeEstimate(estimate).Estimates["3"].Probabilities
	for key, want := range map[string]float64{"0.00": 1, "0.50": 2, "0.95": 3, "0.951": 4, "0.954": 5, "1.00": 6} {
		if probabilities[key].FeeRate != want {
			t.Errorf("confidence %s: got %v want %v", key, probabilities[key], want)
		}
	}
	if len(probabilities) != 6 {
		t.Fatalf("lost confidence: %v", probabilities)
	}
}
