package marketrelay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestCancelledClientsDoNotReleaseUpstreamBudget(t *testing.T) {
	started := make(chan struct{}, 20)
	finished := make(chan struct{}, 20)
	release := make(chan struct{})
	defer close(release)
	relay, _ := New(Config{Token: token, Client: &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		defer func() { finished <- struct{}{} }()
		started <- struct{}{}
		select {
		case <-release:
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})}})
	for i := 0; i < 12; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/coingecko/search?query=coin%d", i), nil).WithContext(ctx)
		req.Header.Set(TokenHeader, token)
		returned := make(chan struct{})
		go func() { defer close(returned); relay.ServeHTTP(httptest.NewRecorder(), req) }()
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("upstream did not start")
		}
		cancel()
		select {
		case <-returned:
		case <-time.After(2 * time.Second):
			t.Fatal("cancelled caller remained blocked")
		}
	}
	req := httptest.NewRequest("GET", "/v1/coingecko/search?query=one-more", nil)
	req.Header.Set(TokenHeader, token)
	response := httptest.NewRecorder()
	relay.ServeHTTP(response, req)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("exceeded upstream budget: %d", response.Code)
	}
	select {
	case <-started:
		t.Fatal("started a thirteenth upstream")
	default:
	}
}

func TestRelayWebSocketEndToEndAndHeaderIsolation(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(TokenHeader) != "" || r.Header.Get("X-Private-Value") != "" || r.Header.Get("X-Cg-Demo-Api-Key") != "" {
			t.Error("forwarded private handshake headers")
		}
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			var value string
			if websocket.Message.Receive(conn, &value) == nil {
				_ = websocket.Message.Send(conn, value)
			}
		}).ServeHTTP(w, r)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	transport := upstream.Client().Transport
	relay, err := New(Config{Token: token, Client: &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "advanced-trade-ws.coinbase.com" || r.URL.Path != "/" {
			t.Error("wrong upstream")
		}
		clone := r.Clone(r.Context())
		clone.URL.Scheme, clone.URL.Host = target.Scheme, target.Host
		clone.Host = target.Host
		return transport.RoundTrip(clone)
	})}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(relay)
	defer server.Close()
	config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/v1/coinbase-ws", "https://www.coinbase.com")
	config.Header.Set(TokenHeader, token)
	config.Header.Set("X-Private-Value", "must-not-leave-relay")
	config.Header.Set("X-Cg-Demo-Api-Key", "must-not-leave-relay")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := config.DialContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Send(conn, "public price"); err != nil {
		t.Fatal(err)
	}
	var value string
	if err := websocket.Message.Receive(conn, &value); err != nil || value != "public price" {
		t.Fatalf("relay stream failed: %s %v", value, err)
	}
}
