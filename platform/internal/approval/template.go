package approval

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// IsBlockedIP returns true if the IP is in a range that should be blocked to prevent SSRF.
// Blocked ranges per spec §11.5:
//   - RFC 1918 private addresses (10/8, 172.16/12, 192.168/16)
//   - Loopback (127/8, ::1)
//   - Link-local unicast (169.254/16)
//   - Cloud metadata endpoint (169.254.169.254)
//   - IPv6 loopback (::1, covered by IsLoopback)
//
// Exported for testing.
func IsBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsLinkLocalUnicast() {
		return true
	}
	// Explicit check for cloud metadata (also covered by IsLinkLocalUnicast on most systems,
	// but belt-and-suspenders per security best practices)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

// ssrfSafeTransport builds an http.Transport with a custom DialContext that:
//  1. Resolves DNS addresses for the target host
//  2. Validates each resolved IP against IsBlockedIP
//  3. Connects to the first non-blocked IP directly (prevents DNS rebinding)
func ssrfSafeTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 5 * time.Second,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("ssrf check: split host/port: %w", err)
		}

		// Resolve DNS
		resolver := &net.Resolver{}
		addrs, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("ssrf check: DNS lookup for %q: %w", host, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("ssrf check: no addresses resolved for %q", host)
		}

		// Find first non-blocked IP
		for _, a := range addrs {
			if IsBlockedIP(a.IP) {
				continue
			}
			// Connect directly to validated IP (prevents DNS rebinding)
			return dialer.DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
		}

		return nil, fmt.Errorf("ssrf check: all resolved IPs for %q are blocked", host)
	}
	return transport
}

// FetchTemplate retrieves a template from templateURL with SSRF prevention per spec §11.5.
// Returns nil, nil if templateURL is empty (two-party approval, fallback rendering).
func FetchTemplate(ctx context.Context, templateURL string) (*models.Template, error) {
	if templateURL == "" {
		return nil, nil
	}

	// Reject non-HTTPS
	if !strings.HasPrefix(templateURL, "https://") {
		return nil, fmt.Errorf("fetch template: URL must use https scheme, got: %q", templateURL)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		// Reject all redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: ssrfSafeTransport(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, templateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch template: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch template: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMovedPermanently ||
		resp.StatusCode == http.StatusFound ||
		resp.StatusCode == http.StatusSeeOther ||
		resp.StatusCode == http.StatusTemporaryRedirect ||
		resp.StatusCode == http.StatusPermanentRedirect {
		return nil, fmt.Errorf("fetch template: redirects are not allowed (got %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch template: server returned status %d", resp.StatusCode)
	}

	// Limit body to 64KB per spec §11.5
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("fetch template: read body: %w", err)
	}

	var tmpl models.Template
	if err := json.Unmarshal(body, &tmpl); err != nil {
		return nil, fmt.Errorf("fetch template: unmarshal: %w", err)
	}

	return &tmpl, nil
}

// templateWithoutProof builds a map of the template excluding the proof field.
// Used for JCS/JWS signing of template integrity per spec §11.3.
func templateWithoutProof(tmpl *models.Template) (map[string]any, error) {
	raw, err := json.Marshal(tmpl)
	if err != nil {
		return nil, fmt.Errorf("marshal template: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal template to map: %w", err)
	}
	delete(m, "proof")
	return m, nil
}

// VerifyTemplateProof verifies the JWS proof in the template against the via entity's public key.
// The proof covers the JCS serialization of the template excluding the proof field.
func VerifyTemplateProof(tmpl *models.Template, viaPubKey ed25519.PublicKey) error {
	m, err := templateWithoutProof(tmpl)
	if err != nil {
		return fmt.Errorf("verify template proof: %w", err)
	}

	payload, err := crypto.CanonicalJSON(m)
	if err != nil {
		return fmt.Errorf("verify template proof: canonical JSON: %w", err)
	}

	// Re-attach payload to detached compact JWS
	jwsToken := tmpl.Proof.Sig
	parts := strings.Split(jwsToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("verify template proof: malformed JWS compact: expected 3 parts, got %d", len(parts))
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	attached := parts[0] + "." + encodedPayload + "." + parts[2]

	parsed, err := jose.ParseSigned(attached, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return fmt.Errorf("verify template proof: parse JWS: %w", err)
	}

	if _, err := parsed.Verify(viaPubKey); err != nil {
		return fmt.Errorf("verify template proof: signature verification failed: %w", err)
	}

	return nil
}

