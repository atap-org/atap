package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/models"
)

// rateLimitConfig holds the in-memory cached rate limit configuration.
// Protected by sync.RWMutex for concurrent read access from middleware.
type rateLimitConfig struct {
	mu        sync.RWMutex
	publicRPM int
	authRPM   int
	allowlist []*net.IPNet
}

// newRateLimitConfig creates a rateLimitConfig with default values.
// Defaults match the migration seed: public=30, auth=120, empty allowlist.
func newRateLimitConfig() *rateLimitConfig {
	return &rateLimitConfig{
		publicRPM: 30,
		authRPM:   120,
	}
}

// refresh loads configuration from the database and updates the in-memory cache.
func (rl *rateLimitConfig) refresh(ctx context.Context, store RateLimitConfigStore) error {
	kvs, err := store.GetRateLimitConfig(ctx)
	if err != nil {
		return fmt.Errorf("refresh rate limit config: %w", err)
	}

	publicRPM := 30
	authRPM := 120
	var allowlist []*net.IPNet

	if v, ok := kvs["public_rpm"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			publicRPM = n
		}
	}
	if v, ok := kvs["auth_rpm"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			authRPM = n
		}
	}
	if v, ok := kvs["ip_allowlist"]; ok {
		var cidrs []string
		if err := json.Unmarshal([]byte(v), &cidrs); err == nil {
			for _, cidr := range cidrs {
				// Try CIDR notation first, then single IP
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					ip := net.ParseIP(cidr)
					if ip != nil {
						// Single IP — wrap in /32 or /128
						if ip.To4() != nil {
							_, ipNet, _ = net.ParseCIDR(cidr + "/32")
						} else {
							_, ipNet, _ = net.ParseCIDR(cidr + "/128")
						}
					}
				}
				if ipNet != nil {
					allowlist = append(allowlist, ipNet)
				}
			}
		}
	}

	rl.mu.Lock()
	rl.publicRPM = publicRPM
	rl.authRPM = authRPM
	rl.allowlist = allowlist
	rl.mu.Unlock()
	return nil
}

// StartRateLimitConfigRefresh starts a background goroutine that refreshes
// rate limit config from the database every 60 seconds. Returns the config.
// The goroutine stops when ctx is cancelled.
func StartRateLimitConfigRefresh(ctx context.Context, store RateLimitConfigStore, log zerolog.Logger) *rateLimitConfig {
	cfg := newRateLimitConfig()
	// Initial load — log error but don't fail (defaults are safe)
	if err := cfg.refresh(ctx, store); err != nil {
		log.Error().Err(err).Msg("rate limit config: initial load failed, using defaults")
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := cfg.refresh(context.Background(), store); err != nil {
					log.Error().Err(err).Msg("rate limit config: refresh failed")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return cfg
}

// ipInAllowlist checks if the given IP is in any of the configured CIDR ranges.
func ipInAllowlist(ip string, allowlist []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range allowlist {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// RateLimitMiddleware returns a Fiber middleware that enforces IP-based rate limiting.
//
// Exempt paths: /v1/health, /.well-known/atap.json
// Tiered: public (no auth header) vs authenticated (Authorization or DPoP header present)
// Fail closed: returns 503 if Redis is unavailable
// Headers: X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After on ALL responses
func (h *Handler) RateLimitMiddleware(cfg *rateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Exempt paths — exact match only
		if path == "/v1/health" || path == "/.well-known/atap.json" {
			return c.Next()
		}

		// Normalize IP (handles IPv4-mapped IPv6 like ::ffff:1.2.3.4)
		rawIP := c.IP()
		ip := rawIP
		if parsed := net.ParseIP(rawIP); parsed != nil {
			ip = parsed.String()
		}

		// Check allowlist
		cfg.mu.RLock()
		allowed := ipInAllowlist(ip, cfg.allowlist)
		publicRPM := cfg.publicRPM
		authRPM := cfg.authRPM
		cfg.mu.RUnlock()

		if allowed {
			return c.Next()
		}

		// Determine group: if Authorization or DPoP header present, use auth bucket
		group := "public"
		limit := publicRPM
		if c.Get("Authorization") != "" || c.Get("DPoP") != "" {
			group = "auth"
			limit = authRPM
		}

		// Fixed window: minute-granularity window ID
		window := time.Now().Unix() / 60
		rateKey := fmt.Sprintf("rl:ip:%s:%s:%d", group, ip, window)

		// Redis INCR — fail closed on error
		count, err := h.redis.Incr(c.Context(), rateKey).Result()
		if err != nil {
			h.log.Error().Err(err).Str("ip", ip).Msg("rate limit: Redis unavailable")
			return c.Status(fiber.StatusServiceUnavailable).JSON(models.ProblemDetail{
				Type:     "https://atap.dev/errors/service-unavailable",
				Title:    "Service Unavailable",
				Status:   fiber.StatusServiceUnavailable,
				Detail:   "Rate limiting service unavailable. Please try again later.",
				Instance: c.Path(),
			}, mimeApplicationProblemJSON)
		}

		// Set TTL on first increment (2x window for safety)
		if count == 1 {
			h.redis.Expire(c.Context(), rateKey, 2*time.Minute)
		}

		// Compute rate limit headers
		windowEnd := (window + 1) * 60
		remaining := int64(limit) - count
		if remaining < 0 {
			remaining = 0
		}
		retryAfter := windowEnd - time.Now().Unix()
		if retryAfter < 1 {
			retryAfter = 1
		}

		c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Set("Retry-After", strconv.FormatInt(retryAfter, 10))

		// Check if over limit
		if count > int64(limit) {
			h.log.Warn().
				Str("ip", ip).
				Str("group", group).
				Int64("count", count).
				Msg("rate limit exceeded")
			return c.Status(fiber.StatusTooManyRequests).JSON(models.ProblemDetail{
				Type:     "https://atap.dev/errors/rate-limit-exceeded",
				Title:    "Rate limit exceeded",
				Status:   fiber.StatusTooManyRequests,
				Detail:   "Too many requests from this IP address. Try again later.",
				Instance: c.Path(),
			}, mimeApplicationProblemJSON)
		}

		return c.Next()
	}
}
