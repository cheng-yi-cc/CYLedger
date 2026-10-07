package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
)

// Only these explicit identities are mapped. A user-created token with a
// matching ticker is never silently treated as the same asset.
var cryptoHistorySymbols = map[string]string{
	"crypto:bitcoin": "BTC", "crypto:ethereum": "ETH", "crypto:solana": "SOL",
	"crypto:tether": "USDT", "crypto:usd-coin": "USDC",
}

func SupportsCryptoDCA(payment, target string) bool {
	return (payment == "crypto:tether" || payment == "crypto:usd-coin") &&
		(target == "crypto:bitcoin" || target == "crypto:ethereum" || target == "crypto:solana")
}

// Both legs use published minute-opening USD reference prices, including the
// stablecoin. They are not claimed to be the user's exchange execution prices.
type CryptoHistoricalQuote struct {
	PaymentPrice string `json:"paymentPrice"`
	TargetPrice  string `json:"targetPrice"`
	PriceTime    int64  `json:"priceTime"`
	Source       string `json:"source"`
	FXRate       string `json:"fxRate"`
	FXDate       string `json:"fxDate"`
	FXSource     string `json:"fxSource"`
	ReceivedAt   int64  `json:"receivedAt"`
}

type cryptoHistoryEntry struct {
	quote CryptoHistoricalQuote
	at    time.Time
	err   error
}

func (s *Service) HistoricalCryptoQuote(ctx context.Context, payment, target string, at int64) (*CryptoHistoricalQuote, error) {
	if !SupportsCryptoDCA(payment, target) || at <= 0 || at%60 != 0 || at+60 > s.config.Now().Unix() {
		return nil, errors.New("定投币种或历史分钟无效，请等待该分钟行情公布")
	}
	key := payment + ":" + target + ":" + strconv.FormatInt(at, 10)
	s.historyMu.Lock()
	entry, found := s.historyCache[key]
	s.historyMu.Unlock()
	if found && (entry.err == nil || s.config.Now().Sub(entry.at) < time.Minute) {
		q := entry.quote
		return &q, entry.err
	}
	q, err := s.fetchCryptoHistory(ctx, payment, target, at)
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	if len(s.historyCache) >= 512 || s.historyCache == nil {
		s.historyCache = make(map[string]cryptoHistoryEntry)
	}
	if ctx.Err() == nil {
		s.historyCache[key] = cryptoHistoryEntry{quote: q, at: s.config.Now(), err: err}
	}
	return &q, err
}

func (s *Service) fetchCryptoHistory(ctx context.Context, payment, target string, at int64) (CryptoHistoricalQuote, error) {
	q := CryptoHistoricalQuote{PriceTime: at, Source: "Bitstamp 分钟开盘参考价", FXSource: SourceECB, ReceivedAt: s.config.Now().Unix()}
	var err error
	q.PaymentPrice, err = s.cryptoMinutePrice(ctx, cryptoHistorySymbols[payment], at)
	if err != nil {
		return q, err
	}
	q.TargetPrice, err = s.cryptoMinutePrice(ctx, cryptoHistorySymbols[target], at)
	if err != nil {
		return q, err
	}
	// Use the previous UTC date as a conservative publication cutoff. Never
	// backfill a morning purchase with an FX rate published later that day.
	cutoff := time.Unix(at, 0).UTC().AddDate(0, 0, -1).Format("2006-01-02")
	base := s.config.HistoricalFXURL
	if base == "" {
		base = "https://api.frankfurter.dev/v2/providers/ecb/rates"
	}
	var rates []struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}
	err = s.getJSON(ctx, queryURL(base, url.Values{"base": {"USD"}, "quotes": {"CNY"}, "date": {cutoff}}), nil, &rates)
	if err != nil || len(rates) != 1 {
		return q, errors.New("历史人民币汇率暂不可用")
	}
	r := rates[0]
	date, dateErr := time.Parse("2006-01-02", r.Date)
	if dateErr != nil || r.Date > cutoff || time.Unix(at, 0).Sub(date) > 10*24*time.Hour || r.Base != "USD" || r.Quote != "CNY" || !validHistoricalDecimal(string(r.Rate)) {
		return q, errors.New("历史人民币汇率日期或数值无效")
	}
	q.FXRate, q.FXDate = string(r.Rate), r.Date
	return q, nil
}

func validHistoricalDecimal(value string) bool {
	if investments.ValidateDecimal(value) != nil {
		return false
	}
	_, ok := positiveDecimal(value)
	return ok
}

func (s *Service) cryptoMinutePrice(ctx context.Context, symbol string, at int64) (string, error) {
	base := s.config.CryptoHistoryURL
	if base == "" {
		base = "https://www.bitstamp.net/api/v2/ohlc"
	}
	var response struct {
		Data struct {
			Pair string `json:"pair"`
			OHLC []struct {
				Timestamp string `json:"timestamp"`
				Open      string `json:"open"`
			} `json:"ohlc"`
		} `json:"data"`
	}
	address := strings.TrimRight(base, "/") + "/" + strings.ToLower(symbol) + "usd/"
	err := s.getJSON(ctx, queryURL(address, url.Values{"step": {"60"}, "start": {strconv.FormatInt(at, 10)}, "end": {strconv.FormatInt(at+59, 10)}, "limit": {"1"}}), nil, &response)
	if err != nil {
		return "", fmt.Errorf("历史分钟行情暂不可用: %w", err)
	}
	if response.Data.Pair != symbol+"/USD" || len(response.Data.OHLC) != 1 {
		return "", errors.New("历史行情币种或分钟不匹配")
	}
	row := response.Data.OHLC[0]
	stamp, err := strconv.ParseInt(row.Timestamp, 10, 64)
	if err != nil || stamp != at || !validHistoricalDecimal(row.Open) {
		return "", errors.New("缺少指定分钟的有效价格")
	}
	return row.Open, nil
}
