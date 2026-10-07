package marketquotes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// 两条路线都保留原域名和 TLS 证书验证。托管 DNS 的 TCP 成功并不代表
// TLS/HTTP 可用；慢连接时用系统路线竞速，避免等满超时仍没有机会回退。
type fundTransport struct {
	primary, fallback http.RoundTripper
	delay             time.Duration
}

type fundResponse struct {
	response *http.Response
	err      error
	route    int
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (b *cancelReadCloser) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.cancel)
	return err
}

func (t *fundTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet || !isFundDNSHost(req.URL.Hostname()) {
		return t.primary.RoundTrip(req)
	}
	results := make(chan fundResponse)
	done := make(chan struct{})
	cancels := make([]context.CancelFunc, 0, 2)
	winner := -1
	defer func() {
		close(done)
		for i, cancel := range cancels {
			if i != winner {
				cancel()
			}
		}
	}()
	start := func(transport http.RoundTripper) {
		ctx, cancel := context.WithCancel(req.Context())
		route := len(cancels)
		cancels = append(cancels, cancel)
		go func() {
			resp, err := transport.RoundTrip(req.Clone(ctx))
			select {
			case results <- fundResponse{resp, err, route}:
			case <-done:
				if resp != nil {
					resp.Body.Close()
				}
				cancel()
			}
		}()
	}
	start(t.primary)
	timer := time.NewTimer(t.delay)
	defer timer.Stop()
	failed := 0
	for {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-timer.C:
			if len(cancels) == 1 {
				start(t.fallback)
			}
		case result := <-results:
			if result.err == nil && result.response.StatusCode == http.StatusOK {
				winner = result.route
				result.response.Body = &cancelReadCloser{ReadCloser: result.response.Body, cancel: cancels[winner]}
				return result.response, nil
			}
			failed++
			if result.response != nil {
				result.response.Body.Close()
				if result.err == nil {
					result.err = fmt.Errorf("fund HTTP status %d", result.response.StatusCode)
				}
			}
			cancels[result.route]()
			if failed == 2 {
				return nil, result.err
			}
			if len(cancels) == 1 {
				timer.Stop()
				start(t.fallback)
			}
		}
	}
}
