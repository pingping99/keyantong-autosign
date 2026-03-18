package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ErrLoginRequired is returned by Sign when the session has expired.
var ErrLoginRequired = errors.New("login required: session expired or not authenticated")

// Pre-compiled regexps for CSRF token extraction.
var (
	reCSRFMeta     = regexp.MustCompile(`<meta[^>]+name="csrf-token"[^>]+content="([^"]+)"`)
	reCSRFInput    = regexp.MustCompile(`<input[^>]+name="_csrf"[^>]+value="([^"]+)"`)
	reCSRFFallback = regexp.MustCompile(`<input[^>]+id="g_csrf_token"[^>]+value="([^"]+)"`)
)

// commonHeaders contains shared HTTP headers to simulate browser requests.
var commonHeaders = map[string]string{
	"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36",
	"Accept":             "application/json, text/javascript, */*; q=0.01",
	"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
	"X-Requested-With":   "XMLHttpRequest",
	"Sec-CH-UA":          `"Not(A:Brand";v="8", "Chromium";v="144", "Google Chrome";v="144"`,
	"Sec-CH-UA-Mobile":   "?0",
	"Sec-CH-UA-Platform": `"Windows"`,
	"Sec-Fetch-Site":     "same-origin",
	"Sec-Fetch-Mode":     "cors",
	"Sec-Fetch-Dest":     "empty",
}

// SignResponse represents the sign-in response.
type SignResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		SignCount    int    `json:"signcount"`
		SignPoint    int    `json:"signpoint"`
		TodayHistory string `json:"today_history"`
		IsAlert      int    `json:"is_alert"`
	} `json:"data"`
}

// LoginResponse represents the login response.
type LoginResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Service handles sign-in operations including HTTP transport.
type Service struct {
	client    *http.Client
	email     string
	password  string
	baseURL   string
	loginPath string
	signPath  string
}

// NewService creates a new sign-in service with configurable endpoints.
func NewService(email, password, baseURL, loginPath, signPath string) (*Service, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
			DisableKeepAlives:  false,
		},
	}

	return &Service{
		client:    client,
		email:     email,
		password:  password,
		baseURL:   baseURL,
		loginPath: loginPath,
		signPath:  signPath,
	}, nil
}

// GetCSRFToken fetches CSRF token from login page.
func (s *Service) GetCSRFToken() (string, error) {
	loginPageURL := s.baseURL + s.loginPath
	req, err := http.NewRequest(http.MethodGet, loginPageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", commonHeaders["User-Agent"])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", commonHeaders["Accept-Language"])

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get login page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

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

// Login performs login operation.
func (s *Service) Login() error {
	csrfToken, err := s.GetCSRFToken()
	if err != nil {
		return fmt.Errorf("failed to get CSRF token: %w", err)
	}

	data := url.Values{}
	data.Set("_csrf", csrfToken)
	data.Set("email", s.email)
	data.Set("password", s.password)
	data.Set("remember", "1")

	loginURL := s.baseURL + s.loginPath
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	for k, v := range commonHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", s.baseURL)
	req.Header.Set("Referer", s.baseURL+s.loginPath)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var result LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		ct := resp.Header.Get("Content-Type")
		return fmt.Errorf("failed to parse login response (Content-Type: %q): %w, body: %s", ct, err, truncateBody(body))
	}

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
func (s *Service) Sign() (*SignResponse, error) {
	signURL := s.baseURL + s.signPath
	req, err := http.NewRequest(http.MethodGet, signURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sign request: %w", err)
	}

	for k, v := range commonHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("Referer", s.baseURL+"/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send sign request: %w", err)
	}
	defer resp.Body.Close()

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

	if strings.Contains(string(body), `need-login-tips`) || strings.Contains(string(body), `对不起，您的操作需要登录才可以进行`) {
		return nil, ErrLoginRequired
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sign request failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var signResp SignResponse
	if err := json.Unmarshal(body, &signResp); err != nil {
		ct := resp.Header.Get("Content-Type")
		return nil, fmt.Errorf("failed to parse sign response (Content-Type: %q): %w, body: %s", ct, err, truncateBody(body))
	}

	return &signResp, nil
}

// truncateBody returns the first 200 bytes of body for error messages.
func truncateBody(body []byte) string {
	if len(body) > 200 {
		return string(body[:200]) + "..."
	}
	return string(body)
}
