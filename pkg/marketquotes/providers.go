package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

func (s *Service) coinGeckoLoop(ctx context.Context) {
	for ctx.Err() == nil {
		s.refreshCoinGecko(ctx)
		// Check for newly unavailable primary quotes without spending another
		// upstream request before the shared fifteen-minute budget allows it.
		if !waitContext(ctx, 30*time.Second) {
			return
		}
	}
}

func (s *Service) refreshCoinGecko(ctx context.Context) {
	s.mu.Lock()
	bindings := make([]Binding, 0)
	seen := make(map[string]bool)
	// Public fallback also serves preset assets when Coinbase is unavailable.
	// Keep refreshing an active fallback, but leave healthy primary feeds alone.
	for _, candidate := range instruments {
		quote, exists := s.quotes[candidate.id]
		if s.config.CoinGeckoAPIKey != "" || !exists || quote.quote.Source == SourceCoinGecko || s.quoteViewLocked(quote).State == StateStale {
			bindings = append(bindings, Binding{Market: "CRYPTO", Provider: "coingecko", ProviderID: candidate.geckoID, Currency: "USD"})
			seen[candidate.geckoID] = true
		}
	}
	for _, b := range s.references {
		if b.Provider == "coingecko" && !seen[b.ProviderID] {
			bindings = append(bindings, b)
			seen[b.ProviderID] = true
		}
	}
	if len(bindings) == 0 {
		s.mu.Unlock()
		return
	}
	now := s.config.Now()
	// Count attempts as budget usage, including HTTP failures and 429s. Neither
	// reconnection nor a client refreshing its page bypasses this shared gate.
	if !s.lastGeckoAttempt.IsZero() && now.Sub(s.lastGeckoAttempt) < s.config.CoinGeckoInterval {
		s.mu.Unlock()
		return
	}
	s.lastGeckoAttempt = now
	s.mu.Unlock()
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ProviderID < bindings[j].ProviderID })
	quotes, err := s.fetchGeckoReferences(ctx, bindings)
	if err != nil {
		return
	}
	for _, q := range quotes {
		// putReferenceQuote only accepts registered public identities.
		s.putReferenceQuote(q)
	}
	for _, candidate := range instruments {
		b := Binding{Provider: "coingecko", ProviderID: candidate.geckoID}
		q, found := quotes[b.Key()]
		if !found {
			continue
		}
		q.InstrumentID = candidate.id
		s.putQuote(q, time.Unix(q.SourceTime, 0))
	}
}

func (s *Service) fxLoop(ctx context.Context) {
	for ctx.Err() == nil {
		delay := s.config.FXInterval
		if err := s.refreshFX(ctx); err != nil {
			delay = s.config.FXRetryInterval
		}
		if !waitContext(ctx, delay) {
			return
		}
	}
}

func (s *Service) refreshFX(ctx context.Context) error {
	var response struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}
	if err := s.getJSON(ctx, s.config.FXURL, nil, &response); err != nil {
		return err
	}
	date, err := time.Parse("2006-01-02", response.Date)
	price, valid := positiveDecimal(string(response.Rate))
	if err != nil || date.After(s.config.Now()) || response.Base != "USD" || response.Quote != "CNY" || !valid {
		return errors.New("invalid ECB reference rate")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fx.Date > response.Date {
		return errors.New("older ECB reference rate")
	}
	s.fx = FXRate{Base: "USD", Quote: "CNY", Rate: price, Date: response.Date, Source: SourceECB, ReceivedAt: s.config.Now().Unix(), State: StateDelayed}
	s.fxRestored = false
	return nil
}
