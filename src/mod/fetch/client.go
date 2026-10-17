package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alyshmahell/matchmedia/src/mod/config"
)

type httpClient struct {
	hc             *http.Client
	userAgent      string
	retries        int
	backoffMin     int
	backoffMax     int
	attemptTimeout time.Duration
	cool           *Circuit
	coolFails      int
	coolMinMS      int
	coolMaxMS      int
}

func (c *httpClient) forSpec(spec config.Provider) *httpClient {
	if c == nil || (spec.Retries <= 0 && spec.AttemptMS <= 0) {
		return c
	}
	cp := *c
	if spec.Retries > 0 {
		cp.retries = spec.Retries
	}
	if spec.AttemptMS > 0 {
		cp.attemptTimeout = time.Duration(spec.AttemptMS) * time.Millisecond
	}
	return &cp
}

func newHTTP(cfg *config.Config, cool *Circuit) *httpClient {
	br := cfg.HTTP.Backoff
	cd := cfg.HTTP.Cooldown
	return &httpClient{
		hc:             &http.Client{Timeout: cfg.HTTPTimeout()},
		userAgent:      "matchmedia/" + cfg.Version + " (+https://github.com/alyshmahell/matchmedia)",
		retries:        cfg.HTTP.Retries,
		backoffMin:     br.MinMS,
		backoffMax:     br.MaxMS,
		attemptTimeout: cfg.AttemptTimeout(),
		cool:           cool,
		coolFails:      cd.Fails,
		coolMinMS:      cd.MinMS,
		coolMaxMS:      cd.MaxMS,
	}
}

func (c *httpClient) noteFail(key string) {
	if c == nil || key == "" {
		return
	}
	c.cool.Fail(key, c.coolFails, c.coolMinMS, c.coolMaxMS)
}

func (c *httpClient) get(ctx context.Context, url string, pace func(context.Context) error, key string) ([]byte, int, error) {
	b, code, _, err := c.do(ctx, http.MethodGet, url, "application/json", nil, pace, key)
	return b, code, err
}

func (c *httpClient) getBytes(ctx context.Context, url string) ([]byte, string, int, error) {
	b, code, ctype, err := c.do(ctx, http.MethodGet, url, "", nil, nil, "")
	return b, ctype, code, err
}

func (c *httpClient) do(ctx context.Context, method, url, contentType string, body io.Reader, pace func(context.Context) error, key string) ([]byte, int, string, error) {
	canRetry := method == http.MethodGet && body == nil
	var lastErr error
	var lastCode int
	for attempt := 0; attempt < c.retries; attempt++ {
		if key != "" {
			if err := c.cool.Wait(ctx, key); err != nil {
				return nil, 0, "", err
			}
		}
		if pace != nil {
			if err := pace(ctx); err != nil {
				return nil, 0, "", err
			}
		}
		reqCtx := ctx
		cancel := func() {}
		if canRetry && c.attemptTimeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, c.attemptTimeout)
		}
		req, err := http.NewRequestWithContext(reqCtx, method, url, body)
		if err != nil {
			cancel()
			return nil, 0, "", err
		}
		req.Header.Set("User-Agent", c.userAgent)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			cancel()
			c.noteFail(key)
			if !canRetry || attempt == c.retries-1 || (key != "" && c.cool.Cooling(key)) {
				return nil, 0, "", err
			}
			lastErr = err
			if err := sleepBackoff(ctx, c.backoff(attempt), 0); err != nil {
				return nil, 0, "", err
			}
			continue
		}
		ctype := resp.Header.Get("Content-Type")
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if err != nil {
			c.noteFail(key)
			return nil, resp.StatusCode, ctype, err
		}
		if canRetry && retryableStatus(resp.StatusCode) && attempt < c.retries-1 {
			c.noteFail(key)
			if key != "" && c.cool.Cooling(key) {
				return nil, resp.StatusCode, ctype, fmt.Errorf("status %d from %s", resp.StatusCode, url)
			}
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, url)
			lastCode = resp.StatusCode
			if err := sleepBackoff(ctx, c.backoff(attempt), retryAfter(resp, c.backoffMax)); err != nil {
				return nil, resp.StatusCode, ctype, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			c.noteFail(key)
		}
		return b, resp.StatusCode, ctype, nil
	}
	return nil, lastCode, "", lastErr
}

func (c *httpClient) backoff(attempt int) time.Duration {
	wait := c.backoffMin
	if attempt > 0 {
		for range attempt {
			next := wait * 2
			if next > c.backoffMax || next <= 0 {
				wait = c.backoffMax
				break
			}
			wait = next
		}
	}
	if wait > c.backoffMax || wait <= 0 {
		wait = c.backoffMax
	}
	lo := wait / 2
	if lo < 1 {
		lo = 1
	}
	return config.JitterMS(lo, wait)
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

func retryAfter(resp *http.Response, maxMS int) time.Duration {
	if resp == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || n <= 0 {
		return 0
	}
	d := time.Duration(n) * time.Second
	if maxMS < 1 {
		return d
	}
	capd := time.Duration(maxMS) * time.Millisecond
	if d > capd {
		return capd
	}
	return d
}

func sleepBackoff(ctx context.Context, wait, after time.Duration) error {
	if after > 0 {
		wait = after
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

func Bytes(ctx context.Context, cfg *config.Config, rawURL, provider string) ([]byte, string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, "", fmt.Errorf("empty url")
	}
	c := newHTTP(cfg, nil)
	if spec, ok := cfg.Providers[provider]; ok {
		c = c.forSpec(spec)
	}
	b, ctype, code, err := c.getBytes(ctx, rawURL)
	if err != nil {
		return nil, "", err
	}
	if code >= 400 {
		return nil, "", fmt.Errorf("status %d", code)
	}
	return b, ctype, nil
}
