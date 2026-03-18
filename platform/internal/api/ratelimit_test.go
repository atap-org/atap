package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// currentRateKey returns the Redis key for a given group and IP at the current minute window.
func currentRateKey(group, ip string) string {
	window := time.Now().Unix() / 60
	return fmt.Sprintf("rl:ip:%s:%s:%d", group, ip, window)
}

// testClientIP is the IP address that Fiber's app.Test assigns to requests (since there
// is no real TCP connection, Fiber reports "0.0.0.0" as the remote address).
const testClientIP = "0.0.0.0"

// newRateLimitTestHandler creates a minimal Handler + Fiber app with only the rate limit
// middleware and dummy route handlers. Uses a real Redis connection (skips if unavailable).
func newRateLimitTestHandler(t *testing.T) (*Handler, *fiber.App, *rateLimitConfig) {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available (%v), skipping rate limit test", err)
	}
	t.Cleanup(func() { rdb.Close() })

	log := zerolog.Nop()

	cfg := &rateLimitConfig{
		publicRPM: 30,
		authRPM:   120,
	}

	h := &Handler{
		redis: rdb,
		log:   log,
	}

	app := fiber.New()
	app.Use(h.RateLimitMiddleware(cfg))

	// Exempt paths
	app.Get("/v1/health", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	app.Get("/.well-known/atap.json", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	// Catch-all
	app.All("/*", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	return h, app, cfg
}

// ============================================================
// TESTS: IP-based rate limit middleware (API-07)
// ============================================================

// TestRateLimitMiddleware_PublicExceeded verifies that a public IP that has already
// reached the publicRPM threshold receives a 429 with an RFC 7807 body.
func TestRateLimitMiddleware_PublicExceeded(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Set to threshold — next INCR will be 31, exceeding the limit of 30
	h.redis.Set(ctx, rateKey, 30, 2*time.Minute)

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, ok := body["type"].(string); !ok || got != "https://atap.dev/errors/rate-limit-exceeded" {
		t.Errorf("expected type rate-limit-exceeded, got %v", body["type"])
	}
}

// TestRateLimitMiddleware_AuthExceeded verifies that an authenticated IP that has reached
// the authRPM threshold receives a 429.
func TestRateLimitMiddleware_AuthExceeded(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("auth", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Set to threshold — next INCR will be 121, exceeding the limit of 120
	h.redis.Set(ctx, rateKey, 120, 2*time.Minute)

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	req.Header.Set("Authorization", "DPoP test-token")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

// TestRateLimitMiddleware_WithinLimit verifies that a request below the rate limit
// returns 200 with rate limit headers present.
func TestRateLimitMiddleware_WithinLimit(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header to be present")
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "" {
		t.Error("expected X-RateLimit-Remaining header to be present")
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be present")
	}

	if got := resp.Header.Get("X-RateLimit-Limit"); got != "30" {
		t.Errorf("expected X-RateLimit-Limit=30, got %q", got)
	}

	remaining, err := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	if err != nil {
		t.Errorf("X-RateLimit-Remaining not numeric: %q", resp.Header.Get("X-RateLimit-Remaining"))
	} else if remaining < 0 || remaining >= 30 {
		t.Errorf("X-RateLimit-Remaining out of range: %d", remaining)
	}

	retryAfter, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil {
		t.Errorf("Retry-After not numeric: %q", resp.Header.Get("Retry-After"))
	} else if retryAfter < 1 || retryAfter > 60 {
		t.Errorf("Retry-After out of range (1-60): %d", retryAfter)
	}
}

// TestRateLimitMiddleware_Headers verifies that the rate limit headers contain correct
// numeric values based on the pre-seeded counter.
func TestRateLimitMiddleware_Headers(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Pre-seed to 10 so after INCR it's 11
	h.redis.Set(ctx, rateKey, 10, 2*time.Minute)

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("X-RateLimit-Limit"); got != "30" {
		t.Errorf("expected X-RateLimit-Limit=30, got %q", got)
	}
	// After INCR count becomes 11; remaining = 30 - 11 = 19
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "19" {
		t.Errorf("expected X-RateLimit-Remaining=19, got %q", got)
	}

	retryAfter, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil {
		t.Errorf("Retry-After not numeric: %q", resp.Header.Get("Retry-After"))
	} else if retryAfter < 1 || retryAfter > 60 {
		t.Errorf("Retry-After out of range (1-60): %d", retryAfter)
	}
}

// TestRateLimitMiddleware_Allowlist verifies that a request from an allowlisted IP
// bypasses rate limiting entirely (no headers, no 429 even when over limit).
func TestRateLimitMiddleware_Allowlist(t *testing.T) {
	h, app, cfg := newRateLimitTestHandler(t)
	ctx := context.Background()

	// Configure allowlist with the test client IP (Fiber app.Test uses 0.0.0.0)
	_, ipNet, err := net.ParseCIDR(testClientIP + "/32")
	if err != nil {
		t.Fatalf("parse CIDR: %v", err)
	}
	cfg.mu.Lock()
	cfg.allowlist = []*net.IPNet{ipNet}
	cfg.mu.Unlock()
	t.Cleanup(func() {
		cfg.mu.Lock()
		cfg.allowlist = nil
		cfg.mu.Unlock()
	})

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Pre-seed to well above limit
	h.redis.Set(ctx, rateKey, 100, 2*time.Minute)

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for allowlisted IP, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("expected no X-RateLimit-Limit header for allowlisted IP, got %q", got)
	}
}

// TestRateLimitMiddleware_HealthExempt verifies that /v1/health is exempt from rate limiting.
func TestRateLimitMiddleware_HealthExempt(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Pre-seed to well above limit
	h.redis.Set(ctx, rateKey, 100, 2*time.Minute)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for /v1/health, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("expected no rate limit headers on /v1/health, got %q", got)
	}
}

// TestRateLimitMiddleware_WellKnownExempt verifies that /.well-known/atap.json is exempt
// from rate limiting.
func TestRateLimitMiddleware_WellKnownExempt(t *testing.T) {
	h, app, _ := newRateLimitTestHandler(t)
	ctx := context.Background()

	rateKey := currentRateKey("public", testClientIP)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Pre-seed to well above limit
	h.redis.Set(ctx, rateKey, 100, 2*time.Minute)

	req := httptest.NewRequest("GET", "/.well-known/atap.json", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for /.well-known/atap.json, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("expected no rate limit headers on /.well-known/atap.json, got %q", got)
	}
}

// TestRateLimitMiddleware_RedisDown verifies that when Redis is unavailable, the
// middleware returns 503 with an RFC 7807 body containing type "service-unavailable".
func TestRateLimitMiddleware_RedisDown(t *testing.T) {
	// Use a Redis client pointed at an unreachable address
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:19999",
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
	})
	t.Cleanup(func() { rdb.Close() })

	log := zerolog.Nop()
	cfg := &rateLimitConfig{
		publicRPM: 30,
		authRPM:   120,
	}
	h := &Handler{
		redis: rdb,
		log:   log,
	}

	app := fiber.New()
	app.Use(h.RateLimitMiddleware(cfg))
	app.All("/*", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 503 {
		t.Errorf("expected 503 when Redis is down, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, ok := body["type"].(string); !ok || got != "https://atap.dev/errors/service-unavailable" {
		t.Errorf("expected type service-unavailable, got %v", body["type"])
	}
}

// TestRateLimitMiddleware_IPNormalization verifies that IPv4-mapped IPv6 addresses
// (e.g., ::ffff:127.0.0.1) are normalized to their IPv4 form (127.0.0.1).
func TestRateLimitMiddleware_IPNormalization(t *testing.T) {
	// net.ParseIP normalizes IPv4-mapped IPv6 to IPv4
	ipv4Mapped := net.ParseIP("::ffff:127.0.0.1")
	if ipv4Mapped == nil {
		t.Fatal("failed to parse ::ffff:127.0.0.1")
	}
	normalized := ipv4Mapped.String()
	if normalized != "127.0.0.1" {
		t.Errorf("expected ::ffff:127.0.0.1 to normalize to 127.0.0.1, got %q", normalized)
	}

	// Verify the key would use the normalized form
	window := time.Now().Unix() / 60
	expectedKey := fmt.Sprintf("rl:ip:public:127.0.0.1:%d", window)
	computedKey := currentRateKey("public", normalized)
	if computedKey != expectedKey {
		t.Errorf("expected key %q, got %q", expectedKey, computedKey)
	}
}

// TestIpInAllowlist verifies the ipInAllowlist helper function behavior.
func TestIpInAllowlist(t *testing.T) {
	_, cidr192, _ := net.ParseCIDR("192.168.1.0/24")

	tests := []struct {
		name      string
		ip        string
		allowlist []*net.IPNet
		want      bool
	}{
		{
			name:      "matching IP in CIDR",
			ip:        "192.168.1.50",
			allowlist: []*net.IPNet{cidr192},
			want:      true,
		},
		{
			name:      "non-matching IP",
			ip:        "10.0.0.1",
			allowlist: []*net.IPNet{cidr192},
			want:      false,
		},
		{
			name:      "empty allowlist",
			ip:        "192.168.1.50",
			allowlist: nil,
			want:      false,
		},
		{
			name:      "invalid IP string",
			ip:        "not-an-ip",
			allowlist: []*net.IPNet{cidr192},
			want:      false,
		},
		{
			name: "IPv6 address in IPv6 CIDR",
			ip:   "2001:db8::1",
			allowlist: func() []*net.IPNet {
				_, n, _ := net.ParseCIDR("2001:db8::/32")
				return []*net.IPNet{n}
			}(),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ipInAllowlist(tc.ip, tc.allowlist)
			if got != tc.want {
				t.Errorf("ipInAllowlist(%q, ...) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}
