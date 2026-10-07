package marketquotes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

type routeTransport func(*http.Request) (*http.Response, error)

func (f routeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func routeResponse() *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}
}

const testRelayToken = "0123456789abcdefghijklmnopqrstuv"

func TestMarketRoutingIsolation(t *testing.T) {
	var direct, overseas atomic.Int32
	s := New(Config{Network: NetworkConfig{RelayURL: "https://quotes.example", RelayToken: testRelayToken},
		HTTPClient: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
			overseas.Add(1)
			if r.Header.Get("X-CYLedger-Relay-Token") != "" {
				t.Error("relay secret leaked to upstream")
			}
			return routeResponse(), nil
		})},
		DomesticHTTPClient: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
			direct.Add(1)
			if r.URL.Host == "quotes.example" {
				if r.URL.Path != "/v1/coinbase/products" || r.Header.Get("X-CYLedger-Relay-Token") != testRelayToken || r.Header.Get("x-cg-demo-api-key") != "" || r.Header.Get("Authorization") != "" {
					t.Error("incorrect relay request")
				}
			} else if r.Header.Get("X-CYLedger-Relay-Token") != "" {
				t.Error("relay secret leaked to domestic provider")
			}
			return routeResponse(), nil
		})},
	})
	for _, address := range []string{s.config.FundSearchURL, s.config.FundNAVURL, s.config.TencentSearchURL, s.config.TencentURL, s.config.CoinbaseRESTURL + "/products", "https://unrelated.example/"} {
		req, _ := http.NewRequest(http.MethodGet, address, nil)
		response, err := s.doPublicRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if req.URL.Host == "quotes.example" || req.Header.Get("X-CYLedger-Relay-Token") != "" {
			t.Fatal("mutated caller request")
		}
	}
	if direct.Load() != 5 || overseas.Load() != 1 {
		t.Fatalf("route pools: domestic %d overseas %d", direct.Load(), overseas.Load())
	}
	transport := newMarketHTTPClient(true).Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("domestic client inherited HTTP proxy")
	}
}

func TestMarketRelayConfigValidation(t *testing.T) {
	for _, address := range []string{"http://example.com", "https://user:password@example.com", "https://example.com/path", "https://example.com?url=evil", "https://example.com#fragment", "file:///tmp/relay"} {
		if (NetworkConfig{RelayURL: address, RelayToken: testRelayToken}).Validate() == nil {
			t.Errorf("accepted %s", address)
		}
	}
	for _, token := range []string{"", "short", testRelayToken + "\r\n"} {
		if (NetworkConfig{RelayURL: "https://example.com", RelayToken: token}).Validate() == nil {
			t.Error("accepted invalid token")
		}
	}
	if (NetworkConfig{RelayURL: "http://127.0.0.1:8787", RelayToken: testRelayToken}).Validate() != nil {
		t.Fatal("loopback tests rejected")
	}
}

func TestMarketRelayStreamHandshake(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/coinbase-ws" || r.Header.Get("X-CYLedger-Relay-Token") != testRelayToken {
			t.Error("wrong relay handshake")
			http.Error(w, "wrong route", 400)
			return
		}
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()
			var text string
			if websocket.Message.Receive(conn, &text) == nil {
				_ = websocket.Message.Send(conn, text)
			}
		}).ServeHTTP(w, r)
	}))
	defer server.Close()
	s := New(Config{Network: NetworkConfig{RelayURL: server.URL, RelayToken: testRelayToken}})
	cfg, _ := websocket.NewConfig(s.config.CoinbaseWSURL, "https://www.coinbase.com")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := s.dialMarketStream(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = websocket.Message.Send(conn, "public ticker"); err != nil {
		t.Fatal(err)
	}
	var text string
	if err = websocket.Message.Receive(conn, &text); err != nil || text != "public ticker" {
		t.Fatalf("stream failed: %q %v", text, err)
	}
}

func TestNetworkChangeInvalidatesIdlePoolsWithoutClearingPrices(t *testing.T) {
	s := New(Config{})
	before := len(s.Quotes())
	s.CloseIdleConnections()
	if len(s.Quotes()) != before {
		t.Fatal("network change cleared public cache")
	}
	// Test URL prefix matching without granting an endpoint's lookalike sibling.
	u, _ := url.Parse("https://api.coinbase.com/api/v3/brokerage/market-evil/products")
	if _, ok := endpointSuffix(u, s.config.CoinbaseRESTURL); ok {
		t.Fatal("matched sibling path")
	}
}
