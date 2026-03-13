package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/models"
)

// TestProblemHelperContentType verifies problem() sets Content-Type: application/problem+json.
func TestProblemHelperContentType(t *testing.T) {
	app := fiber.New()
	app.Get("/test-error", func(c *fiber.Ctx) error {
		return problem(c, 400, "bad-request", "Bad Request", "Something went wrong")
	})

	req := httptest.NewRequest("GET", "/test-error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected Content-Type=application/problem+json, got %q", ct)
	}
}

// TestProblemHelperFields verifies all RFC 7807 fields are present with correct values.
func TestProblemHelperFields(t *testing.T) {
	app := fiber.New()
	app.Get("/v1/entities", func(c *fiber.Ctx) error {
		return problem(c, 404, "not-found", "Not Found", "Entity not found")
	})

	req := httptest.NewRequest("GET", "/v1/entities", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var pd models.ProblemDetail
	if err := json.NewDecoder(resp.Body).Decode(&pd); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if pd.Type != "https://atap.dev/errors/not-found" {
		t.Errorf("expected type=https://atap.dev/errors/not-found, got %q", pd.Type)
	}
	if pd.Title != "Not Found" {
		t.Errorf("expected title=Not Found, got %q", pd.Title)
	}
	if pd.Status != 404 {
		t.Errorf("expected status=404, got %d", pd.Status)
	}
	if pd.Detail != "Entity not found" {
		t.Errorf("expected detail=Entity not found, got %q", pd.Detail)
	}
	if pd.Instance != "/v1/entities" {
		t.Errorf("expected instance=/v1/entities, got %q", pd.Instance)
	}
}

// TestErrorTypeURIFormat verifies different error types produce correct URIs.
func TestErrorTypeURIFormat(t *testing.T) {
	tests := []struct {
		errType  string
		wantType string
	}{
		{"unauthorized", "https://atap.dev/errors/unauthorized"},
		{"not-found", "https://atap.dev/errors/not-found"},
		{"validation", "https://atap.dev/errors/validation"},
		{"internal", "https://atap.dev/errors/internal"},
	}

	for _, tc := range tests {
		t.Run(tc.errType, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				return problem(c, 400, tc.errType, "Title", "Detail")
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			var pd models.ProblemDetail
			if err := json.NewDecoder(resp.Body).Decode(&pd); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if pd.Type != tc.wantType {
				t.Errorf("expected type=%q, got %q", tc.wantType, pd.Type)
			}
		})
	}
}

// TestGlobalErrorHandlerRFC7807 verifies the Fiber global error handler returns RFC 7807 format.
func TestGlobalErrorHandlerRFC7807(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: GlobalErrorHandler,
	})
	app.Get("/panic", func(c *fiber.Ctx) error {
		return errors.New("unexpected internal error")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected Content-Type=application/problem+json, got %q", ct)
	}

	var pd models.ProblemDetail
	if err := json.NewDecoder(resp.Body).Decode(&pd); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if pd.Type == "" {
		t.Error("expected non-empty type field")
	}
	if pd.Status == 0 {
		t.Error("expected non-zero status field")
	}
}

// TestUnknownRoute404IsRFC7807 verifies 404 for unknown routes returns RFC 7807.
func TestUnknownRoute404IsRFC7807(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: GlobalErrorHandler,
	})

	req := httptest.NewRequest("GET", "/this-route-does-not-exist", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("expected Content-Type=application/problem+json, got %q", ct)
	}

	var pd models.ProblemDetail
	if err := json.NewDecoder(resp.Body).Decode(&pd); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if pd.Type != "https://atap.dev/errors/not-found" {
		t.Errorf("expected type=https://atap.dev/errors/not-found, got %q", pd.Type)
	}
}
