package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type FeeEstimateResponse struct {
	MempoolUpdateTime string                         `json:"mempool_update_time"`
	Estimates         map[string]BlockTargetResponse `json:"estimates"`
}

type BlockTargetResponse struct {
	Probabilities map[string]ProbabilityResponse `json:"probabilities"`
}

type ProbabilityResponse struct {
	FeeRate float64 `json:"fee_rate"`
}

type feeEstimateProvider interface {
	GetLatestFeeEstimate() *augur.FeeEstimate
	GetFeeEstimateForTimestamp(int64) (*augur.FeeEstimate, error)
	GetLatestFeeEstimateForBlockTarget(float64) (*augur.FeeEstimate, error)
}

type Handler struct {
	mempoolCollector feeEstimateProvider
	baseURL          string
}

func NewHandler(mempoolCollector feeEstimateProvider, baseURL string) *Handler {
	return &Handler{
		mempoolCollector: mempoolCollector,
		baseURL:          baseURL,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.handleIndex)
	mux.HandleFunc("GET /fees", h.handleFees)
	mux.HandleFunc("GET /fees.json", h.handleFeesJSON)
	mux.HandleFunc("GET /fees/target/", h.handleFeesTarget)
	mux.HandleFunc("GET /historical_fee", h.handleHistoricalFee)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, h.baseURL); err != nil {
		log.Printf("Error rendering index: %v", err)
	}
}

func (h *Handler) handleFees(w http.ResponseWriter, r *http.Request) {
	h.serveFeeEstimate(w, r, false)
}

func (h *Handler) handleFeesJSON(w http.ResponseWriter, r *http.Request) {
	h.serveFeeEstimate(w, r, true)
}

func (h *Handler) serveFeeEstimate(w http.ResponseWriter, r *http.Request, withCache bool) {
	estimate := h.mempoolCollector.GetLatestFeeEstimate()

	if estimate == nil || len(estimate.Estimates) == 0 {
		log.Println("No fee estimates available yet")
		writeError(w, "No fee estimates available yet", http.StatusServiceUnavailable)
		return
	}

	response := transformFeeEstimate(estimate)

	if withCache {
		w.Header().Set("Cache-Control", "public, max-age=15")
	}
	writeJSON(w, response)
}

func (h *Handler) handleFeesTarget(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/fees/target/")
	numBlocks, err := strconv.ParseFloat(path, 64)
	if err != nil || math.IsNaN(numBlocks) || math.IsInf(numBlocks, 0) || numBlocks < 1 || numBlocks > augur.MaxBlockTarget || math.Trunc(numBlocks) != numBlocks {
		log.Println("Invalid or missing num_blocks parameter")
		writeError(w, fmt.Sprintf("Number of blocks must be an integer between 1 and %d", augur.MaxBlockTarget), http.StatusBadRequest)
		return
	}

	estimate, err := h.mempoolCollector.GetLatestFeeEstimateForBlockTarget(numBlocks)
	if err != nil {
		log.Printf("Error getting fee estimate: %v", err)
		writeError(w, "Error calculating fee estimate", http.StatusInternalServerError)
		return
	}

	if estimate == nil || len(estimate.Estimates) == 0 {
		log.Println("No fee estimates available yet")
		writeError(w, "No fee estimates available yet", http.StatusServiceUnavailable)
		return
	}

	response := transformFeeEstimate(estimate)
	writeJSON(w, response)
}

func (h *Handler) handleHistoricalFee(w http.ResponseWriter, r *http.Request) {
	timestampParam := r.URL.Query().Get("timestamp")
	if timestampParam == "" {
		writeError(w, "timestamp parameter is required", http.StatusBadRequest)
		return
	}

	timestamp, err := strconv.ParseInt(timestampParam, 10, 64)
	if err != nil || timestamp < 0 || timestamp > time.Now().Unix() {
		log.Println("timestamp is invalid")
		writeError(w, "timestamp must be a nonnegative Unix timestamp no later than now", http.StatusBadRequest)
		return
	}

	estimate, err := h.mempoolCollector.GetFeeEstimateForTimestamp(timestamp)
	if err != nil {
		log.Printf("Error getting historical fee estimate: %v", err)
		writeError(w, "Error fetching historical fee estimate", http.StatusInternalServerError)
		return
	}

	if estimate == nil || len(estimate.Estimates) == 0 {
		log.Printf("No historical fee estimates available for %d", timestamp)
		writeError(w, fmt.Sprintf("No historical fee estimates available for %d", timestamp), http.StatusServiceUnavailable)
		return
	}

	response := transformFeeEstimate(estimate)
	writeJSON(w, response)
}

func transformFeeEstimate(feeEstimate *augur.FeeEstimate) FeeEstimateResponse {
	estimates := make(map[string]BlockTargetResponse)

	for blocks, target := range feeEstimate.Estimates {
		probs := make(map[string]ProbabilityResponse)
		for prob, feeRate := range target.Probabilities {
			probKey := formatProbability(prob)
			probs[probKey] = ProbabilityResponse{
				FeeRate: roundTo4Decimals(feeRate),
			}
		}
		estimates[strconv.Itoa(blocks)] = BlockTargetResponse{
			Probabilities: probs,
		}
	}

	return FeeEstimateResponse{
		MempoolUpdateTime: feeEstimate.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"),
		Estimates:         estimates,
	}
}

// Preserve custom confidence levels while keeping the default API keys stable.
func formatProbability(p float64) string {
	value := strconv.FormatFloat(p, 'f', -1, 64)
	point := strings.IndexByte(value, '.')
	if point < 0 {
		return value + ".00"
	}
	if len(value)-point == 2 {
		return value + "0"
	}
	return value
}

func roundTo4Decimals(v float64) float64 { return math.Round(v*10000) / 10000 }

func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, message, status)
}

func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		writeError(w, "Error encoding fee estimate", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}

var indexTemplate = template.Must(template.New("index").Parse(indexHTML))

//go:embed index.html
var indexHTML string
