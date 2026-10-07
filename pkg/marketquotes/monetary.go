package marketquotes

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

// MonetaryYield is a published daily distribution, never a unit price.
type MonetaryYield struct {
	Date           string `json:"date"`
	PerTenThousand string `json:"perTenThousand"`
}

func (s *Service) SearchMonetaryFunds(ctx context.Context, query string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 2 || len(query) > 100 {
		return nil, errors.New("请输入基金代码或至少两个字的名称")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.monetarySearchMu.Lock()
	cached, ok := s.monetarySearchCache[strings.ToLower(query)]
	s.monetarySearchMu.Unlock()
	if ok && s.config.Now().Sub(cached.at) < 10*time.Minute {
		return append([]Candidate{}, cached.items...), nil
	}
	return s.coalescedSearch(ctx, "monetary:"+strings.ToLower(query), 0, func(work context.Context) ([]Candidate, error) { return s.searchMonetaryFunds(work, query) })
}

func (s *Service) searchMonetaryFunds(ctx context.Context, query string) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 2 || len(query) > 100 {
		return nil, errors.New("请输入基金代码或至少两个字的名称")
	}
	key := strings.ToLower(query)
	// A selected public catalogue result remains valid for the following save.
	// This caches identities only, never yields, prices, or account data.
	s.monetarySearchMu.Lock()
	cached, ok := s.monetarySearchCache[key]
	s.monetarySearchMu.Unlock()
	if ok && s.config.Now().Sub(cached.at) < 10*time.Minute {
		return append([]Candidate{}, cached.items...), nil
	}
	var response fundSearchResponse
	if err := s.getJSON(ctx, queryURL(s.config.FundSearchURL, url.Values{"m": {"1"}, "key": {query}}), nil, &response); err != nil {
		return nil, err
	}
	if response.ErrorCode != 0 {
		return nil, errors.New("货币基金查询暂不可用")
	}
	items := make([]Candidate, 0)
	for _, f := range response.Data {
		if f.Category == 700 && f.Info != nil && f.Info.Type == "005" && f.Code == f.Info.Code && sixDigits.MatchString(f.Code) && f.Name != "" {
			items = append(items, Candidate{Binding: Binding{Market: "CN_FUND", Provider: "eastmoney", ProviderID: f.Code, Currency: "CNY"}, Symbol: f.Code, Name: f.Name, Type: "MONETARY_FUND"})
		}
		if len(items) >= 50 {
			break
		}
	}
	if len(items) > 0 {
		s.monetarySearchMu.Lock()
		defer s.monetarySearchMu.Unlock()
		if len(s.monetarySearchCache)+len(items)+1 > 200 {
			s.monetarySearchCache = make(map[string]searchEntry)
		}
		now := s.config.Now()
		s.monetarySearchCache[key] = searchEntry{items: append([]Candidate{}, items...), at: now}
		for _, item := range items {
			s.monetarySearchCache[item.ProviderID] = searchEntry{items: []Candidate{item}, at: now}
		}
	}
	return items, nil
}

// Requests contain public fund identifiers and dates only, never ledger balances.
func (s *Service) MonetaryYields(ctx context.Context, code, from, to string) ([]MonetaryYield, error) {
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	if !sixDigits.MatchString(code) || e1 != nil || e2 != nil || end.Before(start) || end.Sub(start) > 90*24*time.Hour {
		return nil, errors.New("收益查询范围无效")
	}
	var response struct {
		ErrorCode int `json:"ErrCode"`
		Data      struct {
			Type string `json:"FundType"`
			Rows []struct {
				Date  string `json:"FSRQ"`
				Yield string `json:"DWJZ"`
			} `json:"LSJZList"`
		} `json:"Data"`
	}
	err := s.getJSON(ctx, queryURL(s.config.FundNAVURL, url.Values{"fundCode": {code}, "pageIndex": {"1"}, "pageSize": {"100"}, "startDate": {from}, "endDate": {to}}), map[string]string{"Referer": "https://fundf10.eastmoney.com/"}, &response)
	if err != nil {
		return nil, err
	}
	if response.ErrorCode != 0 || response.Data.Type != "005" {
		return nil, errors.New("来源未确认该基金为货币基金")
	}
	items := make([]MonetaryYield, 0, len(response.Data.Rows))
	seen := map[string]string{}
	for _, row := range response.Data.Rows {
		if _, err := time.Parse("2006-01-02", row.Date); err != nil || len(row.Date) != 10 || row.Date < from || row.Date > to {
			return nil, errors.New("收益来源返回了无效日期")
		}
		if row.Yield == "" || row.Yield == "--" {
			continue
		}
		if len(row.Yield) > 24 || !decimalText.MatchString(row.Yield) {
			return nil, errors.New("万份收益格式无效")
		}
		value, err := decimal.NewFromString(row.Yield)
		if err != nil || value.Abs().GreaterThan(decimal.NewFromInt(10000)) {
			return nil, errors.New("万份收益超出范围")
		}
		if old, ok := seen[row.Date]; ok {
			if old != value.String() {
				return nil, errors.New("同日收益数据冲突")
			}
			continue
		}
		seen[row.Date] = value.String()
		items = append(items, MonetaryYield{Date: row.Date, PerTenThousand: value.String()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Date < items[j].Date })
	return items, nil
}
