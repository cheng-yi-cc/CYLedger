package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

func (s *Service) coinGeckoLoop(ctx context.Context) {
	for ctx.Err() == nil {
		s.refreshCoinGecko(ctx)
		if !waitContext(ctx, s.config.CoinGeckoInterval) {
			return
		}
	}
}

func (s *Service) refreshCoinGecko(ctx context.Context) {
	if s.config.CoinGeckoAPIKey == "" {
		return
	}
	s.mu.Lock()
	now := s.config.Now()
	// Count attempts as budget usage, including HTTP failures and 429s. Neither
	// reconnection nor a client refreshing its page bypasses this shared gate.
	if !s.lastGeckoAttempt.IsZero() && now.Sub(s.lastGeckoAttempt) < s.config.CoinGeckoInterval {
		s.mu.Unlock()
		return
	}
	s.lastGeckoAttempt = now
	s.mu.Unlock()
	ids := make([]string, 0, len(instruments))
	for _, candidate := range instruments {
		ids = append(ids, candidate.geckoID)
	}
	address, err := url.Parse(s.config.CoinGeckoURL)
	if err != nil {
		return
	}
	query := address.Query()
	query.Set("ids", strings.Join(ids, ","))
	query.Set("vs_currencies", "usd")
	query.Set("include_last_updated_at", "true")
	query.Set("precision", "full")
	address.RawQuery = query.Encode()
	var response map[string]struct {
		USD           json.Number `json:"usd"`
		LastUpdatedAt int64       `json:"last_updated_at"`
	}
	if err = s.getJSON(ctx, address.String(), map[string]string{"x-cg-demo-api-key": s.config.CoinGeckoAPIKey}, &response); err != nil {
		return
	}
	for _, candidate := range instruments {
		price, found := response[candidate.geckoID]
		if !found || price.LastUpdatedAt <= 0 {
			continue
		}
		s.putQuote(Quote{InstrumentID: candidate.id, Price: string(price.USD), Currency: "USD", Source: SourceCoinGecko, ReceivedAt: s.config.Now().Unix(), State: StateDelayed}, time.Unix(price.LastUpdatedAt, 0))
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
