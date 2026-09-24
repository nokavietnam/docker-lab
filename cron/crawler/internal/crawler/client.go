package crawler

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"muaban-crawler/internal/config"
)

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36 Edg/127.0.0.0",
}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 128*1024)) // 128KB pre-allocated scratch buffer
	},
}

type HTTPClient struct {
	cfg           *config.Config
	client        *http.Client
	parsedProxies []*url.URL
	proxyIdx      uint32
	randSource    *rand.Rand
	mu            sync.Mutex
}

func NewHTTPClient(cfg *config.Config) *HTTPClient {
	hc := &HTTPClient{
		cfg:        cfg,
		randSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Pre-parse proxy URLs once during initialization
	for _, p := range cfg.Proxies {
		if parsed, err := url.Parse(p); err == nil {
			hc.parsedProxies = append(hc.parsedProxies, parsed)
		}
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		DisableCompression: false,
		// Staff Engineer Thread-Safe Proxy Function: Eliminates data races during concurrent requests
		Proxy: func(req *http.Request) (*url.URL, error) {
			if len(hc.parsedProxies) == 0 {
				return nil, nil
			}
			idx := atomic.AddUint32(&hc.proxyIdx, 1) % uint32(len(hc.parsedProxies))
			return hc.parsedProxies[idx], nil
		},
	}

	hc.client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return hc
}

// GetRandomUserAgent returns a rotated desktop user-agent
func (c *HTTPClient) GetRandomUserAgent() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return userAgents[c.randSource.Intn(len(userAgents))]
}

// ApplyJitter pauses execution between MinDelay and MaxDelay with context cancellation support
func (c *HTTPClient) ApplyJitter(ctx context.Context) error {
	delay := c.cfg.MinDelay
	if c.cfg.MaxDelay > c.cfg.MinDelay {
		diff := c.cfg.MaxDelay - c.cfg.MinDelay
		c.mu.Lock()
		jitter := time.Duration(c.randSource.Int63n(int64(diff)))
		c.mu.Unlock()
		delay += jitter
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// FetchHTML retrieves page HTML with anti-blocking headers, connection pooling, and exponential backoff
func (c *HTTPClient) FetchHTML(ctx context.Context, targetURL string) ([]byte, error) {
	maxRetries := 3
	backoff := 3 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := c.ApplyJitter(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Inject realistic browser headers
		ua := c.GetRandomUserAgent()
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Sec-Ch-Ua", `"Chromium";v="128", "Not;A=Brand";v="24", "Google Chrome";v="128"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Referer", c.cfg.BaseURL)

		resp, err := c.client.Do(req)
		if err != nil {
			if attempt == maxRetries || ctx.Err() != nil {
				return nil, fmt.Errorf("http request failed: %w", err)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			continue
		}

		// Handle rate limiting / Cloudflare challenge status codes
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			if attempt == maxRetries || ctx.Err() != nil {
				return nil, fmt.Errorf("rate limited or blocked by server: HTTP %d", resp.StatusCode)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
		}

		// Read into pooled buffer to avoid memory thrashing
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		_, err = io.Copy(buf, resp.Body)
		resp.Body.Close()

		if err != nil {
			bufPool.Put(buf)
			return nil, fmt.Errorf("error reading response body: %w", err)
		}

		result := make([]byte, buf.Len())
		copy(result, buf.Bytes())
		bufPool.Put(buf)

		return result, nil
	}

	return nil, fmt.Errorf("exhausted all retries for %s", targetURL)
}
