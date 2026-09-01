package di

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ClientOption represents a functional option for configuring a Client.
type ClientOption func(*Client) error

// Client is a hardened HTTP client configured via functional options.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	timeout      time.Duration
	maxRetries   int
	userAgent    string
	maxIdleConns int
}

// WithTimeout sets a custom request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) error {
		if d <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		c.timeout = d
		return nil
	}
}

// WithMaxRetries configures maximum retry attempts.
func WithMaxRetries(retries int) ClientOption {
	return func(c *Client) error {
		if retries < 0 {
			return errors.New("retries cannot be negative")
		}
		c.maxRetries = retries
		return nil
	}
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) error {
		if ua == "" {
			return errors.New("user agent cannot be empty")
		}
		c.userAgent = ua
		return nil
	}
}

// WithMaxIdleConns configures connection pool idle count.
func WithMaxIdleConns(n int) ClientOption {
	return func(c *Client) error {
		if n <= 0 {
			return errors.New("max idle connections must be > 0")
		}
		c.maxIdleConns = n
		return nil
	}
}

// NewClient constructs a Client applying options over sensible defaults.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL cannot be empty")
	}

	// Sensible production defaults
	client := &Client{
		baseURL:      baseURL,
		timeout:      5 * time.Second,
		maxRetries:   3,
		userAgent:    "MyService-GoClient/1.0",
		maxIdleConns: 100,
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}

	transport := &http.Transport{
		MaxIdleConns:        client.maxIdleConns,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client.httpClient = &http.Client{
		Timeout:   client.timeout,
		Transport: transport,
	}

	return client, nil
}

func (c *Client) BaseURL() string          { return c.baseURL }
func (c *Client) Timeout() time.Duration   { return c.timeout }
func (c *Client) MaxRetries() int         { return c.maxRetries }
func (c *Client) UserAgent() string        { return c.userAgent }
func (c *Client) MaxIdleConns() int       { return c.maxIdleConns }
