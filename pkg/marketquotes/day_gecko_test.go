package marketquotes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGeckoDayReferenceRequiresExactCloseAndKeepsPrecision(t *testing.T) {
	at := time.Date(2026, 10, 6, 16, 0, 0, 0, time.UTC).Unix()
	wrong := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/BTC-USD/candles":
			http.Error(w, "unavailable", http.StatusBadGateway)
		case "/coins/bitcoin/ohlc":
			if r.URL.Query().Get("vs_currency") != "usd" || r.URL.Query().Get("days") != "1" {
				t.Error("wrong history contract")
			}
			closeAt := at
			if wrong {
				closeAt += 1800
			}
			fmt.Fprintf(w, `[[%d,99,102,98,100.123456789012345678]]`, closeAt*1000)
		case "/fx":
			fmt.Fprint(w, `{"date":"2026-10-06","base":"USD","quote":"CNY","rate":7.1}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s := New(Config{CoinbaseRESTURL: server.URL, CoinbaseExchangeURL: server.URL, CoinGeckoCoinURL: server.URL + "/coins", FXURL: server.URL + "/fx", Now: func() time.Time { return time.Unix(at+3600, 0) }})
	q, err := s.fetchDayReference(context.Background(), "crypto:bitcoin", at)
	if err != nil || q.Price != "100.123456789012345678" || q.Period != 1800 || q.SourceTime != at-1800 {
		t.Fatalf("quote=%+v err=%v", q, err)
	}
	restored := New(Config{Now: s.config.Now})
	restored.RestoreDayQuotes([]DayQuote{q})
	if _, ok := restored.DayReference(q.InstrumentID, at); !ok {
		t.Fatal("could not restore valid history")
	}
	wrong = true
	if _, err = s.fetchDayReference(context.Background(), "crypto:bitcoin", at); err == nil {
		t.Fatal("accepted a close after midnight")
	}
}
