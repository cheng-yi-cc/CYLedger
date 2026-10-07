package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

func (s *Service) dailyGeckoID(id string) (string, bool) {
	for _, p := range instruments {
		if p.id == id {
			return p.geckoID, true
		}
	}
	s.mu.RLock()
	b, ok := s.references[id]
	s.mu.RUnlock()
	if !ok || b.Market != "CRYPTO" {
		return "", false
	}
	if b.Provider == "coingecko" {
		return b.ProviderID, true
	}
	if b.Provider == "coinbase" && b.Currency == "USD" {
		for _, p := range instruments {
			if p.productID == b.ProviderID {
				return p.geckoID, true
			}
		}
	}
	return "", false
}

// CoinGecko timestamps describe the candle CLOSE, unlike Coinbase's start.
// Select the exact midnight close from its 30-minute public OHLC history.
func (s *Service) geckoDayClose(ctx context.Context, id string, at int64) (string, error) {
	coin, ok := s.dailyGeckoID(id)
	if !ok || at%1800 != 0 {
		return "", errors.New("该标的暂无匹配零点的备用历史报价")
	}
	var candles [][]json.Number
	headers := map[string]string{}
	if s.config.CoinGeckoAPIKey != "" {
		headers["x-cg-demo-api-key"] = s.config.CoinGeckoAPIKey
	}
	address := strings.TrimRight(s.config.CoinGeckoCoinURL, "/") + "/" + url.PathEscape(coin) + "/ohlc"
	if err := s.getJSON(ctx, queryURL(address, url.Values{"vs_currency": {"usd"}, "days": {"1"}, "precision": {"full"}}), headers, &candles); err != nil {
		return "", err
	}
	price := ""
	for _, c := range candles {
		if len(c) != 5 {
			continue
		}
		closeTime, err := c[0].Int64()
		if err != nil || closeTime != at*1000 {
			continue
		}
		value, valid := positiveDecimal(c[4].String())
		if !valid || price != "" && price != value {
			return "", errors.New("零点备用历史报价无效")
		}
		price = value
	}
	if price == "" {
		return "", errors.New("缺少正好在零点收盘的历史报价")
	}
	return price, nil
}
