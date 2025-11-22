// Package main demonstrates proper error handling with tornago.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Example 1: Handle Tor startup failure
	fmt.Println("Example 1: Tor startup timeout")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
		tornago.WithTorStartupTimeout(1*time.Millisecond), // Too short
	)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	_, err = tornago.StartTorDaemon(launchCfg)
	if err != nil {
		var torErr *tornago.TornagoError
		if errors.As(err, &torErr) {
			fmt.Printf("Error kind: %v\n", torErr.Kind)
			fmt.Printf("Operation: %s\n", torErr.Op)
			fmt.Printf("Message: %s\n", torErr.Msg)
		}
		fmt.Println("Expected failure - continuing with valid timeout...\n")
	}

	// Example 2: Successful startup with proper timeout
	fmt.Println("Example 2: Successful Tor startup")
	launchCfg, err = tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer torProcess.Stop()
	fmt.Println("Tor started successfully\n")

	// Example 3: Handle connection errors
	fmt.Println("Example 3: Connection to invalid address")
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientDialTimeout(5*time.Second),
		tornago.WithClientRequestTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://invalid-domain-that-does-not-exist-12345.onion",
		http.NoBody,
	)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	_, err = client.Do(req)
	if err != nil {
		var torErr *tornago.TornagoError
		if errors.As(err, &torErr) {
			fmt.Printf("Error kind: %v\n", torErr.Kind)
			fmt.Printf("Operation: %s\n", torErr.Op)
			if torErr.Kind == tornago.ErrHTTPFailed {
				fmt.Println("This is an HTTP failure error")
			}
		} else {
			fmt.Printf("Other error: %v\n", err)
		}
		fmt.Println("Expected failure - domain does not exist\n")
	}

	// Example 4: Successful request
	fmt.Println("Example 4: Successful request")
	req, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://example.com",
		http.NoBody,
	)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	fmt.Printf("Success: HTTP %d\n", resp.StatusCode)
}
