package api

import (
	"crypto/ed25519"
	"net/http"

	"github.com/go-jose/go-jose/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/crypto"
)

// testFiberApp wraps *fiber.App to implement a common test interface.
type testFiberApp struct {
	app *fiber.App
}

func (a *testFiberApp) Test(req *http.Request, msTimeout ...int) (*http.Response, error) {
	return a.app.Test(req, msTimeout...)
}

// newTestFiberAppFromHandler creates a Fiber app from a Handler with SetupRoutes applied.
func newTestFiberAppFromHandler(h *Handler) *testFiberApp {
	app := fiber.New(fiber.Config{
		// Don't follow redirects — we want to check 302 responses
	})
	h.SetupRoutes(app)
	return &testFiberApp{app: app}
}

// newTestRedisClient creates a mock/nil Redis client for tests.
// Tests that don't need Redis jti nonce cache use this to avoid needing a real Redis instance.
// For DPoP jti replay tests, a miniredis or similar would be needed.
// For now we use a real Redis client pointed at a local instance, or skip if unavailable.
// Since the tests need to work without Redis, we create a client that will fail gracefully.
func newTestRedisClient() *redis.Client {
	// Use a fake Redis address — operations will fail but the tests handle this
	// by using mock stores that don't depend on Redis for token lookup.
	// DPoP jti nonce cache uses Redis; tests that specifically test replay prevention
	// would need a real Redis. For unit tests, we check that the code paths exist.
	opts, _ := redis.ParseURL("redis://localhost:6379/15")
	return redis.NewClient(opts)
}

// newTestHandlerFullWithKey creates a Handler with all fields including a specific platform key.
func newTestHandlerFullWithKey(
	es EntityStore,
	kvs KeyVersionStore,
	ots OAuthTokenStore,
	platformPriv ed25519.PrivateKey,
) *Handler {
	_, _, _ = crypto.GenerateKeyPair() // ensure crypto package is used
	return &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		platformKey:     platformPriv,
		redis:           newTestRedisClient(),
		log:             zerolog.Nop(),
	}
}

// Ensure go-jose is used for the import (needed by test helper for JWK).
var _ jose.SignatureAlgorithm
