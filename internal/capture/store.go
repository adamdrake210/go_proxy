package capture

import (
	"sync"
)

// Store provides thread-safe in-memory storage for captured requests
type Store struct {
	mu       sync.RWMutex
	requests []*CapturedRequest
	maxSize  int

	// Subscribers for real-time updates
	subMu       sync.RWMutex
	subscribers map[chan *CapturedRequest]struct{}
}

// NewStore creates a new Store with the specified maximum size
func NewStore(maxSize int) *Store {
	if maxSize <= 0 {
		maxSize = 1000 // Default to 1000 requests
	}
	return &Store{
		requests:    make([]*CapturedRequest, 0, maxSize),
		maxSize:     maxSize,
		subscribers: make(map[chan *CapturedRequest]struct{}),
	}
}

// Add stores a new captured request
func (s *Store) Add(req *CapturedRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If at capacity, remove oldest request
	if len(s.requests) >= s.maxSize {
		s.requests = s.requests[1:]
	}

	s.requests = append(s.requests, req)

	// Notify subscribers (non-blocking)
	s.notifySubscribers(req)
}

// GetAll returns all captured requests
func (s *Store) GetAll() []*CapturedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*CapturedRequest, len(s.requests))
	copy(result, s.requests)
	return result
}

// GetRecent returns the most recent n requests
func (s *Store) GetRecent(n int) []*CapturedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 || n > len(s.requests) {
		n = len(s.requests)
	}

	start := len(s.requests) - n
	result := make([]*CapturedRequest, n)
	copy(result, s.requests[start:])
	return result
}

// GetByID returns a specific request by ID
func (s *Store) GetByID(id string) *CapturedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, req := range s.requests {
		if req.ID == id {
			return req
		}
	}
	return nil
}

// Clear removes all stored requests
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = make([]*CapturedRequest, 0, s.maxSize)
}

// Count returns the number of stored requests
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.requests)
}

// Subscribe returns a channel that receives new captured requests
func (s *Store) Subscribe() chan *CapturedRequest {
	ch := make(chan *CapturedRequest, 100)

	s.subMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subMu.Unlock()

	return ch
}

// Unsubscribe removes a subscriber channel
func (s *Store) Unsubscribe(ch chan *CapturedRequest) {
	s.subMu.Lock()
	delete(s.subscribers, ch)
	s.subMu.Unlock()
	close(ch)
}

// notifySubscribers sends the request to all subscribers (non-blocking)
func (s *Store) notifySubscribers(req *CapturedRequest) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- req:
		default:
			// Channel full, skip to avoid blocking
		}
	}
}
