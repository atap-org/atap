// Example: Create and respond to an approval.
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
		atap.WithDID("did:web:localhost%3A8080:agent:requester"),
		atap.WithPrivateKey("<base64-ed25519-seed>"),
		atap.WithClientSecret("atap_requester_secret"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create an approval request.
	approval, err := client.Approvals.Create(
		ctx,
		"did:web:localhost%3A8080:agent:requester",
		"did:web:localhost%3A8080:human:approver",
		atap.ApprovalSubject{
			Type:  "data_access",
			Label: "Access user profile data",
			Payload: map[string]interface{}{
				"resource": "/users/123/profile",
				"scopes":   []string{"read"},
			},
		},
		"", // no intermediary
	)
	if err != nil {
		log.Fatalf("create approval: %v", err)
	}
	fmt.Printf("Approval ID:    %s\n", approval.ID)
	fmt.Printf("State:          %s\n", approval.State)

	// List pending approvals.
	approvals, err := client.Approvals.List(ctx)
	if err != nil {
		log.Fatalf("list approvals: %v", err)
	}
	fmt.Printf("Pending count:  %d\n", len(approvals))
}
