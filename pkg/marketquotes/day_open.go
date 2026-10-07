package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/log"
)

// DayQuote is a public reference at an accounting-day boundary. The price is
// the close of a candle ending exactly at midnight, not a rolling 24-hour change.
// No account, quantity, cost, or user identifier is sent to a provider.
type DayQuote struct {
	InstrumentID string `json:"instrumentId"`
	At           int64  `json:"at"`
	Price        string `json:"price"`
	Currency     string `json:"currency"`
	Source       string `json:"source"`
	SourceTime   int64  `json:"sourceTime"`
	Period       int64  `json:"period"`
	FXRate       string `json:"fxRate"`
	FXDate       string `json:"fxDate"`
	FXSource     string `json:"fxSource"`
}

type dayEntry struct {
	quote   DayQuote
	attempt time.Time
	pending bool
}

var dayRequests = make(chan struct{}, 4)

func dayKey(id string, at int64) string { return fmt.Sprintf("%s:%d", id, at) }

// DayReference is nonblocking. The next foreground read picks up a completed
// lookup. Failed history stays unknown and retries no more than once a minute.
func (s *Service) DayReference(id string, at int64) (DayQuote, bool) {
	now := s.config.Now()
	if at > now.Unix() || at < now.Add(-48*time.Hour).Unix() || at%60 != 0 {
		return DayQuote{}, false
	}
	s.dayMu.Lock()
	key := dayKey(id, at)
	entry := s.days[key]
	if entry != nil && entry.quote.Price != "" {
		q := entry.quote
		s.dayMu.Unlock()
		return q, true
	}
	if entry != nil && (entry.pending || now.Sub(entry.attempt) < time.Minute) {
		s.dayMu.Unlock()
		return DayQuote{}, false
	}
	if entry == nil {
		for k, old := range s.days {
			if old.quote.At < now.Add(-48*time.Hour).Unix() && !old.pending {
				delete(s.days, k)
			}
		}
		if len(s.days) >= 1500 {
			s.dayMu.Unlock()
			return DayQuote{}, false
		}
		entry = &dayEntry{quote: DayQuote{InstrumentID: id, At: at}}
		s.days[key] = entry
	}
	entry.attempt, entry.pending = now, true
	s.dayMu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		var q DayQuote
		var err error
		select {
		case dayRequests <- struct{}{}:
			q, err = s.fetchDayReference(ctx, id, at)
			<-dayRequests
		case <-ctx.Done():
			err = ctx.Err()
		}
		s.dayMu.Lock()
		entry.pending = false
		if err == nil {
			entry.quote = q
		}
		s.dayMu.Unlock()
		if err != nil {
			// Public history only: never log the account, quantity, cost or user.
			log.Warnf(nil, "[marketquotes.DayReference] public reference unavailable: %v", err)
		}
	}()
	return DayQuote{}, false
}

func (s *Service) dailyProduct(id string) (string, bool) {
	for _, candidate := range instruments {
		if candidate.id == id {
			return candidate.productID, true
		}
	}
	s.mu.RLock()
	b, ok := s.references[id]
	s.mu.RUnlock()
	if !ok || b.Market != "CRYPTO" {
		return "", false
	}
	if b.Provider == "coinbase" && b.Currency == "USD" {
		return b.ProviderID, true
	}
	// Cross-provider identity is allowed only for our fixed, verified mappings.
	if b.Provider == "coingecko" {
		for _, candidate := range instruments {
			if candidate.geckoID == b.ProviderID {
				return candidate.productID, true
			}
		}
	}
	return "", false
}

func (s *Service) fetchDayReference(ctx context.Context, id string, at int64) (DayQuote, error) {
	product, ok := s.dailyProduct(id)
	_, geckoOK := s.dailyGeckoID(id)
	if !ok && !geckoOK {
		return DayQuote{}, errors.New("该标的暂无零点历史报价")
	}
	// A stored public binding or fixed preset supplies identity; the response must
	// contain the exact requested, fully closed minute. Never take a nearby candle.
	var response struct {
		Candles []struct {
			Start string `json:"start"`
			Close string `json:"close"`
		} `json:"candles"`
	}
	address := s.config.CoinbaseRESTURL + "/products/" + url.PathEscape(product) + "/candles"
	primary, cancel := context.WithTimeout(ctx, 8*time.Second)
	var err error
	if ok {
		err = s.getJSON(primary, queryURL(address, url.Values{"start": {fmt.Sprint(at - 60)}, "end": {fmt.Sprint(at)}, "granularity": {"ONE_MINUTE"}, "limit": {"2"}}), nil, &response)
	}
	cancel()
	price := ""
	source, period := SourceCoinbaseREST+" 零点前分钟收盘价", int64(60)
	if err == nil {
		for _, candle := range response.Candles {
			if candle.Start != fmt.Sprint(at-60) {
				continue
			}
			value, valid := positiveDecimal(candle.Close)
			if !valid || price != "" && price != value {
				return DayQuote{}, errors.New("零点历史报价无效")
			}
			price = value
		}
	}
	if err != nil || price == "" {
		// Coinbase Exchange exposes the same market's closed minute through a
		// separate public host. Keep JSON decimals exact and validate its timestamp.
		if ok {
			price, err = s.exchangeDayClose(ctx, product, at)
		}
		if err != nil || price == "" {
			price, err = s.geckoDayClose(ctx, id, at)
			if err != nil {
				return DayQuote{}, err
			}
			source, period = "CoinGecko 零点收盘价（30分钟线）", 1800
		}
	}
	if price == "" {
		return DayQuote{}, errors.New("缺少零点前一分钟收盘价")
	}
	// ECB publishes business-day reference rates around 16:00 in Frankfurt.
	// Choose the latest reference date that had reached that publication window.
	date, err := dayFXDate(at)
	if err != nil {
		return DayQuote{}, err
	}
	var fx struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}
	if err = s.getJSON(ctx, queryURL(s.config.FXURL, url.Values{"date": {date}}), nil, &fx); err != nil {
		return DayQuote{}, err
	}
	rate, valid := positiveDecimal(fx.Rate.String())
	day, parseErr := time.Parse("2006-01-02", fx.Date)
	requested, _ := time.Parse("2006-01-02", date)
	if !valid || parseErr != nil || fx.Date > date || requested.Sub(day) > 7*24*time.Hour || strings.ToUpper(fx.Base) != "USD" || strings.ToUpper(fx.Quote) != "CNY" {
		return DayQuote{}, errors.New("缺少有效历史人民币汇率")
	}
	return DayQuote{InstrumentID: id, At: at, Price: price, Currency: "USD", Source: source, SourceTime: at - period, Period: period, FXRate: rate, FXDate: fx.Date, FXSource: SourceECB}, nil
}

func (s *Service) exchangeDayClose(ctx context.Context, product string, at int64) (string, error) {
	var candles [][]json.Number
	address := s.config.CoinbaseExchangeURL + "/products/" + url.PathEscape(product) + "/candles"
	err := s.getJSON(ctx, queryURL(address, url.Values{
		"start": {time.Unix(at-60, 0).UTC().Format(time.RFC3339)},
		"end":   {time.Unix(at, 0).UTC().Format(time.RFC3339)}, "granularity": {"60"},
	}), nil, &candles)
	if err != nil {
		return "", err
	}
	price := ""
	for _, candle := range candles {
		if len(candle) != 6 {
			continue
		}
		start, err := candle[0].Int64()
		if err != nil || start != at-60 {
			continue
		}
		value, valid := positiveDecimal(candle[4].String())
		if !valid || price != "" && price != value {
			return "", errors.New("零点历史报价无效")
		}
		price = value
	}
	if price == "" {
		return "", errors.New("缺少零点前一分钟收盘价")
	}
	return price, nil
}

func dayFXDate(at int64) (string, error) {
	zone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return "", err
	}
	boundary := time.Unix(at, 0).In(zone)
	fixing := time.Date(boundary.Year(), boundary.Month(), boundary.Day(), 16, 0, 0, 0, zone)
	if boundary.Before(fixing) {
		boundary = boundary.AddDate(0, 0, -1)
	}
	return boundary.Format("2006-01-02"), nil
}

func (s *Service) DayQuotes() []DayQuote {
	s.dayMu.Lock()
	defer s.dayMu.Unlock()
	result := make([]DayQuote, 0)
	for _, entry := range s.days {
		if entry.quote.Price != "" {
			result = append(result, entry.quote)
		}
	}
	return result
}

func (s *Service) RestoreDayQuotes(quotes []DayQuote) {
	for _, q := range quotes {
		_, productOK := s.dailyProduct(q.InstrumentID)
		_, geckoOK := s.dailyGeckoID(q.InstrumentID)
		if !productOK && !geckoOK {
			continue
		}
		if _, ok := positiveDecimal(q.Price); !ok {
			continue
		}
		if _, ok := positiveDecimal(q.FXRate); !ok {
			continue
		}
		if q.Period == 0 && q.Source == SourceCoinbaseREST+" 零点前分钟收盘价" {
			q.Period = 60
		}
		validSource := q.Period == 60 && q.Source == SourceCoinbaseREST+" 零点前分钟收盘价" || q.Period == 1800 && q.Source == "CoinGecko 零点收盘价（30分钟线）"
		if !validSource || q.Currency != "USD" || q.At%q.Period != 0 || q.SourceTime != q.At-q.Period || q.FXSource != SourceECB || q.At > s.config.Now().Unix() || q.At < s.config.Now().Add(-48*time.Hour).Unix() {
			continue
		}
		date, err := time.Parse("2006-01-02", q.FXDate)
		expected, zoneErr := dayFXDate(q.At)
		latest, _ := time.Parse("2006-01-02", expected)
		if err != nil || zoneErr != nil || q.FXDate > expected || latest.Sub(date) > 7*24*time.Hour {
			continue
		}
		s.dayMu.Lock()
		if len(s.days) < 1500 {
			s.days[dayKey(q.InstrumentID, q.At)] = &dayEntry{quote: q}
		}
		s.dayMu.Unlock()
	}
}
