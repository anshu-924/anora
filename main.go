package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"grpcon/services"
)

func main() {
	// Default port
	port := ":50051"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	// Create and start server
	server, err := services.NewServer(port)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		server.Stop()
		os.Exit(0)
	}()

	// Start serving
	log.Printf("gRPC server listening on %s", port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
