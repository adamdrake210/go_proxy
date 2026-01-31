package proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/adamdrake/go_proxy/internal/capture"
	"github.com/google/uuid"
)

// handleConnect handles HTTPS CONNECT tunneling
// This creates a tunnel between the client and the target server
// We can see the connection metadata but not the encrypted contents
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Create captured request record for the tunnel
	captured := capture.NewCapturedRequest()
	captured.ID = uuid.New().String()
	captured.Method = "CONNECT"
	captured.Host = r.Host
	captured.URL = "https://" + r.Host
	captured.Path = ""
	captured.Proto = r.Proto
	captured.IsHTTPS = true
	captured.IsTunnel = true
	captured.ClientAddr = r.RemoteAddr
	captured.RequestHeaders = cloneHeaders(r.Header)

	// Parse host and port
	host := r.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		// No port specified, default to 443 for HTTPS
		host = net.JoinHostPort(host, "443")
	}

	// Connect to the target server
	targetConn, err := net.DialTimeout("tcp", host, 30*time.Second)
	if err != nil {
		log.Printf("[CONNECT] Failed to connect to %s: %v", host, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		captured.StatusCode = http.StatusBadGateway
		captured.Duration = time.Since(startTime)
		h.store.Add(captured)
		return
	}
	defer targetConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("[CONNECT] Hijacking not supported")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		captured.StatusCode = http.StatusInternalServerError
		captured.Duration = time.Since(startTime)
		h.store.Add(captured)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("[CONNECT] Failed to hijack connection: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		captured.StatusCode = http.StatusInternalServerError
		captured.Duration = time.Since(startTime)
		h.store.Add(captured)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established to client
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		log.Printf("[CONNECT] Failed to send 200 response: %v", err)
		captured.StatusCode = http.StatusInternalServerError
		captured.Duration = time.Since(startTime)
		h.store.Add(captured)
		return
	}

	captured.StatusCode = http.StatusOK

	log.Printf("[CONNECT] Tunnel established to %s", r.Host)

	// Create a channel to track when piping is done
	done := make(chan struct{}, 2)

	// Pipe data between client and target (bidirectional)
	go func() {
		io.Copy(targetConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, targetConn)
		done <- struct{}{}
	}()

	// Wait for either direction to finish
	<-done

	// Calculate final duration
	captured.Duration = time.Since(startTime)
	h.store.Add(captured)

	log.Printf("[CONNECT] Tunnel closed to %s (duration: %s)", r.Host, captured.Duration)
}
