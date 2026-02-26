package client

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client wraps http.Client with cookie management
type Client struct {
	*http.Client
}

// NewClient creates a new HTTP client with cookie jar
func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			// Disable auto-redirect so we can detect login redirects manually
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
				DisableKeepAlives:  false,
			},
		},
	}, nil
}

// GetCommonHeaders returns common HTTP headers to simulate browser
func GetCommonHeaders() map[string]string {
	return map[string]string{
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
}
