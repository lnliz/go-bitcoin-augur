package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type mockCollector struct {
	latestEstimate    *augur.FeeEstimate
	historicalErr     error
	blockTargetErr    error
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

type collectorInterface interface {
	GetLatestFeeEstimate() *augur.FeeEstimate
	GetFeeEstimateForTimestamp(int64) (*augur.FeeEstimate, error)
	GetLatestFeeEstimateForBlockTarget(float64) (*augur.FeeEstimate, error)
}

func TestHandleFeesNoEstimate(t *testing.T) {
	collector := &MempoolCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/fees", nil)
	w := httptest.NewRecorder()

	handler.handleFees(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHandleFeesWithEstimate(t *testing.T) {
	collector := &MempoolCollector{}
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
	collector.latestFeeEstimate.Store(estimate)

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
	collector := &MempoolCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/historical_fee", nil)
	w := httptest.NewRecorder()

	handler.handleHistoricalFee(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleHistoricalFeeInvalidTimestamp(t *testing.T) {
	collector := &MempoolCollector{}
	handler := NewHandler(collector, "")

	req := httptest.NewRequest("GET", "/historical_fee?timestamp=notanumber", nil)
	w := httptest.NewRecorder()

	handler.handleHistoricalFee(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleFeesTargetInvalidPath(t *testing.T) {
	collector := &MempoolCollector{}
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
