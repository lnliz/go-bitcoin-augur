package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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

type Handler struct {
	mempoolCollector *MempoolCollector
	baseURL          string
}

func NewHandler(mempoolCollector *MempoolCollector, baseURL string) *Handler {
	return &Handler{
		mempoolCollector: mempoolCollector,
		baseURL:          baseURL,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/fees", h.handleFees)
	mux.HandleFunc("/fees.json", h.handleFeesJSON)
	mux.HandleFunc("/fees/target/", h.handleFeesTarget)
	mux.HandleFunc("/historical_fee", h.handleHistoricalFee)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = ""
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, indexHTML, baseURL, baseURL, baseURL)
}

func (h *Handler) handleFees(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for fee estimates")
	h.serveFeeEstimate(w, r, false)
}

func (h *Handler) handleFeesJSON(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for fee estimates (JSON)")
	h.serveFeeEstimate(w, r, true)
}

func (h *Handler) serveFeeEstimate(w http.ResponseWriter, r *http.Request, withCache bool) {
	estimate := h.mempoolCollector.GetLatestFeeEstimate()

	if estimate == nil {
		log.Println("No fee estimates available yet")
		http.Error(w, "No fee estimates available yet", http.StatusServiceUnavailable)
		return
	}

	log.Println("Transforming fee estimates for response")
	response := transformFeeEstimate(estimate)
	log.Printf("Returning fee estimates with %d targets", len(response.Estimates))

	if withCache {
		w.Header().Set("Cache-Control", "public, max-age=15")
	}
	writeJSON(w, response)
}

func (h *Handler) handleFeesTarget(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/fees/target/")
	numBlocks, err := strconv.ParseFloat(path, 64)
	if err != nil {
		log.Println("Invalid or missing num_blocks parameter")
		http.Error(w, "Invalid or missing number of blocks", http.StatusBadRequest)
		return
	}

	log.Printf("Received request for fee estimates targeting %.2f blocks", numBlocks)
	estimate, err := h.mempoolCollector.GetLatestFeeEstimateForBlockTarget(numBlocks)
	if err != nil {
		log.Printf("Error getting fee estimate: %v", err)
		http.Error(w, "Error calculating fee estimate", http.StatusInternalServerError)
		return
	}

	if estimate == nil {
		log.Println("No fee estimates available yet")
		http.Error(w, "No fee estimates available yet", http.StatusServiceUnavailable)
		return
	}

	log.Println("Transforming fee estimates for response")
	response := transformFeeEstimate(estimate)
	log.Printf("Returning fee estimates with %d targets", len(response.Estimates))

	writeJSON(w, response)
}

func (h *Handler) handleHistoricalFee(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request for historical fee estimates")

	timestampParam := r.URL.Query().Get("timestamp")
	if timestampParam == "" {
		http.Error(w, "timestamp parameter is required", http.StatusBadRequest)
		return
	}

	timestamp, err := strconv.ParseInt(timestampParam, 10, 64)
	if err != nil {
		log.Println("timestamp is invalid")
		http.Error(w, "Failed to parse timestamp, please input a unix timestamp", http.StatusBadRequest)
		return
	}

	log.Printf("Fetching historical fee estimate for timestamp: %d", timestamp)
	estimate, err := h.mempoolCollector.GetFeeEstimateForTimestamp(timestamp)
	if err != nil {
		log.Printf("Error getting historical fee estimate: %v", err)
		http.Error(w, "Error fetching historical fee estimate", http.StatusInternalServerError)
		return
	}

	if estimate == nil || len(estimate.Estimates) == 0 {
		log.Printf("No historical fee estimates available for %d", timestamp)
		http.Error(w, fmt.Sprintf("No historical fee estimates available for %d", timestamp), http.StatusServiceUnavailable)
		return
	}

	log.Println("Transforming historical fee estimates for response")
	response := transformFeeEstimate(estimate)
	log.Printf("Returning historical fee estimates with %d targets", len(response.Estimates))

	writeJSON(w, response)
}

func transformFeeEstimate(feeEstimate *augur.FeeEstimate) FeeEstimateResponse {
	estimates := make(map[string]BlockTargetResponse)

	for blocks, target := range feeEstimate.Estimates {
		probs := make(map[string]ProbabilityResponse)
		for prob, feeRate := range target.Probabilities {
			probKey := fmt.Sprintf("%.2f", prob)
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

func roundTo4Decimals(v float64) float64 {
	s := fmt.Sprintf("%.4f", v)
	r, _ := strconv.ParseFloat(s, 64)
	return r
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Augur – Bitcoin Fee Estimates</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    background: #0d0d0d;
    color: #e0e0e0;
    font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
    padding: 2rem;
  }
  a { color: #f7931a; }
  a:hover { color: #fdb34d; }
  h1 {
    color: #f7931a;
    font-size: 1.6rem;
    margin-bottom: 0.3rem;
  }
  .subtitle {
    color: #777;
    font-size: 0.85rem;
    margin-bottom: 1.2rem;
  }
  .api-link {
    margin-bottom: 1.5rem;
    font-size: 0.85rem;
  }
  .table-wrap {
    display: inline-block;
  }
  #status {
    font-size: 0.75rem;
    color: #555;
    margin-top: 0.8rem;
    text-align: center;
  }
  #error {
    color: #ff4444;
    font-size: 0.85rem;
    margin-bottom: 1rem;
    display: none;
  }
  table {
    border-collapse: collapse;
    width: auto;
  }
  th, td {
    padding: 0.5rem 1rem;
    text-align: right;
    border: 1px solid #2a2a2a;
  }
  th {
    background: #1a1a1a;
    color: #f7931a;
    font-weight: 600;
    position: sticky;
    top: 0;
  }
  th:first-child {
    text-align: left;
  }
  td:first-child {
    text-align: left;
    color: #999;
    font-weight: 500;
  }
  tr:hover td {
    background: #1a1406;
  }
  td.fee {
    color: #f7931a;
    font-variant-numeric: tabular-nums;
  }
  .unit {
    color: #555;
    font-size: 0.75rem;
  }
</style>
</head>
<body>
<h1>Bitcoin Fee Estimates</h1>
<p class="api-link">
source code: <br/>
<a href="https://github.com/lnliz/go-bitcoin-augur">github.com/lnliz/go-bitcoin-augur</a>

</p>
<p class="api-link">
json fee endpoint: <br/>
<a href="%s/fees.json">%s/fees.json</a>
</p>
<p id="error"></p>
<div class="table-wrap">
<table>
  <thead>
    <tr>
      <th>Blocks</th>
      <th>50%%</th>
      <th>80%%</th>
      <th>95%%</th>
    </tr>
  </thead>
  <tbody id="fees"></tbody>
</table>
<p id="status"></p>
</div>
<script>
// const BASE = %q;
const BASE = "";
const TARGETS = [3,6,9,12,18,24,36,48,72,96,144];
const PROBS = ["0.50","0.80","0.95"];

function formatFee(v) {
  return v != null ? v.toFixed(2) : "–";
}

async function load() {
  try {
    const r = await fetch(BASE + "/fees.json");
    if (!r.ok) throw new Error(r.status + " " + r.statusText);
    const d = await r.json();
    const tbody = document.getElementById("fees");
    let html = "";
    for (const t of TARGETS) {
      const e = d.estimates[String(t)];
      html += "<tr><td>" + t + " blocks</td>";
      for (const p of PROBS) {
        const fee = e && e.probabilities[p] ? e.probabilities[p].fee_rate : null;
        html += '<td class="fee">' + formatFee(fee) + ' <span class="unit">sat/vB</span></td>';
      }
      html += "</tr>";
    }
    tbody.innerHTML = html;
    document.getElementById("status").textContent = "Updated: " + d.mempool_update_time;
    document.getElementById("error").style.display = "none";
  } catch(e) {
    document.getElementById("error").textContent = "Error loading fees: " + e.message;
    document.getElementById("error").style.display = "block";
  }
}

load();
setInterval(load, 15000);
</script>
</body>
</html>
`
