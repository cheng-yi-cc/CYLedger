package marketquotes

import (
	"context"
	"sort"
	"sync"
	"time"
)

func (s *Service) referenceProviderLoop(ctx context.Context, provider string) {
	for ctx.Err() == nil {
		s.refreshReferenceProvider(ctx, provider)
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.referenceWake[provider]:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *Service) refreshReferences(ctx context.Context) {
	var wg sync.WaitGroup
	for _, provider := range []string{"tencent", "eastmoney", "coinbase"} {
		wg.Add(1)
		go func(provider string) { defer wg.Done(); s.refreshReferenceProvider(ctx, provider) }(provider)
	}
	wg.Wait()
}

// A fund outage does not stop the crypto/security loops. Each source has four
// workers at most, and attempts are gated before dispatch (including failures).
func (s *Service) refreshReferenceProvider(ctx context.Context, provider string) {
	interval := time.Minute
	if provider == "eastmoney" {
		interval = 30 * time.Minute
	}
	s.mu.Lock()
	now := s.config.Now()
	bindings := make([]Binding, 0)
	for key, b := range s.references {
		if b.Provider != provider {
			continue
		}
		if previous := s.lastReferenceAttempt[key]; !previous.IsZero() && now.Sub(previous) < interval {
			continue
		}
		s.lastReferenceAttempt[key] = now
		bindings = append(bindings, b)
	}
	s.mu.Unlock()
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Key() < bindings[j].Key() })
	batchSize := 1
	if provider == "tencent" {
		batchSize = 50
	}
	batches := make([][]Binding, 0)
	for i := 0; i < len(bindings); i += batchSize {
		end := i + batchSize
		if end > len(bindings) {
			end = len(bindings)
		}
		batches = append(batches, bindings[i:end])
	}
	dispatched := make([]bool, len(batches))
	runBounded(ctx, len(batches), 4, func(i int) {
		dispatched[i] = true
		batch := batches[i]
		quotes := make(map[string]Quote)
		if provider == "tencent" {
			quotes, _ = s.fetchTencent(ctx, batch)
		} else {
			b := batch[0]
			var q Quote
			var err error
			if provider == "eastmoney" {
				q, err = s.fetchFund(ctx, b)
			} else {
				_, q, err = s.fetchCoinbaseReference(ctx, b)
			}
			if err == nil {
				quotes[b.Key()] = q
			}
		}
		for _, b := range batch {
			q, ok := quotes[b.Key()]
			if ok && s.putReferenceQuote(q) {
				continue
			}
			// A failed fund request must not suppress recovery for another 30 minutes.
			s.mu.Lock()
			if s.lastReferenceAttempt[b.Key()].Equal(now) {
				s.lastReferenceAttempt[b.Key()] = s.config.Now().Add(-interval + time.Minute)
			}
			s.mu.Unlock()
		}
	})
	// Cancellation must not spend a 30-minute budget on queued, unstarted work.
	s.mu.Lock()
	for i, batch := range batches {
		if dispatched[i] {
			continue
		}
		for _, b := range batch {
			if s.lastReferenceAttempt[b.Key()].Equal(now) {
				delete(s.lastReferenceAttempt, b.Key())
			}
		}
	}
	s.mu.Unlock()
}

func runBounded(ctx context.Context, count, workers int, task func(int)) {
	if count < workers {
		workers = count
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() == nil {
					task(job)
				}
			}
		}()
	}
	for i := 0; i < count; i++ {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}
