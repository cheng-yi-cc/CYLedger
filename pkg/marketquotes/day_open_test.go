package marketquotes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDayReferenceExactClosedMinuteAndHistoricalFX(t *testing.T) {
	at := time.Date(2026, 10, 6, 16, 0, 0, 0, time.UTC).Unix() // Shanghai midnight
	wrong := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/unavailable/") {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		if r.URL.Path == "/fx" {
			if r.URL.Query().Get("date") != "2026-10-06" {
				t.Error("wrong historical fixing date")
			}
			fmt.Fprint(w, `{"date":"2026-10-06","base":"USD","quote":"CNY","rate":7.1}`)
			return
		}
		if r.URL.Path != "/products/BTC-USD/candles" || r.URL.Query().Get("start") != fmt.Sprint(at-60) || r.URL.Query().Get("granularity") != "ONE_MINUTE" {
			t.Error("wrong candle contract")
		}
		start := at - 60
		if wrong {
			start = at - 120
		}
		fmt.Fprintf(w, `{"candles":[{"start":"%d","close":"100.123456789012345678"}]}`, start)
	}))
	defer srv.Close()
	s := New(Config{CoinbaseRESTURL: srv.URL, CoinbaseExchangeURL: srv.URL + "/unavailable", CoinGeckoCoinURL: srv.URL + "/unavailable", FXURL: srv.URL + "/fx"})
	q, err := s.fetchDayReference(context.Background(), "crypto:bitcoin", at)
	if err != nil || q.Price != "100.123456789012345678" || q.FXRate != "7.1" || q.SourceTime != at-60 {
		t.Fatalf("reference=%+v error=%v", q, err)
	}
	wrong = true
	if _, err = s.fetchDayReference(context.Background(), "crypto:bitcoin", at); err == nil {
		t.Fatal("accepted nearby minute instead of midnight")
	}
}

func TestDayReferenceExchangeFallbackKeepsExactDecimals(t *testing.T) {
	at := time.Date(2026, 10, 6, 16, 0, 0, 0, time.UTC).Unix()
	wrong := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/BTC-USD/candles":
			http.Error(w, "unavailable", http.StatusBadGateway)
		case "/exchange/products/BTC-USD/candles":
			if r.URL.Query().Get("start") != "2026-10-06T15:59:00Z" || r.URL.Query().Get("granularity") != "60" {
				t.Error("wrong Exchange candle contract")
			}
			start := at - 60
			if wrong {
				start -= 60
			}
			fmt.Fprintf(w, `[[%d,99,102,101,100.123456789012345678,12]]`, start)
		case "/fx":
			fmt.Fprint(w, `{"date":"2026-10-06","base":"USD","quote":"CNY","rate":7.1}`)
		case "/coins/bitcoin/ohlc":
			http.Error(w, "unavailable", http.StatusBadGateway)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := New(Config{CoinbaseRESTURL: srv.URL, CoinbaseExchangeURL: srv.URL + "/exchange", CoinGeckoCoinURL: srv.URL + "/coins", FXURL: srv.URL + "/fx"})
	q, err := s.fetchDayReference(context.Background(), "crypto:bitcoin", at)
	if err != nil || q.Price != "100.123456789012345678" {
		t.Fatalf("reference=%+v error=%v", q, err)
	}
	wrong = true
	if _, err = s.fetchDayReference(context.Background(), "crypto:bitcoin", at); err == nil {
		t.Fatal("accepted a nearby Exchange minute")
	}
}
