// Package main is the entry point for the Commerce standalone binary.
//
// Commerce can be run as a standalone service with embedded SQLite or
// connected to external analytics via ClickHouse.
//
// Usage:
//
//	commerce serve [address]         # Start the server
//	commerce migrate                 # Run database migrations
//	commerce admin create [email]    # Create admin user
//
// Environment Variables:
//
//	COMMERCE_DIR          Data directory (default: ./commerce_data)
//	COMMERCE_DEV          Enable development mode (default: false)
//	COMMERCE_ANALYTICS    Analytics DSN (optional)
//	COMMERCE_SECRET       Encryption/session secret
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	commerce "github.com/hanzoai/commerce"
)

func main() {
	app := commerce.New()

	// Setup graceful shutdown
	done := make(chan bool, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch
		fmt.Println("\nShutting down...")
		done <- true
	}()

	// Start the application
	go func() {
		if err := app.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			done <- true
		}
	}()

	<-done

	// Cleanup
	if err := app.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		os.Exit(1)
	}
}
