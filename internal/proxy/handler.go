package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/adamdrake/go_proxy/internal/capture"
	"github.com/google/uuid"
)

// Handler handles incoming proxy requests
type Handler struct {
	store          *capture.Store
	httpClient     *http.Client
	maxRequestSize int64
}

// NewHandler creates a new request handler
func NewHandler(store *capture.Store, maxRequestSize int64) *Handler {
	// Create an HTTP client that doesn't follow redirects
	// (we want to capture and forward them as-is)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &Handler{
		store:          store,
		httpClient:     client,
		maxRequestSize: maxRequestSize,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CONNECT method for HTTPS tunneling
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r)
		return
	}

	// Handle regular HTTP requests
	h.handleHTTP(w, r)
}

// handleHTTP forwards regular HTTP requests
func (h *Handler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Create captured request record
	captured := capture.NewCapturedRequest()
	captured.ID = uuid.New().String()
	captured.Method = r.Method
	captured.Host = r.Host
	captured.Proto = r.Proto
	captured.IsHTTPS = false
	captured.IsTunnel = false
	captured.ClientAddr = r.RemoteAddr

	// Build the target URL
	targetURL := h.buildTargetURL(r)
	captured.URL = targetURL
	captured.Path = r.URL.Path

	// Copy request headers
	captured.RequestHeaders = cloneHeaders(r.Header)

	// Read request body if present
	var requestBody []byte
	if r.Body != nil && r.ContentLength > 0 {
		requestBody, _ = io.ReadAll(io.LimitReader(r.Body, h.maxRequestSize))
		captured.RequestBody = requestBody
	}

	// Create the outgoing request
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, strings.NewReader(string(requestBody)))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Copy headers to outgoing request
	copyHeaders(outReq.Header, r.Header)

	// Remove hop-by-hop headers
	removeHopByHopHeaders(outReq.Header)

	// Forward the request
	resp, err := h.httpClient.Do(outReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Capture response
	captured.StatusCode = resp.StatusCode
	captured.ResponseHeaders = cloneHeaders(resp.Header)

	// Read response body
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, h.maxRequestSize))
	if err != nil {
		log.Printf("Error reading response: %v", err)
	}
	captured.ResponseBody = responseBody

	// Calculate duration
	captured.Duration = time.Since(startTime)

	// Store the captured request
	h.store.Add(captured)

	// Log the request
	log.Printf("[HTTP] %s %s -> %d (%s)", r.Method, targetURL, resp.StatusCode, captured.Duration)

	// Copy response headers to client
	copyHeaders(w.Header(), resp.Header)
	removeHopByHopHeaders(w.Header())

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Write response body
	w.Write(responseBody)
}

// buildTargetURL constructs the target URL from the request
func (h *Handler) buildTargetURL(r *http.Request) string {
	// If it's an absolute URL (proxy request), use it directly
	if r.URL.IsAbs() {
		return r.URL.String()
	}

	// Otherwise, construct from Host header
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	u := &url.URL{
		Scheme:   scheme,
		Host:     r.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	return u.String()
}

// cloneHeaders creates a copy of headers
func cloneHeaders(h http.Header) map[string][]string {
	result := make(map[string][]string)
	for key, values := range h {
		result[key] = append([]string{}, values...)
	}
	return result
}

// copyHeaders copies headers from src to dst
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// Hop-by-hop headers that should not be forwarded
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// removeHopByHopHeaders removes hop-by-hop headers
func removeHopByHopHeaders(h http.Header) {
	for _, header := range hopByHopHeaders {
		h.Del(header)
	}
}
