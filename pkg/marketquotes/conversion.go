package marketquotes

import (
	"context"
	"strings"
	"time"
)

// RefreshCryptoConversion is an explicit, bounded refresh for a user preparing
// a conversion. Only public identities leave the ledger; no balances or amounts
// are sent. It shares the CoinGecko attempt budget with periodic refreshes and
// caps on-demand requests to one batch per minute, including failed requests.
func (s *Service) RefreshCryptoConversion(ctx context.Context, ids []string, bindings []Binding) {
	s.conversionMu.Lock()
	defer s.conversionMu.Unlock()
	now := s.config.Now()
	need := false
	for _, id := range ids {
		q, ok := s.Get(id)
		if !ok || q.State == StateUnavailable || q.State == StateStale || (q.State != StateLive && now.Sub(time.Unix(q.SourceTime, 0)) > time.Minute) {
			need = true
		}
	}
	if !need {
		return
	}
	gecko := make(map[string]Binding)
	// The fixed preset identities are verified mappings, independent of names.
	for _, item := range instruments {
		gecko[item.geckoID] = Binding{Market: "CRYPTO", Provider: "coingecko", ProviderID: item.geckoID, Currency: "USD"}
	}
	for _, b := range bindings {
		if b.Provider == "coingecko" {
			gecko[b.ProviderID] = b
		}
		if b.Provider == "coinbase" {
			s.mu.Lock()
			previous := s.lastReferenceAttempt[b.Key()]
			eligible := previous.IsZero() || now.Sub(previous) >= time.Minute
			if eligible {
				s.lastReferenceAttempt[b.Key()] = now
			}
			s.mu.Unlock()
			if eligible {
				_, q, err := s.fetchCoinbaseReference(ctx, b)
				if err == nil {
					s.putReferenceQuote(q)
				}
			}
		}
	}
	s.mu.Lock()
	if !s.lastGeckoAttempt.IsZero() && now.Sub(s.lastGeckoAttempt) < time.Minute {
		s.mu.Unlock()
		return
	}
	s.lastGeckoAttempt = now
	s.mu.Unlock()
	batch := make([]Binding, 0, len(gecko))
	for _, b := range gecko {
		batch = append(batch, b)
	}
	quotes, err := s.fetchGeckoReferences(ctx, batch)
	if err != nil {
		return
	}
	for _, q := range quotes {
		s.putReferenceQuote(q)
		id := strings.TrimPrefix(q.InstrumentID, "market:coingecko:")
		for _, item := range instruments {
			if item.geckoID == id {
				q.InstrumentID = item.id
				s.putQuote(q, time.Unix(q.SourceTime, 0))
				break
			}
		}
	}
}
