package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/adamdrake/go_proxy/internal/capture"
)

// Server provides an HTTP API for accessing captured requests
type Server struct {
	store  *capture.Store
	server *http.Server
}

// NewServer creates a new API server
func NewServer(store *capture.Store, addr string) *Server {
	s := &Server{
		store: store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/requests", s.handleRequests)
	mux.HandleFunc("/api/requests/", s.handleRequestByID)
	mux.HandleFunc("/api/requests/stream", s.handleStream)
	mux.HandleFunc("/api/clear", s.handleClear)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // Disable for SSE
	}

	return s
}

// Start begins serving the API
func (s *Server) Start() error {
	log.Printf("API server listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleRequests returns all or recent captured requests
func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for limit parameter
	limitStr := r.URL.Query().Get("limit")
	var requests []*capture.CapturedRequest

	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		requests = s.store.GetRecent(limit)
	} else {
		requests = s.store.GetAll()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": requests,
		"count":    len(requests),
	})
}

// handleRequestByID returns a specific request by ID
func (s *Server) handleRequestByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path /api/requests/{id}
	id := r.URL.Path[len("/api/requests/"):]
	if id == "" || id == "stream" {
		http.Error(w, "Request ID required", http.StatusBadRequest)
		return
	}

	req := s.store.GetByID(id)
	if req == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// handleStream provides Server-Sent Events for real-time request updates
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to new requests
	ch := s.store.Subscribe()
	defer s.store.Unsubscribe(ch)

	// Send initial connection message
	w.Write([]byte("event: connected\ndata: {\"status\":\"connected\"}\n\n"))
	flusher.Flush()

	// Stream new requests
	for {
		select {
		case req, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(req)
			if err != nil {
				log.Printf("Error marshaling request: %v", err)
				continue
			}
			w.Write([]byte("event: request\ndata: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// handleClear clears all captured requests
func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.store.Clear()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cleared",
	})
}

// handleStats returns statistics about captured requests
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requests := s.store.GetAll()

	var httpCount, httpsCount int
	var totalDuration time.Duration

	for _, req := range requests {
		if req.IsHTTPS {
			httpsCount++
		} else {
			httpCount++
		}
		totalDuration += req.Duration
	}

	avgDuration := time.Duration(0)
	if len(requests) > 0 {
		avgDuration = totalDuration / time.Duration(len(requests))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_requests":       len(requests),
		"http_requests":        httpCount,
		"https_requests":       httpsCount,
		"average_duration_ms":  avgDuration.Milliseconds(),
	})
}

// handleHealth returns a simple health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// corsMiddleware adds CORS headers to allow browser access
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
