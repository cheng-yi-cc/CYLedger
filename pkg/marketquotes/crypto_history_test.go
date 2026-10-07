package marketquotes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCryptoHistoricalQuoteExactMinuteAndUnpeggedStablecoin(t *testing.T) {
	at := time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/fx" {
			require.Equal(t, "2026-09-27", r.URL.Query().Get("date"))
			fmt.Fprint(w, `[{"date":"2026-09-25","base":"USD","quote":"CNY","rate":7.0123456789}]`)
			return
		}
		require.Equal(t, "60", r.URL.Query().Get("step"))
		require.Equal(t, fmt.Sprint(at.Unix()), r.URL.Query().Get("start"))
		pair, price := "BTC/USD", "65000.123456789123456789"
		if strings.Contains(r.URL.Path, "usdt") {
			pair, price = "USDT/USD", "0.987654321123456789"
		}
		fmt.Fprintf(w, `{"data":{"pair":%q,"ohlc":[{"timestamp":%q,"open":%q}]}}`, pair, fmt.Sprint(at.Unix()), price)
	}))
	defer server.Close()
	s := New(Config{HTTPClient: server.Client(), CryptoHistoryURL: server.URL, HistoricalFXURL: server.URL + "/fx", Now: func() time.Time { return at.Add(time.Hour) }})
	q, err := s.HistoricalCryptoQuote(context.Background(), "crypto:tether", "crypto:bitcoin", at.Unix())
	require.NoError(t, err)
	require.Equal(t, "0.987654321123456789", q.PaymentPrice)
	require.Equal(t, "65000.123456789123456789", q.TargetPrice)
	require.Equal(t, "2026-09-25", q.FXDate)
	_, err = s.HistoricalCryptoQuote(context.Background(), "crypto:tether", "crypto:bitcoin", at.Unix())
	require.NoError(t, err)
	require.Equal(t, 3, calls)
	_, err = s.HistoricalCryptoQuote(context.Background(), "custom:tether", "crypto:bitcoin", at.Unix())
	require.Error(t, err)
	_, err = s.HistoricalCryptoQuote(context.Background(), "crypto:tether", "crypto:bitcoin", at.Add(time.Hour).Unix())
	require.Error(t, err)
	require.Equal(t, 3, calls)
}

func TestCryptoHistoricalQuoteRejectsWrongMinuteIdentityAndDecimals(t *testing.T) {
	at := time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)
	for _, bad := range []string{
		`{"data":{"pair":"USDT/USD","ohlc":[{"timestamp":"1","open":"1"}]}}`,
		fmt.Sprintf(`{"data":{"pair":"BTC/USD","ohlc":[{"timestamp":"%d","open":"1"}]}}`, at.Unix()),
		fmt.Sprintf(`{"data":{"pair":"USDT/USD","ohlc":[{"timestamp":"%d","open":"1e99999999"}]}}`, at.Unix()),
		`{"data":{"pair":"USDT/USD","ohlc":[]}}`,
	} {
		t.Run(bad[:20], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, bad) }))
			defer srv.Close()
			s := New(Config{HTTPClient: srv.Client(), CryptoHistoryURL: srv.URL, Now: func() time.Time { return at.Add(time.Hour) }})
			_, err := s.HistoricalCryptoQuote(context.Background(), "crypto:tether", "crypto:bitcoin", at.Unix())
			require.Error(t, err)
		})
	}
}

func TestLiveCryptoDCAHistory(t *testing.T) {
	if os.Getenv("CYLEDGER_LIVE_MARKET_TEST") != "1" {
		t.Skip("explicit public network opt-in required")
	}
	s := New(Config{})
	at := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Minute).Unix()
	for _, pair := range [][2]string{{"crypto:tether", "crypto:bitcoin"}, {"crypto:usd-coin", "crypto:ethereum"}, {"crypto:tether", "crypto:solana"}} {
		q, err := s.HistoricalCryptoQuote(context.Background(), pair[0], pair[1], at)
		require.NoError(t, err)
		require.Equal(t, at, q.PriceTime)
		require.NotEmpty(t, q.FXDate)
		t.Logf("%s -> %s: public minute %d, FX date %s", pair[0], pair[1], q.PriceTime, q.FXDate)
	}
}
