package marketquotes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *testClock) Advance(delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	c.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func validProducts() string {
	return `{"products":[{"product_id":"BTC-USD","base_currency_id":"BTC","quote_currency_id":"USD","product_type":"SPOT","status":"online"},{"product_id":"ETH-USD","base_currency_id":"ETH","quote_currency_id":"USD","product_type":"SPOT","status":"online"},{"product_id":"USDC-USD","base_currency_id":"BTC","quote_currency_id":"USD","product_type":"SPOT","status":"online"},{"product_id":"SOL-USD","base_currency_id":"SOL","quote_currency_id":"USD","product_type":"SPOT","status":"offline"}]}`
}

func TestVerifiedMappingRESTAndStaleCache(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "CYLedger/") {
			t.Error("missing application User-Agent")
		}
		switch r.URL.Path {
		case "/products":
			if len(r.URL.Query()["product_ids"]) != 5 {
				t.Error("expected five explicit product candidates")
			}
			_, _ = w.Write([]byte(validProducts()))
		case "/products/BTC-USD/ticker", "/products/ETH-USD/ticker":
			product := strings.Split(r.URL.Path, "/")[2]
			writeJSON(w, map[string]interface{}{"trades": []map[string]string{{"product_id": product, "price": "123.123456789012345678", "time": clock.Now().Add(-time.Second).Format(time.RFC3339Nano)}}})
		default:
			t.Errorf("unverified product queried: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s := New(Config{CoinbaseRESTURL: server.URL, Now: clock.Now})
	if err := s.verifyProducts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if products := strings.Join(s.verifiedProducts(), ","); products != "BTC-USD,ETH-USD" {
		t.Fatalf("incorrect verified products %s", products)
	}
	s.refreshREST(context.Background())
	quote, ok := s.Get("crypto:bitcoin")
	if !ok || quote.Price != "123.123456789012345678" || quote.State != StateDelayed || quote.Source != SourceCoinbaseREST || quote.Connected {
		t.Fatalf("wrong REST price: %+v", quote)
	}
	if quote.SourceTime != clock.Now().Add(-time.Second).Unix() || quote.ReceivedAt != clock.Now().Unix() {
		t.Fatalf("source and receipt times conflated: %+v", quote)
	}
	if _, ok := s.Get("crypto:usd-coin"); ok {
		t.Fatal("missing stablecoin should not become a $1 quote")
	}
	quotes := s.Quotes()
	if len(quotes) != 5 || quotes[4].State != StateUnavailable || quotes[4].Price != "" {
		t.Fatalf("missing quote not explicit: %+v", quotes)
	}
	for i := 0; i < 10; i++ {
		s.Quotes()
		s.FX()
		s.Get("crypto:bitcoin")
	}
	if calls.Load() != 3 {
		t.Fatalf("cache reads triggered upstream requests: %d", calls.Load())
	}
	clock.Advance(3 * time.Minute)
	stale, _ := s.Get("crypto:bitcoin")
	if stale.State != StateStale || stale.Price != quote.Price || stale.ReceivedAt != quote.ReceivedAt {
		t.Fatalf("stale data lost its provenance: %+v", stale)
	}
	quotes[0].Price = "1"
	unchanged, _ := s.Get("crypto:bitcoin")
	if unchanged.Price != quote.Price {
		t.Fatal("returned quote aliases shared cache")
	}
}

func tickerFrame(sequence int64, at time.Time, product, price string) map[string]interface{} {
	return map[string]interface{}{"channel": "ticker_batch", "timestamp": at.Format(time.RFC3339Nano), "sequence_num": sequence, "events": []map[string]interface{}{{"type": "snapshot", "tickers": []map[string]string{{"product_id": product, "price": price}}}}}
}

func heartbeatFrame(sequence, counter int64, at time.Time) map[string]interface{} {
	return map[string]interface{}{"channel": "heartbeats", "timestamp": at.Format(time.RFC3339Nano), "sequence_num": sequence, "events": []map[string]interface{}{{"heartbeat_counter": counter}}}
}

func eventually(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached before timeout")
}

func TestWebSocketSubscriptionOrderingAndCancellation(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)}
	var subscribed atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(validProducts())) })
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		defer conn.Close()
		var ticker, heartbeat struct {
			Type     string   `json:"type"`
			Channel  string   `json:"channel"`
			Products []string `json:"product_ids"`
		}
		if websocket.JSON.Receive(conn, &ticker) != nil || websocket.JSON.Receive(conn, &heartbeat) != nil {
			return
		}
		if ticker.Type != "subscribe" || ticker.Channel != "ticker_batch" || strings.Join(ticker.Products, ",") != "BTC-USD,ETH-USD" || heartbeat.Channel != "heartbeats" {
			t.Errorf("wrong public subscription: %+v %+v", ticker, heartbeat)
			return
		}
		frames := []interface{}{
			tickerFrame(0, clock.Now(), "BTC-USD", "64000.000000000000000001"),
			heartbeatFrame(1, 100, clock.Now()),
			tickerFrame(0, clock.Now().Add(time.Second), "BTC-USD", "1"),  // repeated sequence
			tickerFrame(2, clock.Now().Add(-time.Second), "BTC-USD", "2"), // older source timestamp
			tickerFrame(3, clock.Now(), "USDC-USD", "1"),                  // not verified
			tickerFrame(4, clock.Now(), "ETH-USD", "-5"),                  // invalid price
			heartbeatFrame(5, 101, clock.Now()),
		}
		for _, frame := range frames {
			if websocket.JSON.Send(conn, frame) != nil {
				return
			}
		}
		subscribed.Store(true)
		var ignored interface{}
		_ = websocket.JSON.Receive(conn, &ignored)
	}))
	server := httptest.NewServer(mux)
	defer server.Close()
	s := New(Config{CoinbaseRESTURL: server.URL, CoinbaseWSURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/ws", Now: clock.Now})
	if err := s.verifyProducts(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.stream(ctx) }()
	eventually(t, time.Second, func() bool { quote, ok := s.Get("crypto:bitcoin"); return subscribed.Load() && ok && quote.Connected })
	quote, _ := s.Get("crypto:bitcoin")
	if quote.Price != "64000.000000000000000001" || quote.State != StateLive {
		t.Fatalf("stale envelope overwrote quote: %+v", quote)
	}
	if _, ok := s.Get("crypto:usd-coin"); ok {
		t.Fatal("unverified websocket instrument accepted")
	}
	if _, ok := s.Get("crypto:ethereum"); ok {
		t.Fatal("negative price accepted")
	}
	// A current heartbeat keeps an unchanged ticker live without pretending the
	// supplier sent another price or changing either stored timestamp.
	clock.Advance(5 * time.Minute)
	s.mu.Lock()
	s.heartbeatAt = clock.Now()
	s.mu.Unlock()
	unchanged, _ := s.Get("crypto:bitcoin")
	if unchanged.State != StateLive || unchanged.SourceTime != quote.SourceTime || unchanged.ReceivedAt != quote.ReceivedAt {
		t.Fatalf("unchanged price misrepresented: %+v", unchanged)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close websocket")
	}
}

func TestHeartbeatReadTimeoutReconnectsDespiteTickerTraffic(t *testing.T) {
	var connections atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(validProducts())) })
	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		product := strings.Split(r.URL.Path, "/")[2]
		writeJSON(w, map[string]interface{}{"trades": []map[string]string{{"product_id": product, "price": "100", "time": time.Now().Format(time.RFC3339Nano)}}})
	})
	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		defer conn.Close()
		var ignored interface{}
		if websocket.JSON.Receive(conn, &ignored) != nil || websocket.JSON.Receive(conn, &ignored) != nil {
			return
		}
		number := connections.Add(1)
		_ = websocket.JSON.Send(conn, tickerFrame(0, time.Now(), "BTC-USD", fmt.Sprint(100+number)))
		_ = websocket.JSON.Send(conn, heartbeatFrame(1, 1, time.Now()))
		for sequence := int64(2); ; sequence++ {
			time.Sleep(10 * time.Millisecond)
			var frame interface{} = tickerFrame(sequence, time.Now(), "BTC-USD", fmt.Sprint(100+number))
			if number > 1 {
				frame = heartbeatFrame(sequence, sequence, time.Now())
			}
			if websocket.JSON.Send(conn, frame) != nil {
				return
			}
		}
	}))
	server := httptest.NewServer(mux)
	defer server.Close()
	s := New(Config{CoinbaseRESTURL: server.URL, CoinbaseWSURL: "ws" + strings.TrimPrefix(server.URL, "http") + "/ws", ReadTimeout: 90 * time.Millisecond, ReconnectMin: 10 * time.Millisecond, ReconnectMax: 20 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.coinbaseLoop(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	eventually(t, 2*time.Second, func() bool {
		quote, ok := s.Get("crypto:bitcoin")
		return connections.Load() >= 2 && ok && quote.Price == "102" && quote.Connected
	})
}

func TestCoinGeckoOptionalKeySharedBudgetAndExactNumbers(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)}
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("x-cg-demo-api-key") != "demo-test-key" {
			t.Error("Demo key missing from backend header")
		}
		if r.URL.Query().Get("ids") != "bitcoin,ethereum,solana,tether,usd-coin" || r.URL.Query().Get("vs_currencies") != "usd" {
			t.Error("unexpected or private data sent to CoinGecko")
		}
		if fail.Load() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = fmt.Fprintf(w, `{"bitcoin":{"usd":12345.678901234567890123,"last_updated_at":%d},"usd-coin":{"usd":0.998765432109876543,"last_updated_at":%d}}`, clock.Now().Add(-time.Minute).Unix(), clock.Now().Add(-time.Minute).Unix())
	}))
	defer server.Close()
	s := New(Config{CoinGeckoURL: server.URL, CoinGeckoAPIKey: "demo-test-key", CoinGeckoInterval: time.Second, Now: clock.Now})
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() { defer group.Done(); s.refreshCoinGecko(context.Background()) }()
	}
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("shared call budget bypassed: %d", calls.Load())
	}
	quote, ok := s.Get("crypto:bitcoin")
	if !ok || quote.Price != "12345.678901234567890123" || quote.State != StateDelayed || quote.Source != SourceCoinGecko {
		t.Fatalf("CoinGecko decimal lost precision: %+v", quote)
	}
	stablecoin, _ := s.Get("crypto:usd-coin")
	if stablecoin.Price != "0.998765432109876543" {
		t.Fatal("stablecoin price was fixed or rounded")
	}
	clock.Advance(14 * time.Minute)
	s.refreshCoinGecko(context.Background())
	if calls.Load() != 1 {
		t.Fatal("requested CoinGecko before fifteen-minute boundary")
	}
	fail.Store(true)
	clock.Advance(time.Minute)
	s.refreshCoinGecko(context.Background())
	s.refreshCoinGecko(context.Background())
	if calls.Load() != 2 {
		t.Fatal("failed requests did not count toward the budget")
	}
	clock.Advance(16 * time.Minute)
	stale, _ := s.Get("crypto:bitcoin")
	if stale.State != StateStale || stale.ReceivedAt != quote.ReceivedAt || stale.SourceTime != quote.SourceTime || stale.Price != quote.Price {
		t.Fatalf("failed fallback pretended to refresh: %+v", stale)
	}
}

func TestCoinGeckoKeylessFallbackForUnavailablePresets(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)}
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("x-cg-demo-api-key") != "" {
			t.Error("keyless fallback sent an unexpected credential")
		}
		if calls.Load() == 1 && r.URL.Query().Get("ids") != "bitcoin" {
			t.Error("healthy primary quotes were unnecessarily requested")
		}
		if fail.Load() {
			http.Error(w, "limited", http.StatusTooManyRequests)
			return
		}
		writeJSON(w, map[string]any{"bitcoin": map[string]any{"usd": json.Number("60000.12345678"), "last_updated_at": clock.Now().Unix()}})
	}))
	defer server.Close()
	s := New(Config{Now: clock.Now, CoinGeckoURL: server.URL})
	primary := func() {
		for _, candidate := range instruments {
			s.putQuote(Quote{InstrumentID: candidate.id, Price: "500", Currency: "USD", Source: SourceCoinbaseREST, ReceivedAt: clock.Now().Unix()}, clock.Now())
		}
	}
	primary()
	s.refreshCoinGecko(context.Background())
	if calls.Load() != 0 {
		t.Fatal("healthy primary feeds used the keyless fallback")
	}
	delete(s.quotes, "crypto:bitcoin")
	s.refreshCoinGecko(context.Background())
	first, ok := s.Get("crypto:bitcoin")
	if !ok || first.Price != "60000.12345678" || first.Source != SourceCoinGecko || first.State != StateDelayed {
		t.Fatalf("missing BTC quote did not use public fallback: %+v", first)
	}
	s.refreshCoinGecko(context.Background())
	clock.Advance(14 * time.Minute)
	s.refreshCoinGecko(context.Background())
	if calls.Load() != 1 {
		t.Fatal("keyless requests escaped the shared fifteen-minute budget")
	}
	fail.Store(true)
	clock.Advance(time.Minute)
	s.refreshCoinGecko(context.Background())
	s.refreshCoinGecko(context.Background())
	old, _ := s.Get("crypto:bitcoin")
	if calls.Load() != 2 || old.Price != first.Price || old.SourceTime != first.SourceTime || old.ReceivedAt != first.ReceivedAt {
		t.Fatal("failed fallback changed the quote or escaped its retry budget")
	}
	clock.Advance(15 * time.Minute)
	primary()
	s.refreshCoinGecko(context.Background())
	recovered, _ := s.Get("crypto:bitcoin")
	if calls.Load() != 2 || recovered.Source != SourceCoinbaseREST || recovered.Price != "500" {
		t.Fatal("recovered primary feed did not replace the fallback")
	}
}

func TestFXReferenceDateFailureAndRestoration(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 20, 13, 0, 0, 0, time.UTC)} // Sunday
	var failure atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failure.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"date":"2026-09-18","base":"USD","quote":"CNY","rate":6.6976}`))
	}))
	defer server.Close()
	s := New(Config{FXURL: server.URL, Now: clock.Now})
	if empty := s.FX()[0]; empty.State != StateUnavailable || empty.Rate != "" {
		t.Fatalf("unavailable FX should not be zero: %+v", empty)
	}
	if err := s.refreshFX(context.Background()); err != nil {
		t.Fatal(err)
	}
	fx := s.FX()[0]
	if fx.Date != "2026-09-18" || fx.State != StateDelayed || fx.Rate != "6.6976" || fx.Source != SourceECB {
		t.Fatalf("daily ECB provenance wrong: %+v", fx)
	}
	restored := New(Config{FXURL: server.URL, Now: clock.Now})
	restored.Restore(nil, []FXRate{fx})
	if restored.FX()[0].State != StateStale {
		t.Fatal("restored FX should require provider refresh")
	}
	if err := restored.refreshFX(context.Background()); err != nil {
		t.Fatal(err)
	}
	if restored.FX()[0].State != StateDelayed {
		t.Fatal("refresh should clear restore marker, including unchanged date")
	}
	failure.Store(true)
	clock.Advance(48 * time.Hour)
	if err := s.refreshFX(context.Background()); err == nil {
		t.Fatal("expected supplier failure")
	}
	failed := s.FX()[0]
	if failed.State != StateStale || failed.ReceivedAt != fx.ReceivedAt || failed.Rate != fx.Rate || failed.Date != fx.Date {
		t.Fatalf("failed FX refresh changed facts: %+v", failed)
	}
}

func TestRestoredPricesRemainStaleAndCannotOverwriteNewerData(t *testing.T) {
	now := time.Now().UTC()
	s := New(Config{})
	quote := Quote{InstrumentID: "crypto:bitcoin", Price: "100", Currency: "USD", Source: SourceCoinbaseWS, SourceTime: now.Unix(), ReceivedAt: now.Unix(), State: StateLive, Connected: true}
	s.Restore([]Quote{quote, {InstrumentID: "private:secret", Price: "100", Currency: "USD", Source: SourceCoinbaseWS, SourceTime: now.Unix(), ReceivedAt: now.Unix()}}, nil)
	if got, ok := s.Get("crypto:bitcoin"); !ok || got.State != StateStale || got.Connected {
		t.Fatalf("restored live flag trusted: %+v", got)
	}
	if _, ok := s.Get("private:secret"); ok {
		t.Fatal("private asset entered public cache")
	}
	newQuote := quote
	newQuote.Price, newQuote.Source, newQuote.State = "101", SourceCoinbaseREST, StateDelayed
	if !s.putQuote(newQuote, now.Add(time.Second)) {
		t.Fatal("newer provider quote rejected")
	}
	s.Restore([]Quote{quote}, nil)
	if got, _ := s.Get("crypto:bitcoin"); got.Price != "101" || got.State != StateDelayed {
		t.Fatalf("restore replaced fresh data: %+v", got)
	}
	for _, price := range []string{"0", "-1", "NaN", "1e999999999", "1e-999999999"} {
		if _, valid := positiveDecimal(price); valid {
			t.Fatalf("invalid price accepted: %s", price)
		}
	}
}

// Opt in explicitly; deterministic tests never depend on internet access.
func TestLivePublicProviders(t *testing.T) {
	if os.Getenv("CYLEDGER_LIVE_MARKET_TEST") != "1" {
		t.Skip("set CYLEDGER_LIVE_MARKET_TEST=1 to probe public market sources")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	s := New(Config{})
	if err := s.Probe(ctx); err != nil {
		t.Fatal(err)
	}
	s.Start(ctx)
	eventually(t, 20*time.Second, func() bool {
		btc, btcOK := s.Get("crypto:bitcoin")
		eth, ethOK := s.Get("crypto:ethereum")
		return btcOK && ethOK && btc.State == StateLive && eth.State == StateLive && btc.Connected && eth.Connected
	})
	for _, quote := range s.Quotes() {
		t.Logf("public quote %s source=%s state=%s sourceTime=%d receivedAt=%d", quote.InstrumentID, quote.Source, quote.State, quote.SourceTime, quote.ReceivedAt)
	}
	fx := s.FX()[0]
	if fx.Rate == "" || fx.Date == "" {
		t.Fatal("live FX missing")
	}
	t.Logf("reference FX source=%s date=%s state=%s", fx.Source, fx.Date, fx.State)
}
