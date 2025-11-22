// Package main demonstrates Tor circuit rotation using NEWNYM signal.
// Circuit rotation is useful when you want to change your exit node IP address.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Launch Tor daemon with ControlPort access
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor daemon: %v", err)
	}
	defer torProcess.Stop()

	fmt.Printf("Tor daemon started (SOCKS: %s, Control: %s)\n",
		torProcess.SocksAddr(), torProcess.ControlAddr())

	// Get control authentication
	auth, _, err := tornago.ControlAuthFromTor(torProcess.ControlAddr(), 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get control auth: %v", err)
	}

	// Create client
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientRequestTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create ControlClient for circuit management
	controlClient, err := tornago.NewControlClient(
		torProcess.ControlAddr(),
		auth,
		30*time.Second,
	)
	if err != nil {
		log.Fatalf("Failed to create control client: %v", err)
	}
	defer controlClient.Close()

	if err := controlClient.Authenticate(); err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}

	// Check IP address before rotation
	ip1, err := getCurrentIP(client)
	if err != nil {
		log.Fatalf("Failed to get IP: %v", err)
	}
	fmt.Printf("Current exit IP: %s\n", ip1)

	// Rotate circuit using NEWNYM
	fmt.Println("\nRotating circuit (NEWNYM)...")
	if err := controlClient.NewIdentity(context.Background()); err != nil {
		log.Fatalf("Failed to rotate circuit: %v", err)
	}

	// Wait for new circuit to be established
	fmt.Println("Waiting for new circuit...")
	time.Sleep(5 * time.Second)

	// Check IP address after rotation
	ip2, err := getCurrentIP(client)
	if err != nil {
		log.Fatalf("Failed to get IP: %v", err)
	}
	fmt.Printf("New exit IP: %s\n", ip2)

	if ip1 != ip2 {
		fmt.Println("\nCircuit rotation successful - IP changed")
	} else {
		fmt.Println("\nWarning: IP did not change (may need more time or manual check)")
	}
}

func getCurrentIP(client *tornago.Client) (string, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://api.ipify.org",
		http.NoBody,
	)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}
