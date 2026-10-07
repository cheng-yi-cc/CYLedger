package marketrelay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const token = "0123456789abcdefghijklmnopqrstuv"

func TestRelayTargetAllowlist(t *testing.T) {
	for _, path := range []string{"/v1/coinbase/products?limit=100", "/v1/coinbase/products/BTC-USD/ticker?limit=1", "/v1/coingecko/simple/price?ids=bitcoin&vs_currencies=usd", "/v1/coingecko/coins/bitcoin?market_data=false", "/v1/fx/usd/cny"} {
		u, _ := url.Parse(path)
		target, _, err := ResolveTarget(u)
		if err != nil || !strings.HasPrefix(target, "https://") {
			t.Fatalf("valid target rejected: %s %v", path, err)
		}
	}
	for _, path := range []string{"https://evil.example/anything", "/v1/coinbase/accounts", "/v1/coinbase/products/../../../accounts", "/v1/coingecko/coins/%2fetc", "/v1/coingecko/search?url=http://127.0.0.1", "/v1/fx/usd/cny?target=evil", "/v1/coinbase-exchange/accounts", "/v1/coinbase-exchange/products/BTC-USD/candles?url=https://evil.example", "/v1/coingecko/coins/bitcoin/market_chart", "/v1/bitstamp/ohlc/dogeusd/", "/v1/bitstamp/ohlc/btcusd/?url=https://evil.example", "/v1/fx/rates?url=https://evil.example"} {
		u, _ := url.Parse(path)
		if _, _, err := ResolveTarget(u); err == nil {
			t.Errorf("accepted unsupported target %s", path)
		}
	}
}

func TestRelayAuthCacheAndCredentialIsolation(t *testing.T) {
	var calls atomic.Int32
	relay, err := New(Config{Token: token, CoinGeckoAPIKey: "server-demo-key", Client: &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "api.coingecko.com" || r.Header.Get(TokenHeader) != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("x-cg-demo-api-key") != "server-demo-key" {
			t.Error("credentials/route crossed trust boundary")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"coins":[]}`))}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/v1/coingecko/search?query=btc", nil)
		if i > 0 {
			req.Header.Set(TokenHeader, token)
		}
		response := httptest.NewRecorder()
		relay.ServeHTTP(response, req)
		if i == 0 && response.Code != 401 || i > 0 && response.Code != 200 {
			t.Fatalf("unexpected status %d", response.Code)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("cache/auth generated %d upstream requests", calls.Load())
	}
	req := httptest.NewRequest("POST", "/v1/coingecko/search", nil)
	req.Header.Set(TokenHeader, token)
	response := httptest.NewRecorder()
	relay.ServeHTTP(response, req)
	if response.Code != 405 {
		t.Fatal("accepted mutation")
	}
}

func TestRelayFailureIsNotCached(t *testing.T) {
	var calls atomic.Int32
	relay, _ := New(Config{Token: token, Client: &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("limited"))}, nil
	})}})
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/v1/fx/usd/cny", nil)
		req.Header.Set(TokenHeader, token)
		response := httptest.NewRecorder()
		relay.ServeHTTP(response, req)
		if response.Code != 429 || response.Header().Get("Retry-After") == "" {
			t.Fatal("lost upstream limit status")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("error cached as successful market data")
	}
}
