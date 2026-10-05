package marketquotes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestConversionKeylessFallbackIsBoundedAndKeepsProvenance(t *testing.T) {
	clock := &testClock{now: time.Now()}
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "limited", 429)
			return
		}
		if r.URL.Query().Get("vs_currencies") != "usd" {
			t.Error("unexpected quote currency")
		}
		writeJSON(w, map[string]any{"bitcoin": map[string]any{"usd": json.Number("60000.123"), "last_updated_at": clock.Now().Unix()}, "tether": map[string]any{"usd": json.Number("0.999"), "last_updated_at": clock.Now().Unix()}})
	}))
	defer server.Close()
	s := New(Config{Now: clock.Now, CoinGeckoURL: server.URL})
	for i := 0; i < 2; i++ {
		s.RefreshCryptoConversion(context.Background(), []string{"crypto:bitcoin", "crypto:tether"}, nil)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	q, ok := s.Get("crypto:tether")
	if !ok || q.Price != "0.999" || q.Source != SourceCoinGecko {
		t.Fatalf("unexpected quote: %+v", q)
	}
	clock.Advance(61 * time.Second)
	fail.Store(true)
	s.RefreshCryptoConversion(context.Background(), []string{"crypto:tether"}, nil)
	s.RefreshCryptoConversion(context.Background(), []string{"crypto:tether"}, nil)
	old, _ := s.Get("crypto:tether")
	if calls.Load() != 2 || old.Price != q.Price || old.ReceivedAt != q.ReceivedAt {
		t.Fatal("failed request changed price/time or escaped budget")
	}
}
