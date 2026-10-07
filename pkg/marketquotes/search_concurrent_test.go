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

func TestSlowCryptoSearchDoesNotBlockFundSearch(t *testing.T) {
	started := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fund" {
			_, _ = w.Write([]byte(`{"ErrCode":0,"Datas":[{"CODE":"000001","NAME":"样本基金","CATEGORY":700,"FundBaseInfo":{"FCODE":"000001","FUNDTYPE":"002"}}]}`))
			return
		}
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	s := New(Config{HTTPClient: server.Client(), CoinGeckoSearchURL: server.URL + "/gecko", CoinbaseRESTURL: server.URL + "/coinbase", FundSearchURL: server.URL + "/fund", SearchTimeout: time.Second})
	cryptoDone := make(chan struct{})
	go func() { defer close(cryptoDone); _, _ = s.Search(context.Background(), "bitcoin", "CRYPTO") }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	items, err := s.Search(ctx, "000001", "CN_FUND")
	if err != nil || len(items) != 1 {
		t.Fatalf("fund blocked by crypto: %+v %v", items, err)
	}
	select {
	case <-cryptoDone:
		t.Fatal("crypto unexpectedly finished before fund")
	default:
	}
	<-cryptoDone
}

func TestCoalescedSearchHasIndependentCancellation(t *testing.T) {
	s := New(Config{SearchTimeout: time.Second})
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	task := func(ctx context.Context) ([]Candidate, error) {
		calls.Add(1)
		close(started)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
			return []Candidate{{Name: "shared"}}, nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := s.coalescedSearch(ctx, "same", 0, task); first <- err }()
	<-started
	cancel()
	if <-first != context.Canceled {
		t.Fatal("caller cancellation lost")
	}
	go func() { time.Sleep(30 * time.Millisecond); close(release) }()
	items, err := s.coalescedSearch(context.Background(), "same", 0, task)
	if err != nil || len(items) != 1 || calls.Load() != 1 {
		t.Fatalf("shared work cancelled/duplicated: %v %d", err, calls.Load())
	}
}

func TestPartialSearchIsFastAndNotCached(t *testing.T) {
	var products atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gecko" {
			<-r.Context().Done()
			return
		}
		products.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"products": []map[string]interface{}{{"product_id": "BTC-USD", "base_currency_id": "BTC", "quote_currency_id": "USD", "base_name": "Bitcoin", "product_type": "SPOT", "status": "online"}}})
	}))
	defer server.Close()
	s := New(Config{HTTPClient: server.Client(), CoinGeckoSearchURL: server.URL + "/gecko", CoinbaseRESTURL: server.URL + "/coinbase", SearchTimeout: 3 * time.Second})
	for i := 0; i < 2; i++ {
		start := time.Now()
		items, err := s.Search(context.Background(), "bitcoin", "CRYPTO")
		if err != nil || len(items) != 1 || time.Since(start) > 2*time.Second {
			t.Fatalf("lost fast alternative: %+v %v", items, err)
		}
	}
	if products.Load() != 2 {
		t.Fatal("cached incomplete catalogue as complete")
	}
}

func TestProviderRefreshProgressesWhileFundIsBlocked(t *testing.T) {
	started := make(chan struct{}, 1)
	now := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fund" {
			started <- struct{}{}
			<-r.Context().Done()
			return
		}
		body, _ := json.Marshal(securityFixture("600519", "100", "20260921150000", "CNY"))
		_, _ = w.Write([]byte("v_sh600519=" + string(body) + ";"))
	}))
	defer server.Close()
	s := New(Config{HTTPClient: server.Client(), FundNAVURL: server.URL + "/fund", TencentURL: server.URL + "/stock", Now: func() time.Time { return now }})
	_ = s.Register(Binding{"CN_FUND", "eastmoney", "000001", "CNY"})
	b := Binding{"CN_SH", "tencent", "sh600519", "CNY"}
	_ = s.Register(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.refreshReferences(ctx) }()
	<-started
	deadline := time.After(time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("security waiting for fund")
		case <-tick.C:
			if q, ok := s.Get(b.Key()); ok && q.Price == "100" {
				cancel()
				<-done
				return
			}
		}
	}
}

func TestBoundedRefreshWorkers(t *testing.T) {
	var active, max atomic.Int32
	runBounded(context.Background(), 24, 4, func(int) {
		n := active.Add(1)
		for old := max.Load(); n > old; old = max.Load() {
			if max.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
	})
	if max.Load() > 4 || max.Load() < 2 {
		t.Fatalf("worker limit not respected: %d", max.Load())
	}
}
