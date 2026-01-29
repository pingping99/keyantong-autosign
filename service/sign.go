package service

import (
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

// SignResponse represents the sign-in response
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
func (s *Service) GetCSRFToken() (string, error) {
	req, err := http.NewRequest("GET", LoginPage, nil)
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

	// Extract CSRF token from HTML
	// Try meta csrf
	reMeta := regexp.MustCompile(`<meta[^>]+name="csrf-token"[^>]+content="([^"]+)"`)
	if metaMatch := reMeta.FindSubmatch(body); len(metaMatch) >= 2 {
		return string(metaMatch[1]), nil
	}

	// Fall back to input hidden tokens (allow additional attributes)
	reInput := regexp.MustCompile(`<input[^>]+name="_csrf"[^>]+value="([^"]+)"`)
	if inputMatch := reInput.FindSubmatch(body); len(inputMatch) >= 2 {
		return string(inputMatch[1]), nil
	}

	// Finally check fallback token fields
	reFallback := regexp.MustCompile(`<input[^>]+id="g_csrf_token"[^>]+value="([^"]+)"`)
	if fallback := reFallback.FindSubmatch(body); len(fallback) >= 2 {
		return string(fallback[1]), nil
	}

	return "", fmt.Errorf("CSRF token not found in page")
}

// Login performs login operation
func (s *Service) Login() error {
	// Get CSRF token
	csrfToken, err := s.GetCSRFToken()
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
	req, err := http.NewRequest("POST", LoginURL, strings.NewReader(data.Encode()))
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

	// Check login result
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	// Check if login successful
	if code, ok := result["code"].(float64); ok && code != 0 {
		msg := result["msg"].(string)
		return fmt.Errorf("login failed: %s", msg)
	}

	return nil
}

// Sign performs sign-in operation
func (s *Service) Sign() (*SignResponse, error) {
	req, err := http.NewRequest("GET", SignURL, nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sign response: %w", err)
	}

	// Parse response
	var signResp SignResponse
	if err := json.Unmarshal(body, &signResp); err != nil {
		return nil, fmt.Errorf("failed to parse sign response: %w", err)
	}

	return &signResp, nil
}

// AutoSign performs automatic sign-in process
func (s *Service) AutoSign() error {
	// Login first
	fmt.Println("正在登录...")
	if err := s.Login(); err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}
	fmt.Println("✓ 登录成功")

	// Perform sign-in
	fmt.Println("正在签到...")
	signResp, err := s.Sign()
	if err != nil {
		return fmt.Errorf("签到失败: %w", err)
	}

	// Display result
	if signResp.Code == 0 {
		fmt.Printf("✓ %s\n", signResp.Msg)
		fmt.Printf("  连续签到: %d 次\n", signResp.Data.SignCount)
		fmt.Printf("  本次获得: %d 积分\n", signResp.Data.SignPoint)
	} else {
		return fmt.Errorf("签到失败: %s", signResp.Msg)
	}

	return nil
}
