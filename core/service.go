package core

import (
	"context"
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

var ErrLoginRequired = errors.New("login required: session expired or not authenticated")

const maxResponseBody = 2 << 20

var (
	reCSRFMeta     = regexp.MustCompile(`<meta[^>]+name=["']csrf-token["'][^>]+content=["']([^"']+)["']`)
	reCSRFInput    = regexp.MustCompile(`<input[^>]+name=["']_csrf["'][^>]+value=["']([^"']+)["']`)
	reCSRFFallback = regexp.MustCompile(`<input[^>]+id=["']g_csrf_token["'][^>]+value=["']([^"']+)["']`)
)

var commonHeaders = map[string]string{
	"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/144.0.0.0 Safari/537.36",
	"Accept":           "application/json, text/javascript, */*; q=0.01",
	"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
	"X-Requested-With": "XMLHttpRequest",
}

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

type LoginResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type SignService interface {
	LoginWithContext(ctx context.Context) error
	SignWithContext(ctx context.Context) (*SignResponse, error)
}

type Service struct {
	client    *http.Client
	email     string
	password  string
	baseURL   *url.URL
	loginPath string
	signPath  string
}

func NewService(email, password, baseURL, loginPath, signPath string) (*Service, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.MaxIdleConns = 10
	transport.MaxIdleConnsPerHost = 5
	transport.IdleConnTimeout = 30 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 20 * time.Second

	return &Service{
		client: &http.Client{
			Jar:       jar,
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		email:     email,
		password:  password,
		baseURL:   parsedBaseURL,
		loginPath: loginPath,
		signPath:  signPath,
	}, nil
}

func (service *Service) endpoint(path string) string {
	reference := &url.URL{Path: path}
	return service.baseURL.ResolveReference(reference).String()
}

func (service *Service) GetCSRFTokenWithContext(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.endpoint(service.loginPath), nil)
	if err != nil {
		return "", fmt.Errorf("create login page request: %w", err)
	}
	req.Header.Set("User-Agent", commonHeaders["User-Agent"])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", commonHeaders["Accept-Language"])

	resp, err := service.client.Do(req)
	if err != nil {
		return "", requestError(ctx, "get login page", err)
	}
	defer resp.Body.Close()
	body, err := readResponseBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login page returned status %d: %s", resp.StatusCode, truncateBody(body))
	}

	for _, expression := range []*regexp.Regexp{reCSRFMeta, reCSRFInput, reCSRFFallback} {
		if match := expression.FindSubmatch(body); len(match) >= 2 {
			return string(match[1]), nil
		}
	}
	return "", fmt.Errorf("CSRF token not found in login page")
}

func (service *Service) LoginWithContext(ctx context.Context) error {
	csrfToken, err := service.GetCSRFTokenWithContext(ctx)
	if err != nil {
		return fmt.Errorf("get CSRF token: %w", err)
	}
	form := url.Values{
		"_csrf":    {csrfToken},
		"email":    {service.email},
		"password": {service.password},
		"remember": {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, service.endpoint(service.loginPath), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
	}
	for key, value := range commonHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", strings.TrimRight(service.baseURL.String(), "/"))
	req.Header.Set("Referer", service.endpoint(service.loginPath))

	resp, err := service.client.Do(req)
	if err != nil {
		return requestError(ctx, "send login request", err)
	}
	defer resp.Body.Close()
	body, err := readResponseBody(resp.Body)
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, truncateBody(body))
	}
	var result LoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse login response: %w; body: %s", err, truncateBody(body))
	}
	if result.Code != 0 {
		message := strings.TrimSpace(result.Msg)
		if message == "" {
			message = fmt.Sprintf("unknown error code %d", result.Code)
		}
		return fmt.Errorf("login failed: %s", message)
	}
	return nil
}

func (service *Service) SignWithContext(ctx context.Context) (*SignResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, service.endpoint(service.signPath), nil)
	if err != nil {
		return nil, fmt.Errorf("create sign request: %w", err)
	}
	for key, value := range commonHeaders {
		req.Header.Set(key, value)
	}
	req.Header.Set("Referer", service.baseURL.String())

	resp, err := service.client.Do(req)
	if err != nil {
		return nil, requestError(ctx, "send sign request", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location == "" || strings.Contains(strings.ToLower(location), "login") {
			return nil, ErrLoginRequired
		}
	}
	body, err := readResponseBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sign response: %w", err)
	}
	bodyText := string(body)
	if strings.Contains(bodyText, "need-login-tips") || strings.Contains(bodyText, "操作需要登录") {
		return nil, ErrLoginRequired
	}
	if resp.StatusCode != http.StatusOK {
		message := fmt.Sprintf("sign request returned status %d: %s", resp.StatusCode, truncateBody(body))
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			return nil, NewSignError(ErrTypeRateLimit, message, nil)
		case resp.StatusCode >= http.StatusInternalServerError:
			return nil, NewSignError(ErrTypeServer, message, nil)
		default:
			return nil, fmt.Errorf("%s", message)
		}
	}
	var result SignResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse sign response: %w; body: %s", err, truncateBody(body))
	}
	return &result, nil
}

func readResponseBody(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, maxResponseBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBody {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxResponseBody)
	}
	return body, nil
}

func requestError(ctx context.Context, action string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s cancelled: %w", action, ctx.Err())
	}
	return fmt.Errorf("%s: %w", action, err)
}

func truncateBody(body []byte) string {
	const limit = 200
	if len(body) > limit {
		return string(body[:limit]) + "..."
	}
	return string(body)
}
