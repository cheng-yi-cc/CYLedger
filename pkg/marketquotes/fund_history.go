package marketquotes

import (
	"context"
	"errors"
	"github.com/shopspring/decimal"
	"net/url"
	"sort"
	"time"
)

type FundNAV struct {
	Date  string `json:"date"`
	Price string `json:"price"`
}

// 仅接收公开代码和日期；货币基金万份收益不能用于确认份额。
func (s *Service) FundHistory(ctx context.Context, code, from, to string) ([]FundNAV, error) {
	a, e1 := time.Parse("2006-01-02", from)
	b, e2 := time.Parse("2006-01-02", to)
	if !sixDigits.MatchString(code) || e1 != nil || e2 != nil || b.Before(a) || b.Sub(a) > 90*24*time.Hour {
		return nil, errors.New("净值查询范围无效")
	}
	var response struct {
		ErrorCode int `json:"ErrCode"`
		Data      struct {
			Type string `json:"FundType"`
			Rows []struct {
				Date  string `json:"FSRQ"`
				Price string `json:"DWJZ"`
			} `json:"LSJZList"`
		} `json:"Data"`
	}
	if err := s.getJSON(ctx, queryURL(s.config.FundNAVURL, url.Values{"fundCode": {code}, "pageIndex": {"1"}, "pageSize": {"100"}, "startDate": {from}, "endDate": {to}}), map[string]string{"Referer": "https://fundf10.eastmoney.com/"}, &response); err != nil {
		return nil, err
	}
	if response.ErrorCode != 0 || response.Data.Type == "005" || response.Data.Type == "" {
		return nil, errors.New("未取得普通基金单位净值，货币基金请使用收益同步")
	}
	result := []FundNAV{}
	seen := map[string]string{}
	for _, row := range response.Data.Rows {
		if _, err := time.Parse("2006-01-02", row.Date); err != nil || len(row.Date) != 10 || row.Date < from || row.Date > to {
			return nil, errors.New("来源返回的净值日期无效")
		}
		if row.Price == "" || row.Price == "--" {
			continue
		}
		if len(row.Price) > 24 || !decimalText.MatchString(row.Price) {
			return nil, errors.New("净值格式无效")
		}
		price, err := decimal.NewFromString(row.Price)
		if err != nil || !price.IsPositive() {
			return nil, errors.New("净值必须为正数")
		}
		if old, ok := seen[row.Date]; ok {
			if old != price.String() {
				return nil, errors.New("同日净值数据冲突")
			}
			continue
		}
		seen[row.Date] = price.String()
		result = append(result, FundNAV{Date: row.Date, Price: price.String()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result, nil
}
