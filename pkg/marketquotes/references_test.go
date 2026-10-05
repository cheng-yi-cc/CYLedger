package marketquotes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func securityFixture(code, price, at, currency string) string {
	fields := make([]string, 88)
	fields[0], fields[1], fields[2], fields[3], fields[30], fields[32], fields[35] = "1", "供应商资产", code, price, at, "-2.125", currency
	fields[75], fields[82] = currency, currency
	return strings.Join(fields, "~")
}

func TestReferenceSecurityIdentityAndTimeZones(t *testing.T) {
	now := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		binding  Binding
		code, at string
		expected time.Time
	}{
		{Binding{"CN_SH", "tencent", "sh600519", "CNY"}, "600519", "20260921150000", time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC)},
		{Binding{"HK", "tencent", "hk00700", "HKD"}, "00700", "2026/09/21 16:08:36", time.Date(2026, 9, 21, 8, 8, 36, 0, time.UTC)},
		{Binding{"HK", "tencent", "hk80700", "CNY"}, "80700", "2026/09/21 16:08:36", time.Date(2026, 9, 21, 8, 8, 36, 0, time.UTC)},
		{Binding{"HK", "tencent", "hk09046", "USD"}, "09046", "2026/09/21 16:08:36", time.Date(2026, 9, 21, 8, 8, 36, 0, time.UTC)},
		{Binding{"US", "tencent", "usBRK.B", "USD"}, "BRK.B.N", "2026-09-18 16:05:15", time.Date(2026, 9, 18, 20, 5, 15, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.binding.Market, func(t *testing.T) {
			q, err := parseTencent(tt.binding, securityFixture(tt.code, "123.123456789012345678", tt.at, tt.binding.Currency), now)
			if err != nil || q.SourceTime != tt.expected.Unix() || q.Price != "123.123456789012345678" || q.ChangePercent == nil || *q.ChangePercent != "-2.125" || q.ChangePeriod != "session" {
				t.Fatalf("unexpected reference: %+v %v", q, err)
			}
			if _, err = parseTencent(tt.binding, securityFixture("DIFFERENT", "1", tt.at, tt.binding.Currency), now); err == nil {
				t.Fatal("accepted another security's quote")
			}
			wrongCurrency := "USD"
			if tt.binding.Currency == "USD" {
				wrongCurrency = "CNY"
			}
			if _, err = parseTencent(tt.binding, securityFixture(tt.code, "1", tt.at, wrongCurrency), now); err == nil {
				t.Fatal("accepted a different quote currency")
			}
		})
	}
	if usTicker("brk.b.n") != "BRK.B" {
		t.Fatal("US class suffix lost")
	}
	if signedPercent("") != nil || signedPercent("null") != nil || signedPercent("1e9999999") != nil || signedPercent("0") == nil {
		t.Fatal("unknown and zero percentage conflated")
	}
}

func TestReferenceHongKongCounterCurrencyComesFromProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			_, _ = w.Write([]byte(`v_hint="hk~00700~Tencent~tx~GP^hk~80700~Tencent RMB~txr~GP^hk~09046~Ether ETF USD~eth~GP"`))
			return
		}
		for _, counter := range []struct{ code, currency string }{{"00700", "HKD"}, {"80700", "CNY"}, {"09046", "USD"}} {
			fields := strings.Split(securityFixture(counter.code, "100", "2026/09/21 16:08:36", counter.currency), "~")
			if counter.currency == "USD" {
				fields[63] = "GP-FUND"
			}
			value, _ := json.Marshal(strings.Join(fields, "~"))
			_, _ = fmt.Fprintf(w, "v_hk%s=%s;", counter.code, value)
		}
	}))
	defer server.Close()
	s := New(Config{TencentURL: server.URL + "/quote", TencentSearchURL: server.URL + "/search"})
	items, err := s.Search(context.Background(), "counter", "HK")
	if err != nil || len(items) != 3 {
		t.Fatalf("counter search failed: %+v %v", items, err)
	}
	expected := map[string]string{"hk00700": "HKD", "hk80700": "CNY", "hk09046": "USD"}
	for _, item := range items {
		if item.Currency != expected[item.ProviderID] {
			t.Fatalf("counter currency inferred from exchange: %+v", item)
		}
		if item.ProviderID == "hk09046" && item.Type != "FUND" {
			t.Fatal("Hong Kong ETF was classified as a stock")
		}
	}
}

func TestReferenceSearchBindingAndFailureKeepsPrice(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)}
	var fail atomic.Bool
	var quoteRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`v_hint="sh~600519~\u8d35\u5dde\u8305\u53f0~gzmt~GP-A^us~brk.b.n~Berkshire~b~GP^sh~000001~Index~i~ZS"`))
		case "/quote":
			quoteRequests.Add(1)
			if fail.Load() {
				http.Error(w, "limited", 429)
				return
			}
			if r.URL.Query().Get("q") != "sh600519" {
				t.Error("request contains unexpected/nonpublic identifier")
			}
			value, _ := json.Marshal(securityFixture("600519", "1252.570", "20260921150000", "CNY"))
			encoded, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("v_sh600519=" + string(value) + ";"))
			_, _ = w.Write(encoded)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s := New(Config{TencentURL: server.URL + "/quote", TencentSearchURL: server.URL + "/search", Now: clock.Now})
	items, err := s.Search(context.Background(), "600519", "CN")
	if err != nil || len(items) != 1 || items[0].ProviderID != "sh600519" || items[0].Name != "贵州茅台" {
		t.Fatalf("wrong search identities: %+v %v", items, err)
	}
	confirmed, err := s.Resolve(context.Background(), items[0].Binding)
	if err != nil || confirmed == nil {
		t.Fatalf("binding failed: %v", err)
	}
	first, ok := s.Get(confirmed.Key())
	if !ok || first.Price != "1252.57" || first.State != StateDelayed {
		t.Fatalf("wrong cached quote: %+v", first)
	}
	fail.Store(true)
	clock.Advance(time.Minute)
	s.refreshReferences(context.Background())
	old, _ := s.Get(confirmed.Key())
	if old.Price != first.Price || old.ReceivedAt != first.ReceivedAt || old.SourceTime != first.SourceTime {
		t.Fatal("failure changed cached observation")
	}
	before := quoteRequests.Load()
	s.refreshReferences(context.Background())
	s.Quotes()
	s.Get(confirmed.Key())
	if quoteRequests.Load() != before {
		t.Fatal("read/retry exceeded shared refresh budget")
	}
	clock.Advance(11 * time.Minute)
	stale, _ := s.Get(confirmed.Key())
	if stale.State != StateStale {
		t.Fatal("missed stale retrieval")
	}
	restored := New(Config{Now: clock.Now})
	if err = restored.Register(confirmed.Binding); err != nil {
		t.Fatal(err)
	}
	restored.Restore([]Quote{first}, nil)
	q, _ := restored.Get(confirmed.Key())
	if q.State != StateStale || q.SourceTime != first.SourceTime {
		t.Fatal("restore reset provenance")
	}
}

func TestReferenceFundRejectsMoneyYield(t *testing.T) {
	var money atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://fundf10.eastmoney.com/" {
			t.Error("missing NAV referer")
		}
		typ, yield := "002", ""
		if money.Load() {
			typ, yield = "005", "每万份收益"
		}
		writeJSON(w, map[string]any{"ErrCode": 0, "Data": map[string]any{"FundType": typ, "SYType": yield, "LSJZList": []map[string]string{{"FSRQ": "2026-09-18", "DWJZ": "1.3330", "JZZZL": "-0.50"}}}})
	}))
	defer server.Close()
	s := New(Config{FundNAVURL: server.URL, Now: func() time.Time { return time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC) }})
	b := Binding{"CN_FUND", "eastmoney", "000001", "CNY"}
	q, err := s.fetchFund(context.Background(), b)
	if err != nil || q.Price != "1.333" || q.ChangePeriod != "nav" || q.ChangePercent == nil || *q.ChangePercent != "-0.5" {
		t.Fatalf("invalid NAV: %+v %v", q, err)
	}
	money.Store(true)
	if _, err = s.fetchFund(context.Background(), b); err == nil {
		t.Fatal("money-market yield used as unit NAV")
	}
}

func TestReferenceCryptoFallbackAndExplicitIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			http.Error(w, "blocked", 429)
			return
		}
		writeJSON(w, map[string]any{"products": []map[string]string{{"product_id": "DOGE-USD", "base_currency_id": "DOGE", "quote_currency_id": "USD", "base_name": "Dogecoin", "product_type": "SPOT", "status": "online"}, {"product_id": "DOGE-USDC", "base_currency_id": "DOGE", "quote_currency_id": "USDC", "base_name": "Dogecoin", "product_type": "SPOT", "status": "online"}, {"product_id": "DOGE-PERP", "base_currency_id": "DOGE", "quote_currency_id": "USD", "base_name": "Dogecoin", "product_type": "FUTURE", "status": "online"}}})
	}))
	defer server.Close()
	s := New(Config{CoinGeckoSearchURL: server.URL + "/search", CoinbaseRESTURL: server.URL})
	items, err := s.Search(context.Background(), "DOGE", "CRYPTO")
	if err != nil || len(items) != 1 || items[0].ProviderID != "DOGE-USD" || items[0].Currency != "USD" {
		t.Fatalf("fallback included mismatched pair: %+v %v", items, err)
	}
	if (Binding{"US", "tencent", "usAAPL", "HKD"}).Valid() || (Binding{"CN_SH", "tencent", "sz000001", "CNY"}).Valid() {
		t.Fatal("market/currency mismatch accepted")
	}
}

func TestReferenceQuoteRejectsIdentityAndTimeRegression(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)}
	s := New(Config{Now: clock.Now})
	b := Binding{"HK", "tencent", "hk00700", "HKD"}
	if err := s.Register(b); err != nil {
		t.Fatal(err)
	}
	change := "2.50"
	first := Quote{InstrumentID: b.Key(), Price: "430", Currency: "HKD", Source: SourceTencent, SourceTime: clock.Now().Add(-time.Minute).Unix(), ReceivedAt: clock.Now().Unix(), ChangePercent: &change, ChangePeriod: "session"}
	if !s.putReferenceQuote(first) {
		t.Fatal("initial verified reference rejected")
	}
	for _, test := range []struct {
		name string
		edit func(*Quote)
	}{
		{"wrong currency", func(q *Quote) { q.Currency = "CNY" }},
		{"wrong provider", func(q *Quote) { q.Source = SourceFundNAV }},
		{"another asset", func(q *Quote) { q.InstrumentID = "market:tencent:hk00005" }},
		{"older source", func(q *Quote) { q.SourceTime-- }},
		{"older retrieval", func(q *Quote) { q.ReceivedAt-- }},
		{"future source", func(q *Quote) { q.SourceTime = clock.Now().Add(3 * time.Minute).Unix() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := first
			bad.Price = "1"
			test.edit(&bad)
			if s.putReferenceQuote(bad) {
				t.Fatal("invalid quote replaced the reference")
			}
			got, _ := s.Get(b.Key())
			if got.Price != "430" || got.SourceTime != first.SourceTime || got.ReceivedAt != first.ReceivedAt {
				t.Fatal("rejection changed the cached observation")
			}
		})
	}
	// Both inserted and restored optional percentages must own their memory and
	// preserve the supplier's period, never an unvalidated value from a backup.
	change = "999"
	got, _ := s.Get(b.Key())
	if got.ChangePercent == nil || *got.ChangePercent != "2.5" {
		t.Fatal("caller mutated stored percentage")
	}
	badChange := "1e99999999"
	first.ChangePercent = &badChange
	restored := New(Config{Now: clock.Now})
	_ = restored.Register(b)
	restored.Restore([]Quote{first}, nil)
	got, _ = restored.Get(b.Key())
	if got.ChangePercent != nil || got.ChangePeriod != "" || got.State != StateStale {
		t.Fatal("restore kept invalid optional metadata")
	}
	if !restored.putReferenceQuote(Quote{InstrumentID: b.Key(), Price: "430", Currency: "HKD", Source: SourceTencent, SourceTime: first.SourceTime, ReceivedAt: first.ReceivedAt}) {
		t.Fatal("unchanged fresh supplier response could not clear restored state")
	}
	got, _ = restored.Get(b.Key())
	if got.State != StateDelayed {
		t.Fatal("confirmed current observation remained restored")
	}
}

func TestReferenceCoinGeckoSharesPresetRefreshBudget(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)}
	var requests atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		counts := map[string]int{}
		for _, id := range ids {
			counts[id]++
		}
		if counts["bitcoin"] != 1 || counts["dogecoin"] != 1 || len(ids) != 6 {
			t.Error("custom and preset public identifiers were not deduplicated into one batch")
		}
		if fail.Load() {
			http.Error(w, "limited", http.StatusTooManyRequests)
			return
		}
		writeJSON(w, map[string]any{"bitcoin": map[string]any{"usd": json.Number("60000"), "last_updated_at": clock.Now().Unix()}, "dogecoin": map[string]any{"usd": json.Number("0.12"), "last_updated_at": clock.Now().Unix()}})
	}))
	defer server.Close()
	s := New(Config{Now: clock.Now, CoinGeckoURL: server.URL, CoinGeckoAPIKey: "test-key"})
	for _, id := range []string{"bitcoin", "dogecoin"} {
		if err := s.Register(Binding{"CRYPTO", "coingecko", id, "USD"}); err != nil {
			t.Fatal(err)
		}
	}
	s.refreshReferences(context.Background())
	s.refreshCoinGecko(context.Background())
	s.refreshCoinGecko(context.Background())
	if requests.Load() != 1 {
		t.Fatal("periodic loops spent separate provider budgets")
	}
	for _, id := range []string{"crypto:bitcoin", "market:coingecko:bitcoin", "market:coingecko:dogecoin"} {
		if _, ok := s.Get(id); !ok {
			t.Errorf("shared batch did not populate %s", id)
		}
	}
	first, _ := s.Get("market:coingecko:dogecoin")
	fail.Store(true)
	clock.Advance(15 * time.Minute)
	s.refreshCoinGecko(context.Background())
	s.refreshCoinGecko(context.Background())
	got, _ := s.Get("market:coingecko:dogecoin")
	if requests.Load() != 2 || got.Price != first.Price || got.ReceivedAt != first.ReceivedAt {
		t.Fatal("failed batch reset provenance or escaped the attempt budget")
	}
}

func TestReferenceHKDFXPreservesObservationOnFailure(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)}
	date, base, rate := "2026-09-21", "HKD", json.Number("0.9")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"date": date, "base": base, "quote": "CNY", "rate": rate})
	}))
	defer server.Close()
	s := New(Config{Now: clock.Now, HKDFXURL: server.URL})
	if err := s.refreshHKDFX(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := s.FX()[1]
	clock.Advance(time.Hour)
	for _, test := range []struct{ date, base, rate string }{{"2026-09-20", "HKD", "0.1"}, {"2026-09-21", "USD", "7"}, {"2026-09-22", "HKD", "0.9"}, {"2026-09-21", "HKD", "0"}} {
		date, base, rate = test.date, test.base, json.Number(test.rate)
		if err := s.refreshHKDFX(context.Background()); err == nil {
			t.Fatal("invalid HKD reference accepted")
		}
		got := s.FX()[1]
		if got != first {
			t.Fatal("invalid response changed the previous FX observation")
		}
	}
	restored := New(Config{Now: clock.Now, HKDFXURL: server.URL})
	restored.Restore(nil, []FXRate{first})
	if restored.FX()[1].State != StateStale {
		t.Fatal("restored FX was presented as newly fetched")
	}
	date, base, rate = "2026-09-21", "HKD", json.Number("0.9")
	if err := restored.refreshHKDFX(context.Background()); err != nil || restored.FX()[1].State != StateDelayed {
		t.Fatalf("current provider reference failed to clear restored state: %v", err)
	}
}

func TestLiveReferenceSearchAndQuotes(t *testing.T) {
	if os.Getenv("CYLEDGER_LIVE_MARKET_TEST") != "1" {
		t.Skip("set CYLEDGER_LIVE_MARKET_TEST=1 to query public providers")
	}
	s := New(Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	for _, test := range []struct{ query, market, providerID string }{{"600519", "CN", "sh600519"}, {"510300", "CN", "sh510300"}, {"00700", "HK", "hk00700"}, {"80700", "HK", "hk80700"}, {"09046", "HK", "hk09046"}, {"AAPL", "US", "usAAPL"}, {"000001", "CN_FUND", "000001"}, {"DOGE", "CRYPTO", "DOGE-USD"}} {
		items, err := s.Search(ctx, test.query, test.market)
		if err != nil {
			t.Fatal(err)
		}
		var selected *Candidate
		for _, item := range items {
			if item.ProviderID == test.providerID {
				copy := item
				selected = &copy
				break
			}
		}
		if selected == nil {
			t.Fatalf("missing exact public identity %s", test.providerID)
		}
		confirmed, err := s.Resolve(ctx, selected.Binding)
		if err != nil {
			t.Fatal(err)
		}
		q, ok := s.Get(confirmed.Key())
		if !ok || q.Price == "" || q.SourceTime <= 0 {
			t.Fatalf("missing verified quote: %+v", q)
		}
		t.Log(fmt.Sprintf("%s %s source=%s state=%s", test.providerID, q.Currency, q.Source, q.State))
	}
}
