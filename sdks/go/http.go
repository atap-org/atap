package atap

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient is a low-level HTTP client with DPoP proof injection and error handling.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// RequestOptions holds optional parameters for HTTP requests.
type RequestOptions struct {
	JSONBody    interface{}
	RawBody     []byte
	ContentType string
	Headers     map[string]string
	Params      map[string]string
}

// NewHTTPClient creates a new HTTPClient.
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *HTTPClient) buildURL(path string, params map[string]string) string {
	u := c.baseURL + path
	if len(params) > 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}
		u += "?" + vals.Encode()
	}
	return u
}

// Request makes a public HTTP request.
func (c *HTTPClient) Request(ctx context.Context, method, path string, opts *RequestOptions) (map[string]interface{}, error) {
	if opts == nil {
		opts = &RequestOptions{}
	}

	var body io.Reader
	if opts.JSONBody != nil {
		b, err := json.Marshal(opts.JSONBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = bytes.NewReader(b)
	} else if opts.RawBody != nil {
		body = bytes.NewReader(opts.RawBody)
	}

	fullURL := c.buildURL(path, opts.Params)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if opts.JSONBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", opts.ContentType)
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

// AuthenticatedRequest makes a DPoP-authenticated HTTP request.
func (c *HTTPClient) AuthenticatedRequest(
	ctx context.Context,
	method, path string,
	signingKey ed25519.PrivateKey,
	accessToken, platformDomain string,
	opts *RequestOptions,
) (map[string]interface{}, error) {
	if opts == nil {
		opts = &RequestOptions{}
	}

	htu := fmt.Sprintf("https://%s%s", platformDomain, path)
	dpopProof := MakeDPoPProof(signingKey, method, htu, accessToken)

	if opts.Headers == nil {
		opts.Headers = make(map[string]string)
	}
	opts.Headers["Authorization"] = "DPoP " + accessToken
	opts.Headers["DPoP"] = dpopProof

	return c.Request(ctx, method, path, opts)
}

// PostForm sends a form-encoded POST request (for OAuth token endpoint).
func (c *HTTPClient) PostForm(ctx context.Context, path string, formData map[string]string, dpopProof string) (map[string]interface{}, error) {
	vals := url.Values{}
	for k, v := range formData {
		vals.Set(k, v)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create form request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if dpopProof != "" {
		req.Header.Set("DPoP", dpopProof)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http form request: %w", err)
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

// GetRedirect makes a GET request expecting a 302 redirect and returns the Location URL.
func (c *HTTPClient) GetRedirect(ctx context.Context, path string, params map[string]string, dpopProof string) (string, error) {
	fullURL := c.buildURL(path, params)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("create redirect request: %w", err)
	}
	if dpopProof != "" {
		req.Header.Set("DPoP", dpopProof)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http redirect request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		_, handleErr := c.handleResponse(resp)
		if handleErr != nil {
			return "", handleErr
		}
		return "", NewATAPError(fmt.Sprintf("expected 302 redirect, got %d", resp.StatusCode), resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", NewATAPError("302 redirect with no Location header", 302)
	}
	return location, nil
}

func (c *HTTPClient) handleResponse(resp *http.Response) (map[string]interface{}, error) {
	if resp.StatusCode == http.StatusNoContent {
		return map[string]interface{}{}, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return map[string]interface{}{}, nil
		}
		return nil, NewATAPError(
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
			resp.StatusCode,
		)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, nil
	}

	// Parse RFC 7807 Problem Detail.
	var problem *ProblemDetail
	if _, hasType := data["type"]; hasType {
		if _, hasStatus := data["status"]; hasStatus {
			problem = &ProblemDetail{
				Type:   getString(data, "type"),
				Title:  getString(data, "title"),
				Status: getInt(data, "status", resp.StatusCode),
				Detail: getString(data, "detail"),
			}
			if inst, ok := data["instance"].(string); ok {
				problem.Instance = inst
			}
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		msg := "Authentication failed"
		if problem != nil && problem.Detail != "" {
			msg = problem.Detail
		} else if d := getString(data, "detail"); d != "" {
			msg = d
		}
		return nil, NewATAPAuthError(msg, resp.StatusCode, problem)
	case http.StatusNotFound:
		msg := "Not found"
		if problem != nil && problem.Detail != "" {
			msg = problem.Detail
		}
		return nil, NewATAPNotFoundError(msg, problem)
	case http.StatusConflict:
		msg := "Conflict"
		if problem != nil && problem.Detail != "" {
			msg = problem.Detail
		}
		return nil, NewATAPConflictError(msg, problem)
	case http.StatusTooManyRequests:
		msg := "Rate limit exceeded"
		if problem != nil && problem.Detail != "" {
			msg = problem.Detail
		}
		return nil, NewATAPRateLimitError(msg, problem)
	default:
		if problem != nil {
			return nil, NewATAPProblemError(*problem)
		}
		msg := getString(data, "detail")
		if msg == "" {
			msg = getString(data, "message")
		}
		if msg == "" {
			msg = fmt.Sprintf("%v", data)
		}
		return nil, NewATAPError(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, msg), resp.StatusCode)
	}
}

func getString(data map[string]interface{}, key string) string {
	if v, ok := data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(data map[string]interface{}, key string, fallback int) int {
	if v, ok := data[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return fallback
}
