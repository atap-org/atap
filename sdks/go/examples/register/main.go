// Example: Register an agent entity and print its DID.
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
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Register a new agent (server generates keypair).
	entity, err := client.Entities.Register(context.Background(), "agent", &atap.RegisterOptions{
		Name: "my-first-agent",
	})
	if err != nil {
		log.Fatalf("register: %v", err)
	}

	fmt.Printf("Entity ID:      %s\n", entity.ID)
	fmt.Printf("DID:            %s\n", entity.DID)
	fmt.Printf("Client Secret:  %s\n", entity.ClientSecret)
	fmt.Printf("Private Key:    %s\n", entity.PrivateKey)

	// Now create an authenticated client using the returned credentials.
	authedClient, err := atap.NewClient(
		atap.WithBaseURL("http://localhost:8080"),
		atap.WithDID(entity.DID),
		atap.WithPrivateKey(entity.PrivateKey),
		atap.WithClientSecret(entity.ClientSecret),
	)
	if err != nil {
		log.Fatalf("create authed client: %v", err)
	}
	defer authedClient.Close()

	// Retrieve the entity.
	fetched, err := authedClient.Entities.Get(context.Background(), entity.ID)
	if err != nil {
		log.Fatalf("get entity: %v", err)
	}
	fmt.Printf("Fetched Name:   %s\n", fetched.Name)
}
