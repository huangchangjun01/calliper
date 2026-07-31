package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PredictionService is an HTTP client for the Python ML prediction service.
type PredictionService struct {
	baseURL    string
	httpClient *http.Client
}

// NewPredictionService creates a new PredictionService.
func NewPredictionService(baseURL string) *PredictionService {
	return &PredictionService{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: newPooledHTTPClient(),
	}
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

	resp, err := s.httpClient.Get(url)
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

	return &result, nil
}

// GetBatchPredictions fetches predictions for multiple symbols.
func (s *PredictionService) GetBatchPredictions(symbols []string) ([]PredictionResult, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/batch", s.baseURL)

	body, err := json.Marshal(BatchRequest{Symbols: symbols})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	resp, err := s.httpClient.Post(url, "application/json", strings.NewReader(string(body)))
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

	return results, nil
}

// GetPredictionHistory fetches historical predictions for a symbol.
func (s *PredictionService) GetPredictionHistory(symbol string, period string, limit int) ([]Prediction, error) {
	url := fmt.Sprintf("%s/api/v1/predictions/%s/history?period=%s&limit=%d",
		s.baseURL, symbol, period, limit)

	resp, err := s.httpClient.Get(url)
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

	resp, err := s.httpClient.Post(url, "application/json", nil)
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

	resp, err := s.httpClient.Get(url)
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

	resp, err := s.httpClient.Get(url)
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