package capture

import (
	"encoding/json"
	"time"
)

// CapturedRequest represents a captured HTTP request and its response
type CapturedRequest struct {
	ID              string              `json:"id"`
	Timestamp       time.Time           `json:"timestamp"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	Host            string              `json:"host"`
	Path            string              `json:"path"`
	RequestHeaders  map[string][]string `json:"request_headers"`
	RequestBody     []byte              `json:"request_body,omitempty"`

	// Response (filled in after)
	StatusCode      int                 `json:"status_code"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody    []byte              `json:"response_body,omitempty"`

	// Timing
	Duration time.Duration `json:"duration_ms"`

	// Connection type
	IsHTTPS bool `json:"is_https"`

	// For HTTPS CONNECT tunneling, we only see metadata
	IsTunnel bool `json:"is_tunnel"`

	// Process info (for future use)
	ProcessName string `json:"process_name,omitempty"`
	ProcessID   int    `json:"process_id,omitempty"`
}

// MarshalJSON custom marshaler to handle Duration as milliseconds
func (c CapturedRequest) MarshalJSON() ([]byte, error) {
	type Alias CapturedRequest
	return json.Marshal(&struct {
		Alias
		DurationMS int64 `json:"duration_ms"`
	}{
		Alias:      Alias(c),
		DurationMS: c.Duration.Milliseconds(),
	})
}

// NewCapturedRequest creates a new CapturedRequest with a generated ID and timestamp
func NewCapturedRequest() *CapturedRequest {
	return &CapturedRequest{
		Timestamp:       time.Now(),
		RequestHeaders:  make(map[string][]string),
		ResponseHeaders: make(map[string][]string),
	}
}
