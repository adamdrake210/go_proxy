package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adamdrake/go_proxy/internal/api"
	"github.com/adamdrake/go_proxy/internal/capture"
	"github.com/adamdrake/go_proxy/internal/proxy"
)

func main() {
	// Command line flags
	proxyAddr := flag.String("proxy", ":8080", "Proxy server listen address")
	apiAddr := flag.String("api", ":8081", "API server listen address")
	maxRequests := flag.Int("max-requests", 1000, "Maximum number of requests to store in memory")
	flag.Parse()

	// Print banner
	printBanner(*proxyAddr, *apiAddr)

	// Create the capture store
	store := capture.NewStore(*maxRequests)

	// Create and configure the proxy server
	proxyConfig := proxy.DefaultConfig()
	proxyConfig.ListenAddr = *proxyAddr
	proxyServer := proxy.NewServer(proxyConfig, store)

	// Create the API server
	apiServer := api.NewServer(store, *apiAddr)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start servers in goroutines
	errChan := make(chan error, 2)

	go func() {
		if err := proxyServer.Start(); err != nil {
			errChan <- fmt.Errorf("proxy server error: %w", err)
		}
	}()

	go func() {
		if err := apiServer.Start(); err != nil {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
	case err := <-errChan:
		log.Printf("Server error: %v", err)
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := proxyServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Proxy server shutdown error: %v", err)
	}
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	log.Println("Servers stopped")
}

func printBanner(proxyAddr, apiAddr string) {
	banner := `
 ██████╗  ██████╗     ██████╗ ██████╗  ██████╗ ██╗  ██╗██╗   ██╗
██╔════╝ ██╔═══██╗    ██╔══██╗██╔══██╗██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝
██║  ███╗██║   ██║    ██████╔╝██████╔╝██║   ██║ ╚███╔╝  ╚████╔╝ 
██║   ██║██║   ██║    ██╔═══╝ ██╔══██╗██║   ██║ ██╔██╗   ╚██╔╝  
╚██████╔╝╚██████╔╝    ██║     ██║  ██║╚██████╔╝██╔╝ ██╗   ██║   
 ╚═════╝  ╚═════╝     ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   
                                                                 
HTTP/HTTPS Proxy with Request Capture
`
	fmt.Println(banner)
	fmt.Printf("Proxy Server: %s\n", proxyAddr)
	fmt.Printf("API Server:   %s\n", apiAddr)
	fmt.Println()
	fmt.Println("Configure your system/browser proxy to:", proxyAddr)
	fmt.Println("View captured requests at: http://localhost" + apiAddr + "/api/requests")
	fmt.Println("Stream requests in real-time: http://localhost" + apiAddr + "/api/requests/stream")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("─────────────────────────────────────────────────────────────────")
}
