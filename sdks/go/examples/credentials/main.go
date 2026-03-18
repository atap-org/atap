// Example: Verify an email address and list credentials.
package main

import (
	"context"
	"fmt"
	"log"

	atap "github.com/8upio/atap/sdks/go"
)

func main() {
	client, err := atap.NewClient(
		atap.WithBaseURL("http://localhost:8080"),
		atap.WithDID("did:web:localhost%3A8080:human:user1"),
		atap.WithPrivateKey("<base64-ed25519-seed>"),
		atap.WithClientSecret("atap_user_secret"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Start email verification.
	msg, err := client.Credentials.StartEmailVerification(ctx, "user@example.com")
	if err != nil {
		log.Fatalf("start email verification: %v", err)
	}
	fmt.Printf("Status: %s\n", msg)

	// Verify email with OTP (in a real app, the user enters the code).
	cred, err := client.Credentials.VerifyEmail(ctx, "user@example.com", "123456")
	if err != nil {
		log.Fatalf("verify email: %v", err)
	}
	fmt.Printf("Credential ID:   %s\n", cred.ID)
	fmt.Printf("Credential Type: %s\n", cred.Type)

	// List all credentials.
	creds, err := client.Credentials.List(ctx)
	if err != nil {
		log.Fatalf("list credentials: %v", err)
	}
	fmt.Printf("Total credentials: %d\n", len(creds))

	// Check public status list.
	statusData, err := client.Credentials.StatusList(ctx, "1")
	if err != nil {
		log.Fatalf("status list: %v", err)
	}
	fmt.Printf("Status list type: %v\n", statusData["type"])
}
