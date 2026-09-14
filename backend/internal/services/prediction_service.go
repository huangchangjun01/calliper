package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// PredictionService is an HTTP client for the Python ML prediction service.
type PredictionService struct {
	baseURL    string
	httpClient *http.Client
	db         *gorm.DB
	apiKey     string
}

// NewPredictionService creates a new PredictionService.
func NewPredictionService(baseURL, apiKey string) *PredictionService {
	// 预测接口耗时随股票数增长（每只股票三周期模型前向），
	// 使用比默认（15s）更长的超时客户端，避免批量生成时超时。
	hc := &http.Client{
		Timeout: 180 * time.Second,
	}
	return &PredictionService{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: hc,
		apiKey:     apiKey,
	}
}

// get performs an authenticated GET request against the ML service.
func (s *PredictionService) get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req)
	return s.httpClient.Do(req)
}

// postJSON performs an authenticated JSON POST request against the ML service.
func (s *PredictionService) postJSON(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(req)
	return s.httpClient.Do(req)
}

// setAuthHeader attaches the ML API key when configured.
func (s *PredictionService) setAuthHeader(req *http.Request) {
	if s.apiKey != "" {
		req.Header.Set("X-ML-API-Key", s.apiKey)
	}
}

// SetDB attaches a database handle so predictions can be persisted locally.
func (s *PredictionService) SetDB(db *gorm.DB) {
	s.db = db
}

// ──────────────────────────────────────────────────────────────
// Response types (mirrors Python ML service schemas)
// ──────────────────────────────────────────────────────────────

// FactorItem represents a single prediction factor.
type FactorItem struct {
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// PredictionResult represents a prediction from the ML service.
type PredictionResult struct {
	Symbol       string       `json:"symbol"`
	Period       string       `json:"period"`
	Direction    string       `json:"direction"`
	Confidence   float64      `json:"confidence"`
	TargetPrice  float64      `json:"target_price"`
	Factors      []FactorItem `json:"factors"`
	ModelVersion string       `json:"model_version"`
	PredictedAt  string       `json:"predicted_at"`
}

// Prediction represents a historical prediction record.
type Prediction struct {
	ID          int     `json:"id"`
	Symbol      string  `json:"symbol"`
	Period      string  `json:"period"`
	Direction   string  `json:"direction"`
	Confidence  float64 `json:"confidence"`
	TargetPrice float64 `json:"target_price"`
	PredictedAt string  `json:"predicted_at"`
	IsCorrect   *bool   `json:"is_correct,omitempty"`
}

// AccuracyReport represents prediction accuracy metrics.
type AccuracyReport struct {
	Symbol           string  `json:"symbol"`
	Accuracy7d       float64 `json:"accuracy_7d"`
	Accuracy30d      float64 `json:"accuracy_30d"`
	AccuracyTotal    float64 `json:"accuracy_total"`
	TotalPredictions int     `json:"total_predictions"`
}

// ModelStatus represents model status information.
type ModelStatus struct {
	Period      string  `json:"period"`
	Version     string  `json:"version"`
	Accuracy    float64 `json:"accuracy"`
	LastTrained string  `json:"last_trained"`
	IsHealthy   bool    `json:"is_healthy"`
	ModelType   string  `json:"model_type"`
	Framework   string  `json:"framework"`
}

// BatchRequest is the request body for batch prediction.
type BatchRequest struct {
	Symbols []string `json:"symbols"`
}

// ──────────────────────────────────────────────────────────────
// API methods
// ──────────────────────────────────────────────────────────────

// readLimitedBody reads up to maxBytes from the response body for error logging.
// This prevents reading large HTML error pages into memory.
func readLimitedBody(r io.Reader, maxBytes int64) string {
	limited := io.LimitReader(r, maxBytes)
	b, _ := io.ReadAll(limited)
	return string(b)
}

// GetPrediction fetches a single stock prediction from the ML service.
func (s *PredictionService) GetPrediction(symbol string) (*PredictionResult, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/%s", s.baseURL, symbol)

	resp, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("prediction request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prediction service returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	var result PredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode prediction response: %w", err)
	}

	normalizeResult(&result)
	return &result, nil
}

// GetBatchPredictions fetches predictions for multiple symbols.
func (s *PredictionService) GetBatchPredictions(symbols []string) ([]PredictionResult, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/batch", s.baseURL)

	body, err := json.Marshal(BatchRequest{Symbols: symbols})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	resp, err := s.postJSON(url, body)
	if err != nil {
		return nil, fmt.Errorf("batch prediction request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch prediction service returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	var results []PredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode batch prediction response: %w", err)
	}

	for i := range results {
		normalizeResult(&results[i])
	}
	return results, nil
}

// GetPredictionHistory fetches historical predictions for a symbol.
func (s *PredictionService) GetPredictionHistory(symbol string, period string, limit int) ([]Prediction, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/%s/history?period=%s&limit=%d",
		s.baseURL, symbol, period, limit)

	resp, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("prediction history request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prediction history service returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	var history []Prediction
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode prediction history: %w", err)
	}

	return history, nil
}

// TriggerPrediction triggers the daily prediction task on the ML service.
func (s *PredictionService) TriggerPrediction() error {
	url := fmt.Sprintf("%s/api/v1/predictions/run", s.baseURL)

	resp, err := s.postJSON(url, nil)
	if err != nil {
		return fmt.Errorf("trigger prediction failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("trigger prediction returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	return nil
}

// GetPredictionAccuracy fetches prediction accuracy metrics for a symbol.
func (s *PredictionService) GetPredictionAccuracy(symbol string) (*AccuracyReport, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/accuracy/%s", s.baseURL, symbol)

	resp, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("accuracy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("accuracy service returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	var report AccuracyReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to decode accuracy report: %w", err)
	}

	return &report, nil
}

// GetModelStatus fetches all model statuses from the ML service.
func (s *PredictionService) GetModelStatus() ([]ModelStatus, error) {
	url := fmt.Sprintf("%s/api/v1/models/status", s.baseURL)

	resp, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("model status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model status service returned %d: %s", resp.StatusCode, readLimitedBody(resp.Body, 1024))
	}

	var statuses []ModelStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, fmt.Errorf("failed to decode model status: %w", err)
	}

	return statuses, nil
}

// ──────────────────────────────────────────────────────────────
// Prediction generation & persistence (A 方案：ML 预测落库)
// ──────────────────────────────────────────────────────────────

// SymbolGenStatus reports the generation outcome for one requested symbol.
type SymbolGenStatus struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"` // "predicted" | "no_data"
	Count  int    `json:"count"`
}

// GenerateAndPersist fetches predictions for the given symbols from the ML
// service and writes them into the local predictions table.
// It returns a per-symbol status list: "predicted" with the number of
// persisted predictions, or "no_data" when the ML service returned no
// results for that symbol (e.g. it lacks real market data). Individual
// no_data symbols do not fail the whole call.
func (s *PredictionService) GenerateAndPersist(symbols []string) ([]SymbolGenStatus, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not configured for prediction persistence")
	}

	results, err := s.GetBatchPredictions(symbols)
	if err != nil {
		return nil, fmt.Errorf("batch prediction failed: %w", err)
	}

	// Dedupe requested symbols while preserving request order.
	unique := make([]string, 0, len(symbols))
	seen := make(map[string]struct{}, len(symbols))
	for _, sym := range symbols {
		if sym == "" {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		unique = append(unique, sym)
	}

	bySymbol := make(map[string][]PredictionResult, len(results))
	for _, r := range results {
		bySymbol[r.Symbol] = append(bySymbol[r.Symbol], r)
	}

	now := time.Now()
	statuses := make([]SymbolGenStatus, 0, len(unique))

	for _, sym := range unique {
		symbolResults := bySymbol[sym]
		if len(symbolResults) == 0 {
			statuses = append(statuses, SymbolGenStatus{Symbol: sym, Status: "no_data", Count: 0})
			continue
		}

		persisted := 0
		for _, r := range symbolResults {
			// Normalize the ML period label to the DB convention short|medium|long.
			r.Period = normalizePeriod(r.Period)
			validUntil := now.AddDate(0, 0, periodDays(r.Period))

			// Map ML direction labels to the same convention used in the DB.
			dir := normalizeDirection(r.Direction)

			pred := models.Prediction{
				StockID:      0, // resolved below
				Period:       r.Period,
				Direction:    dir,
				Confidence:   r.Confidence,
				TargetPrice:  r.TargetPrice,
				Factors:      toJSON(buildFactorMap(r.Factors)),
				ModelVersion: r.ModelVersion,
				PredictedAt:  now,
				ValidUntil:   validUntil,
				Success:      nil,
			}

			// Resolve stock id from symbol; skip unknown symbols.
			var stock models.Stock
			if err := s.db.Where("symbol = ?", r.Symbol).First(&stock).Error; err != nil {
				continue
			}
			pred.StockID = stock.ID

			if err := s.db.Create(&pred).Error; err != nil {
				continue
			}
			persisted++
		}

		status := "predicted"
		if persisted == 0 {
			// ML 返回了结果但一条都未能落库（如 symbol 不在 stocks 表），按无数据处理
			status = "no_data"
		}
		statuses = append(statuses, SymbolGenStatus{Symbol: sym, Status: status, Count: persisted})
	}

	return statuses, nil
}

// periodDays returns the validity window in days for a prediction period.
func periodDays(period string) int {
	switch period {
	case "short_term", "short":
		return 3
	case "medium_term", "medium":
		return 10
	case "long_term", "long":
		return 30
	default:
		return 3
	}
}

// normalizePeriod normalizes an ML period label to the DB convention
// short|medium|long. Empty/unknown values default to "short".
func normalizePeriod(period string) string {
	switch period {
	case "short", "short_term":
		return "short"
	case "medium", "medium_term":
		return "medium"
	case "long", "long_term":
		return "long"
	default:
		return "short"
	}
}

// normalizeResult rewrites ML response fields to the API/DB convention before
// they are passed through to callers: direction becomes up|down|flat and period
// becomes short|medium|long. target_price is sanitized so a negative value
// (which the ML service returns as a 涨跌幅 decimal) is not surfaced as a price;
// non-positive values are zeroed rather than fabricating a positive price.
func normalizeResult(r *PredictionResult) {
	r.Direction = normalizeDirection(r.Direction)
	r.Period = normalizePeriod(r.Period)
	if r.TargetPrice <= 0 {
		r.TargetPrice = 0
	}
}

func normalizeDirection(dir string) string {
	// 统一为前端与评估模块约定的方向值：up / down / flat
	switch dir {
	case "上涨", "看涨", "bullish", "up", "buy":
		return "up"
	case "下跌", "看跌", "bearish", "down", "sell":
		return "down"
	case "震荡", "震荡趋势", "neutral", "flat", "hold":
		return "flat"
	default:
		return "flat"
	}
}

func buildFactorMap(factors []FactorItem) map[string]interface{} {
	m := make(map[string]interface{}, len(factors))
	for _, f := range factors {
		m[f.Name] = f.Value
	}
	return m
}

func toJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
