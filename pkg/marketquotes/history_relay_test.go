package marketquotes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/marketrelay"
	"github.com/stretchr/testify/require"
)

func historyRelayService(t *testing.T, now time.Time, upstream func(*http.Request) (string, error)) *Service {
	t.Helper()
	relay, err := marketrelay.New(marketrelay.Config{Token: testRelayToken, Client: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get(marketrelay.TokenHeader) != "" {
			t.Error("relay credential reached a public provider")
		}
		body, err := upstream(r)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}})
	require.NoError(t, err)
	server := httptest.NewServer(relay)
	t.Cleanup(server.Close)
	return New(Config{Now: func() time.Time { return now }, Network: NetworkConfig{RelayURL: server.URL, RelayToken: testRelayToken}, DomesticHTTPClient: server.Client(), HTTPClient: &http.Client{Transport: routeTransport(func(r *http.Request) (*http.Response, error) {
		t.Errorf("historical request bypassed relay: %s", r.URL.Host)
		return nil, errors.New("direct overseas access unavailable")
	})}})
}

func TestDayReferenceAndFallbacksUseRelay(t *testing.T) {
	at := time.Date(2026, 10, 6, 16, 0, 0, 0, time.UTC)
	for _, source := range []string{"coinbase", "exchange", "coingecko"} {
		t.Run(source, func(t *testing.T) {
			var calls atomic.Int32
			s := historyRelayService(t, at.Add(time.Hour), func(r *http.Request) (string, error) {
				calls.Add(1)
				switch r.URL.Host {
				case "api.coinbase.com":
					if source != "coinbase" {
						return "", errors.New("primary unavailable")
					}
					if r.URL.Path != "/api/v3/brokerage/market/products/BTC-USD/candles" || r.URL.Query().Get("start") != fmt.Sprint(at.Unix()-60) {
						t.Error("lost primary historical path or minute")
					}
					return fmt.Sprintf(`{"candles":[{"start":"%d","close":"65000.123456789"}]}`, at.Unix()-60), nil
				case "api.exchange.coinbase.com":
					if source != "exchange" {
						return "", errors.New("exchange unavailable")
					}
					if r.URL.Path != "/products/BTC-USD/candles" || r.URL.Query().Get("start") != at.Add(-time.Minute).Format(time.RFC3339) {
						t.Error("lost exchange historical path or minute")
					}
					return fmt.Sprintf(`[[%d,64000,66000,64500,65000.123456789,1]]`, at.Unix()-60), nil
				case "api.coingecko.com":
					if r.URL.Path != "/api/v3/coins/bitcoin/ohlc" || r.URL.Query().Get("precision") != "full" {
						t.Error("lost fallback identity or precision")
					}
					return fmt.Sprintf(`[[%d,64500,66000,64000,65000.123456789]]`, at.Unix()*1000), nil
				case "api.frankfurter.dev":
					if r.URL.Path != "/v2/providers/ecb/rate/usd/cny" || r.URL.Query().Get("date") != "2026-10-06" {
						t.Error("lost historical FX date")
					}
					return `{"date":"2026-10-06","base":"USD","quote":"CNY","rate":7.0123456789}`, nil
				default:
					return "", errors.New("unexpected provider")
				}
			})
			quote, err := s.fetchDayReference(context.Background(), "crypto:bitcoin", at.Unix())
			require.NoError(t, err)
			require.Equal(t, "65000.123456789", quote.Price)
			require.Equal(t, "7.0123456789", quote.FXRate)
			require.Equal(t, "2026-10-06", quote.FXDate)
			require.Equal(t, map[string]int32{"coinbase": 2, "exchange": 3, "coingecko": 4}[source], calls.Load())
		})
	}
}

func TestCryptoDCAHistoryUsesRelay(t *testing.T) {
	at := time.Date(2026, 10, 7, 4, 4, 0, 0, time.UTC)
	var calls atomic.Int32
	s := historyRelayService(t, at.Add(time.Hour), func(r *http.Request) (string, error) {
		calls.Add(1)
		switch r.URL.Host {
		case "www.bitstamp.net":
			pair, price := "BTC/USD", "65000.123456789123456789"
			switch r.URL.Path {
			case "/api/v2/ohlc/usdtusd/":
				pair, price = "USDT/USD", "0.987654321123456789"
			case "/api/v2/ohlc/btcusd/":
			default:
				return "", errors.New("unexpected historical pair")
			}
			if r.URL.Query().Get("start") != fmt.Sprint(at.Unix()) || r.URL.Query().Get("step") != "60" {
				t.Error("lost DCA historical minute")
			}
			return fmt.Sprintf(`{"data":{"pair":%q,"ohlc":[{"timestamp":"%d","open":%q}]}}`, pair, at.Unix(), price), nil
		case "api.frankfurter.dev":
			if r.URL.Path != "/v2/providers/ecb/rates" || r.URL.Query().Get("date") != "2026-10-06" || r.URL.Query().Get("base") != "USD" || r.URL.Query().Get("quotes") != "CNY" {
				t.Error("lost DCA historical FX cutoff or currencies")
			}
			return `[{"date":"2026-10-06","base":"USD","quote":"CNY","rate":7.0123456789}]`, nil
		default:
			return "", errors.New("unexpected provider")
		}
	})
	quote, err := s.HistoricalCryptoQuote(context.Background(), "crypto:tether", "crypto:bitcoin", at.Unix())
	require.NoError(t, err)
	require.Equal(t, "0.987654321123456789", quote.PaymentPrice)
	require.Equal(t, "65000.123456789123456789", quote.TargetPrice)
	require.Equal(t, "7.0123456789", quote.FXRate)
	require.Equal(t, "2026-10-06", quote.FXDate)
	require.Equal(t, int32(3), calls.Load())
}

func TestCoinGeckoFullPrecisionRefreshUsesRelay(t *testing.T) {
	now := time.Date(2026, 10, 7, 6, 0, 0, 0, time.UTC)
	s := historyRelayService(t, now, func(r *http.Request) (string, error) {
		if r.URL.Host != "api.coingecko.com" || r.URL.Path != "/api/v3/simple/price" || r.URL.Query().Get("precision") != "full" {
			return "", errors.New("lost CoinGecko price route or full precision")
		}
		return fmt.Sprintf(`{"bitcoin":{"usd":65000.123456789123456789,"last_updated_at":%d}}`, now.Unix()), nil
	})
	s.refreshCoinGecko(context.Background())
	quote, ok := s.Get("crypto:bitcoin")
	require.True(t, ok)
	require.Equal(t, "65000.123456789123456789", quote.Price)
}

func TestFailedFundRetriesAfterForegroundLeaseExpires(t *testing.T) {
	var now, calls atomic.Int64
	now.Store(time.Date(2026, 10, 7, 6, 0, 0, 0, time.UTC).Unix())
	s := New(Config{Now: func() time.Time { return time.Unix(now.Load(), 0) }, HTTPClient: &http.Client{Transport: routeTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("temporary fund outage")
	})}})
	require.NoError(t, s.Register(Binding{Market: "CN_FUND", Provider: "eastmoney", ProviderID: "000001", Currency: "CNY"}))
	s.MarkActive()
	s.refreshReferenceProvider(context.Background(), "eastmoney")
	now.Add(30)
	s.refreshReferenceProvider(context.Background(), "eastmoney")
	require.Equal(t, int64(1), calls.Load())
	now.Add(90) // The foreground lease has expired; failure recovery still takes one minute.
	s.refreshReferenceProvider(context.Background(), "eastmoney")
	require.Equal(t, int64(2), calls.Load())
}
