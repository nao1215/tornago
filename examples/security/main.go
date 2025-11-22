// Package main demonstrates security features: Tor connection verification,
// DNS leak detection, and Hidden Service client authentication.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Example 1: Verify Tor Connection
	fmt.Println("=== Example 1: Verify Tor Connection ===")
	verifyTorExample()

	// Example 2: Check for DNS Leaks
	fmt.Println("\n=== Example 2: Check for DNS Leaks ===")
	checkDNSLeakExample()

	// Example 3: Hidden Service with Client Authentication
	fmt.Println("\n=== Example 3: Hidden Service Client Authentication ===")
	hiddenServiceAuthExample()

	// Example 4: Private Key Management Best Practices
	fmt.Println("\n=== Example 4: Private Key Management ===")
	privateKeyManagementExample()
}

func verifyTorExample() {
	// Launch Tor daemon
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
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer torProcess.Stop()

	fmt.Printf("Tor started (SOCKS: %s, Control: %s)\n",
		torProcess.SocksAddr(), torProcess.ControlAddr())

	// Create client
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientDialTimeout(30*time.Second),
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

	// Verify we're actually using Tor
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Verifying Tor connection via check.torproject.org...")
	status, err := client.VerifyTorConnection(ctx)
	if err != nil {
		log.Fatalf("Verification failed: %v", err)
	}

	fmt.Printf("Connection Status: %s\n", status)
	fmt.Printf("  Using Tor: %v\n", status.IsUsingTor())
	fmt.Printf("  Exit IP: %s\n", status.ExitIP())
	fmt.Printf("  Message: %s\n", status.Message())
	fmt.Printf("  Latency: %v\n", status.Latency())

	if !status.IsUsingTor() {
		log.Printf("WARNING: Connection is NOT going through Tor!")
		log.Printf("This could indicate a configuration problem or Tor bypass.")
	} else {
		fmt.Println("✓ Verified: Traffic is routed through Tor")
	}
}

func checkDNSLeakExample() {
	// Use the same Tor setup from previous example
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer torProcess.Stop()

	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Check for DNS leaks
	ctx := context.Background()
	fmt.Println("Checking for DNS leaks...")
	leakCheck, err := client.CheckDNSLeak(ctx)
	if err != nil {
		log.Fatalf("DNS leak check failed: %v", err)
	}

	fmt.Printf("DNS Leak Check: %s\n", leakCheck)
	fmt.Printf("  Has Leak: %v\n", leakCheck.HasLeak())
	fmt.Printf("  Resolved IPs: %v\n", leakCheck.ResolvedIPs())
	fmt.Printf("  Message: %s\n", leakCheck.Message())
	fmt.Printf("  Latency: %v\n", leakCheck.Latency())

	if leakCheck.HasLeak() {
		log.Printf("WARNING: DNS leak detected!")
		log.Printf("Your DNS queries may be visible to your ISP or DNS provider.")
		log.Printf("Consider configuring system-wide DNS to use Tor.")
	} else {
		fmt.Println("✓ No DNS leak detected")
	}
}

func hiddenServiceAuthExample() {
	fmt.Println("Creating Hidden Service with client authentication...")
	fmt.Println()

	// NOTE: This is a demonstration of the API. Client authentication requires:
	// 1. Generating x25519 key pairs for authorized clients
	// 2. Sharing the public keys with the hidden service operator
	// 3. Clients using their private keys to authenticate

	fmt.Println("Step 1: Hidden Service Operator")
	fmt.Println("  Generate client authorization keys:")
	fmt.Println("    Client public key: descriptor:x25519:<base64-encoded-public-key>")
	fmt.Println()
	fmt.Println("  Create hidden service with client auth:")
	fmt.Println("    auth := tornago.HiddenServiceAuth{")
	fmt.Println("        ClientName: \"authorized-client\",")
	fmt.Println("        PublicKey:  \"descriptor:x25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",")
	fmt.Println("    }")
	fmt.Println("    listener, _ := client.ListenWithConfig(ctx, cfg, 8080)")
	fmt.Println()

	fmt.Println("Step 2: Authorized Client")
	fmt.Println("  Configure client with private key in torrc:")
	fmt.Println("    ClientOnionAuthDir /var/lib/tor/onion_auth")
	fmt.Println("    <onion-address>.auth_private:")
	fmt.Println("      <onion-address>:descriptor:x25519:<base64-private-key>")
	fmt.Println()
	fmt.Println("  Then connect using tornago client:")
	fmt.Println("    client, _ := tornago.NewClient(cfg)")
	fmt.Println("    resp, _ := client.Do(req) // to .onion address")
	fmt.Println()

	fmt.Println("Security Benefits:")
	fmt.Println("  ✓ Only authorized clients can discover the service")
	fmt.Println("  ✓ Protects against descriptor enumeration attacks")
	fmt.Println("  ✓ Provides end-to-end authentication")
	fmt.Println()

	fmt.Println("Key Generation Example (using openssl):")
	fmt.Println("  # Generate private key")
	fmt.Println("  openssl genpkey -algorithm x25519 -out private.pem")
	fmt.Println("  # Extract public key")
	fmt.Println("  openssl pkey -in private.pem -pubout -out public.pem")
	fmt.Println("  # Convert to base64 for Tor")
	fmt.Println("  openssl pkey -in public.pem -pubin -outform DER | tail -c 32 | base64")
}

func privateKeyManagementExample() {
	fmt.Println("Best Practices for Hidden Service Private Key Management:")
	fmt.Println()

	fmt.Println("1. Storage Location:")
	fmt.Println("   ✓ Store keys in a secure directory with restricted permissions")
	fmt.Println("     chmod 700 /path/to/keys")
	fmt.Println("     chmod 600 /path/to/keys/hs_ed25519_secret_key")
	fmt.Println()

	fmt.Println("2. Key Persistence (Production):")
	fmt.Println("   ✓ Use SavePrivateKey() to persist keys across restarts")
	fmt.Println("     privateKey := listener.PrivateKey()")
	fmt.Println("     err := tornago.SavePrivateKey(\"/secure/path/key.pem\", privateKey)")
	fmt.Println()
	fmt.Println("   ✓ Load existing keys on startup")
	fmt.Println("     privateKey, _ := tornago.LoadPrivateKey(\"/secure/path/key.pem\")")
	fmt.Println("     cfg := tornago.NewHiddenServiceConfig(")
	fmt.Println("         tornago.WithHiddenServicePrivateKey(privateKey),")
	fmt.Println("     )")
	fmt.Println()

	fmt.Println("3. Backup Strategy:")
	fmt.Println("   ✓ Keep encrypted backups of private keys")
	fmt.Println("   ✓ Store backups in a separate physical location")
	fmt.Println("   ✓ Test backup restoration regularly")
	fmt.Println()

	fmt.Println("4. Access Control:")
	fmt.Println("   ✓ Limit key access to the Tor process user only")
	fmt.Println("   ✓ Use SELinux/AppArmor for additional protection")
	fmt.Println("   ✓ Monitor key file access with audit logs")
	fmt.Println()

	fmt.Println("5. Key Rotation:")
	fmt.Println("   ⚠ Rotating hidden service keys changes the .onion address")
	fmt.Println("   ✓ Plan key rotation carefully with users")
	fmt.Println("   ✓ Keep old keys available during transition period")
	fmt.Println()

	fmt.Println("6. Development vs Production:")
	fmt.Println("   Development: Use ephemeral keys (no persistence)")
	fmt.Println("     // Keys are generated and discarded on each run")
	fmt.Println("     listener, _ := client.Listen(ctx, 80, 8080)")
	fmt.Println()
	fmt.Println("   Production: Use persistent keys (save/load)")
	fmt.Println("     // Same .onion address across restarts")
	fmt.Println("     privateKey, _ := tornago.LoadPrivateKey(keyPath)")
	fmt.Println("     cfg := tornago.NewHiddenServiceConfig(")
	fmt.Println("         tornago.WithHiddenServicePrivateKey(privateKey),")
	fmt.Println("     )")
	fmt.Println()

	fmt.Println("Example: Complete Production Setup")
	fmt.Println("  const keyPath = \"/var/lib/myapp/onion.pem\"")
	fmt.Println()
	fmt.Println("  // Try to load existing key")
	fmt.Println("  privateKey, err := tornago.LoadPrivateKey(keyPath)")
	fmt.Println("  if err != nil {")
	fmt.Println("      // First run: create and save new key")
	fmt.Println("      listener, _ := client.Listen(ctx, 80, 8080)")
	fmt.Println("      privateKey = listener.PrivateKey()")
	fmt.Println("      tornago.SavePrivateKey(keyPath, privateKey)")
	fmt.Println("      os.Chmod(keyPath, 0600)")
	fmt.Println("  } else {")
	fmt.Println("      // Use existing key")
	fmt.Println("      cfg := tornago.NewHiddenServiceConfig(")
	fmt.Println("          tornago.WithHiddenServicePrivateKey(privateKey),")
	fmt.Println("      )")
	fmt.Println("      listener, _ := client.ListenWithConfig(ctx, cfg, 8080)")
	fmt.Println("  }")
	fmt.Println("  fmt.Printf(\"Service: http://\" + listener.Addr().String() + \"\\n\")")
}
