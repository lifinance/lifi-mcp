package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lifinance/lifi-mcp/server"
)

const version = "1.0.0"

func main() {
	var (
		keystoreName = flag.String("keystore", "", "Name of the keystore file to load")
		password     = flag.String("password", "", "Password for the keystore file")
		showVersion  = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("lifi-mcp version %s\n", version)
		return
	}

	// Create the server
	s := server.NewServer(version)

	// Load keystore if provided
	if *keystoreName != "" {
		if *password == "" {
			log.Fatal("Password is required when loading a keystore")
		}
		
		err := s.LoadKeystore(*keystoreName, *password)
		if err != nil {
			log.Fatalf("Failed to load keystore: %v", err)
		}
		
		address, err := s.GetWalletAddress()
		if err != nil {
			log.Fatalf("Failed to get wallet address: %v", err)
		}
		
		log.Printf("Loaded keystore with address: %s", address)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, exiting...")
		os.Exit(0)
	}()

	// Start the server
	log.Printf("Starting LiFi MCP Server v%s", version)
	if err := s.ServeStdio(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}