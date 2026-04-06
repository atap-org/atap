// atap is a command-line tool for interacting with the ATAP platform.
// It hides all DPoP/OAuth/crypto complexity behind simple commands.
//
// Usage:
//
//	atap register agent --name "TravelBot"
//	atap claim create --name "TravelBot" --description "Books flights"
//	atap claim status ATAP-7X9K
//	atap inbox
//	atap whoami
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	atap "github.com/8upio/atap/sdks/go"
)

const (
	defaultServer  = "http://localhost:8080"
	configFileName = ".atap"
)

// profile stores credentials for an authenticated entity.
type profile struct {
	Server       string `json:"server"`
	DID          string `json:"did"`
	EntityID     string `json:"entity_id"`
	EntityType   string `json:"entity_type"`
	Name         string `json:"name"`
	PrivateKey   string `json:"private_key"`
	ClientSecret string `json:"client_secret,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "register":
		cmdRegister(args)
	case "claim":
		if len(args) < 1 {
			fatal("usage: atap claim <create|status>")
		}
		switch args[0] {
		case "create":
			cmdClaimCreate(args[1:])
		case "status":
			cmdClaimStatus(args[1:])
		default:
			fatal("unknown claim command: %s", args[0])
		}
	case "inbox":
		cmdInbox()
	case "whoami":
		cmdWhoami()
	case "health":
		cmdHealth(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fatal("unknown command: %s", cmd)
	}
}

func printUsage() {
	fmt.Println(`atap - ATAP command-line interface

Usage:
  atap register <agent|human|machine|org> [flags]    Register a new entity
  atap claim create [flags]                           Create a claim link for a human
  atap claim status <code>                            Check if a claim was redeemed
  atap inbox                                          Check your DIDComm inbox
  atap whoami                                         Show current identity
  atap health [server]                                Check server health

Register flags:
  --name <name>           Entity name
  --server <url>          Server URL (default: http://localhost:8080)

Claim create flags:
  --name <name>           Agent display name (default: from registration)
  --description <desc>    What the agent does
  --scopes <s1,s2>        Requested scopes (default: atap:inbox,atap:send)

Credentials are saved to ~/.atap and reused automatically.`)
}

// ============================================================
// REGISTER
// ============================================================

func cmdRegister(args []string) {
	if len(args) < 1 {
		fatal("usage: atap register <agent|human|machine|org> [--name NAME] [--server URL]")
	}

	entityType := args[0]
	name := flagValue(args, "--name")
	server := flagValue(args, "--server")
	if server == "" {
		server = defaultServer
	}

	client, err := atap.NewClient(atap.WithBaseURL(server))
	if err != nil {
		fatal("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entity, err := client.Entities.Register(ctx, entityType, &atap.RegisterOptions{Name: name})
	if err != nil {
		fatal("registration failed: %v", err)
	}

	p := profile{
		Server:       server,
		DID:          entity.DID,
		EntityID:     entity.ID,
		EntityType:   entityType,
		Name:         name,
		PrivateKey:   entity.PrivateKey,
		ClientSecret: entity.ClientSecret,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	saveProfile(p)

	fmt.Println("Registered!")
	fmt.Printf("  ID:     %s\n", entity.ID)
	fmt.Printf("  DID:    %s\n", entity.DID)
	fmt.Printf("  Type:   %s\n", entityType)
	if name != "" {
		fmt.Printf("  Name:   %s\n", name)
	}
	fmt.Printf("  Saved:  %s\n", configPath())
}

// ============================================================
// CLAIM CREATE
// ============================================================

func cmdClaimCreate(args []string) {
	p := loadProfile()
	if p.EntityType != "agent" {
		fatal("only agents can create claims (you are: %s)", p.EntityType)
	}

	name := flagValue(args, "--name")
	if name == "" {
		name = p.Name
	}
	description := flagValue(args, "--description")
	scopeStr := flagValue(args, "--scopes")
	scopes := []string{"atap:inbox", "atap:send"}
	if scopeStr != "" {
		scopes = strings.Split(scopeStr, ",")
	}

	client := newAuthedClient(p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.Do(ctx, "POST", "/v1/claims", map[string]interface{}{
		"name":        name,
		"description": description,
		"scopes":      scopes,
	})
	if err != nil {
		fatal("failed to create claim: %v", err)
	}

	fmt.Println("Claim created!")
	fmt.Printf("  Code:     %s\n", result["code"])
	fmt.Printf("  URL:      %s\n", result["url"])
	fmt.Printf("  Expires:  %s\n", result["expires_at"])
	fmt.Println()
	fmt.Println("Share this link with the human who should claim this agent.")
}

// ============================================================
// CLAIM STATUS
// ============================================================

func cmdClaimStatus(args []string) {
	if len(args) < 1 {
		fatal("usage: atap claim status <code>")
	}
	code := strings.ToUpper(args[0])

	p := loadProfile()
	client := newAuthedClient(p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inbox, err := client.DIDComm.Inbox(ctx, 50)
	if err != nil {
		fatal("failed to check inbox: %v", err)
	}

	for _, msg := range inbox.Messages {
		if msg.MessageType == "https://atap.dev/protocols/claim/1.0/redeemed" {
			fmt.Printf("Claim redeemed!\n")
			fmt.Printf("  Payload: %s\n", msg.Payload)
			return
		}
	}
	fmt.Printf("Claim %s: no redemption yet. The human hasn't approved it.\n", code)
}

// ============================================================
// INBOX
// ============================================================

func cmdInbox() {
	p := loadProfile()
	client := newAuthedClient(p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inbox, err := client.DIDComm.Inbox(ctx, 50)
	if err != nil {
		fatal("failed to check inbox: %v", err)
	}

	if len(inbox.Messages) == 0 {
		fmt.Println("Inbox is empty.")
		return
	}

	fmt.Printf("%d message(s):\n\n", len(inbox.Messages))
	for i, msg := range inbox.Messages {
		fmt.Printf("  [%d] %s\n", i+1, msg.MessageType)
		if msg.SenderDID != "" {
			fmt.Printf("      from: %s\n", msg.SenderDID)
		}
		if msg.CreatedAt != "" {
			fmt.Printf("      time: %s\n", msg.CreatedAt)
		}
		fmt.Println()
	}
}

// ============================================================
// WHOAMI
// ============================================================

func cmdWhoami() {
	p := loadProfile()
	fmt.Printf("  ID:      %s\n", p.EntityID)
	fmt.Printf("  DID:     %s\n", p.DID)
	fmt.Printf("  Type:    %s\n", p.EntityType)
	fmt.Printf("  Name:    %s\n", p.Name)
	fmt.Printf("  Server:  %s\n", p.Server)
}

// ============================================================
// HEALTH
// ============================================================

func cmdHealth(args []string) {
	server := defaultServer
	if len(args) > 0 {
		server = args[0]
	}

	client, err := atap.NewClient(atap.WithBaseURL(server))
	if err != nil {
		fatal("client error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	disc, err := client.Discovery.Discover(ctx)
	if err != nil {
		fatal("server unreachable: %v", err)
	}

	fmt.Printf("Server:  %s\n", server)
	fmt.Printf("Domain:  %s\n", disc.Domain)
	fmt.Println("Status:  OK")
}

// ============================================================
// HELPERS
// ============================================================

func newAuthedClient(p profile) *atap.Client {
	opts := []atap.Option{
		atap.WithBaseURL(p.Server),
		atap.WithDID(p.DID),
		atap.WithPrivateKey(p.PrivateKey),
	}
	if p.ClientSecret != "" {
		opts = append(opts, atap.WithClientSecret(p.ClientSecret))
	}
	client, err := atap.NewClient(opts...)
	if err != nil {
		fatal("failed to authenticate: %v", err)
	}
	return client
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configFileName)
}

func saveProfile(p profile) {
	data, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(configPath(), data, 0600); err != nil {
		fatal("failed to save credentials: %v", err)
	}
}

func loadProfile() profile {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("not logged in. Run: atap register <type> --name <name>")
	}
	var p profile
	if err := json.Unmarshal(data, &p); err != nil {
		fatal("corrupted config at %s: %v", path, err)
	}
	return p
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
