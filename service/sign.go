package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"keyantong/client"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	BaseURL   = "https://www.ablesci.com"
	LoginURL  = BaseURL + "/site/login"
	SignURL   = BaseURL + "/user/sign"
	LoginPage = BaseURL + "/site/login"
)

// Pre-compiled regexps for CSRF token extraction
var (
	reCSRFMeta     = regexp.MustCompile(`<meta[^>]+name="csrf-token"[^>]+content="([^"]+)"`)
	reCSRFInput    = regexp.MustCompile(`<input[^>]+name="_csrf"[^>]+value="([^"]+)"`)
	reCSRFFallback = regexp.MustCompile(`<input[^>]+id="g_csrf_token"[^>]+value="([^"]+)"`)
)

// SignResponse represents the sign-in response
type SignResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		SignCount     int    `json:"signcount"`
		SignPoint     int    `json:"signpoint"`
		TodayHistory string `json:"today_history"`
		IsAlert      int    `json:"is_alert"`
	} `json:"data"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Service handles sign-in operations
type Service struct {
	client   *client.Client
	email    string
	password string
}

// NewService creates a new sign-in service
func NewService(email, password string) (*Service, error) {
	c, err := client.NewClient()
	if err != nil {
		return nil, err
	}

	return &Service{
		client:   c,
		email:    email,
		password: password,
	}, nil
}

// GetCSRFToken fetches CSRF token from login page
func (s *Service) GetCSRFToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LoginPage, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	headers := client.GetCommonHeaders()
	req.Header.Set("User-Agent", headers["User-Agent"])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", headers["Accept-Language"])

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get login page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Extract CSRF token from HTML using pre-compiled regexps
	if match := reCSRFMeta.FindSubmatch(body); len(match) >= 2 {
		return string(match[1]), nil
	}
	if match := reCSRFInput.FindSubmatch(body); len(match) >= 2 {
		return string(match[1]), nil
	}
	if match := reCSRFFallback.FindSubmatch(body); len(match) >= 2 {
		return string(match[1]), nil
	}

	return "", fmt.Errorf("CSRF token not found in page")
}

// Login performs login operation
func (s *Service) Login(ctx context.Context) error {
	// Get CSRF token
	csrfToken, err := s.GetCSRFToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get CSRF token: %w", err)
	}

	// Prepare login data
	data := url.Values{}
	data.Set("_csrf", csrfToken)
	data.Set("email", s.email)
	data.Set("password", s.password)
	data.Set("remember", "1")

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, LoginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	// Set headers
	headers := client.GetCommonHeaders()
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", LoginPage)

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	// Parse JSON response (skip Content-Type check as server may return incorrect headers)
	var result LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		ct := resp.Header.Get("Content-Type")
		return fmt.Errorf("failed to parse login response (Content-Type: %q): %w, body: %s", ct, err, truncateBody(body))
	}

	// Check if login successful
	if result.Code != 0 {
		msg := result.Msg
		if msg == "" {
			msg = fmt.Sprintf("unknown error (code: %d)", result.Code)
		}
		return fmt.Errorf("login failed: %s", msg)
	}

	return nil
}

// Sign performs sign-in operation.
// Returns (nil, ErrLoginRequired) if the session has expired (HTTP 302 redirect to login page).
func (s *Service) Sign(ctx context.Context) (*SignResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SignURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sign request: %w", err)
	}

	// Set headers
	headers := client.GetCommonHeaders()
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Referer", BaseURL+"/")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send sign request: %w", err)
	}
	defer resp.Body.Close()

	// Detect session expiry: server redirects to login page
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		location := resp.Header.Get("Location")
		if strings.Contains(location, "login") || location == "" {
			return nil, ErrLoginRequired
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sign response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sign request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	// Parse response (skip Content-Type check as server may return incorrect headers)
	var signResp SignResponse
	if err := json.Unmarshal(body, &signResp); err != nil {
		ct := resp.Header.Get("Content-Type")
		return nil, fmt.Errorf("failed to parse sign response (Content-Type: %q): %w, body: %s", ct, err, truncateBody(body))
	}

	return &signResp, nil
}

// ErrLoginRequired is returned by Sign when the session has expired.
var ErrLoginRequired = fmt.Errorf("login required: session expired or not authenticated")

// truncateBody returns the first 200 bytes of body for error messages.
func truncateBody(body []byte) string {
	if len(body) > 200 {
		return string(body[:200]) + "..."
	}
	return string(body)
}
