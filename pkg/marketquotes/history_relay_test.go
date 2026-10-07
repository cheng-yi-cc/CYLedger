package marketquotes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/marketrelay"
)

func TestHistoryLookupsUseRelay(t *testing.T) {
	var direct, upstream atomic.Int32
	relay, err := marketrelay.New(marketrelay.Config{Token: testRelayToken, Client: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
		upstream.Add(1)
		if r.Header.Get(marketrelay.TokenHeader) != "" {
			t.Error("relay credential leaked")
		}
		switch r.URL.Host {
		case "api.coinbase.com", "api.exchange.coinbase.com", "api.coingecko.com", "api.frankfurter.dev":
		default:
			t.Error("unexpected upstream host")
		}
		return routeResponse(), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(relay)
	defer server.Close()
	s := New(Config{Network: NetworkConfig{RelayURL: server.URL, RelayToken: testRelayToken}, DomesticHTTPClient: server.Client(), HTTPClient: &http.Client{Transport: routeTransport(func(*http.Request) (*http.Response, error) {
		direct.Add(1)
		return nil, errors.New("overseas source is unavailable without relay")
	})}})
	for _, address := range []string{
		s.config.CoinbaseRESTURL + "/products/BTC-USD/candles?start=1&end=61&granularity=ONE_MINUTE&limit=2",
		s.config.CoinbaseExchangeURL + "/products/BTC-USD/candles?start=2026-10-06T00:00:00Z&end=2026-10-06T00:01:00Z&granularity=60",
		s.config.CoinGeckoCoinURL + "/bitcoin/ohlc?vs_currency=usd&days=1&precision=full",
		s.config.FXURL + "?date=2026-10-05",
	} {
		var value map[string]interface{}
		if err := s.getJSON(context.Background(), address, nil, &value); err != nil {
			t.Fatalf("history route: %s: %v", address, err)
		}
	}
	if direct.Load() != 0 || upstream.Load() != 4 {
		t.Fatalf("direct=%d relay=%d", direct.Load(), upstream.Load())
	}
	for _, path := range []string{"/v1/coinbase-exchange/accounts", "/v1/coinbase-exchange/products/BTC-USD/candles?url=https://evil.example", "/v1/coingecko/coins/bitcoin/market_chart", "/v1/fx/usd/cny?base=USD"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(marketrelay.TokenHeader, testRelayToken)
		recorder := httptest.NewRecorder()
		relay.ServeHTTP(recorder, req)
		if recorder.Code != 400 || strings.Contains(recorder.Body.String(), testRelayToken) {
			t.Errorf("invalid history request: %s", path)
		}
	}
	if upstream.Load() != 4 {
		t.Error("unsupported request reached upstream")
	}
}

func TestFailedFundRetriesAfterForegroundLeaseExpires(t *testing.T) {
	var now, calls atomic.Int64
	now.Store(1791324000)
	s := New(Config{Now: func() time.Time { return time.Unix(now.Load(), 0) }, HTTPClient: &http.Client{Transport: routeTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("temporary fund outage")
	})}})
	s.Register(Binding{Market: "CN_FUND", Provider: "eastmoney", ProviderID: "000001", Currency: "CNY"})
	s.MarkActive()
	s.refreshReferenceProvider(context.Background(), "eastmoney")
	now.Add(120) // The 90-second foreground lease has expired.
	s.refreshReferenceProvider(context.Background(), "eastmoney")
	if calls.Load() != 2 {
		t.Fatalf("fund recovery suppressed by idle budget: %d requests", calls.Load())
	}
}
