package services

import (
	"encoding/json"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"time"
)

type InvestmentReportScope struct {
	BookIds []string
}

type InvestmentReportItem struct {
	Event  InvestmentEvent         `json:"event"`
	Effect investments.EventEffect `json:"effect"`
}
type InvestmentReportPoint struct {
	At     int64   `json:"at"`
	Value  *string `json:"value"`
	Profit *string `json:"profit"`
}
type InvestmentReport struct {
	Items      []InvestmentReportItem  `json:"items"`
	History    []InvestmentReportPoint `json:"history"`
	Annualized *string                 `json:"annualized"`
	TimeZone   string                  `json:"timeZone"`
}

func (s *InvestmentService) InvestmentReport(c core.Context, uid int64, account, instrument string, scopes ...InvestmentReportScope) (*InvestmentReport, error) {
	if len(account) > 64 || len(instrument) > 64 {
		return nil, investmentError("筛选条件无效")
	}
	if err := s.rebuildHistory(c, uid); err != nil {
		return nil, err
	}
	events, err := s.Events(c, uid)
	if err != nil {
		return nil, err
	}
	replayed, err := replayInvestments(events)
	if err != nil {
		return nil, err
	}
	settings, err := s.Settings(c, uid)
	if err != nil {
		return nil, err
	}
	zone := settings.TimeZone
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	result := &InvestmentReport{Items: []InvestmentReportItem{}, History: []InvestmentReportPoint{}, TimeZone: zone}
	profiles := map[string]models.InvestmentHoldingProfile{}
	var preferences *models.AssetPreferences
	var bookIds []string
	if len(scopes) > 0 {
		bookIds = scopes[0].BookIds
		if len(bookIds) > 100 {
			return nil, investmentError("账本筛选过多")
		}
		for _, id := range bookIds {
			if len(id) > 64 {
				return nil, investmentError("账本筛选无效")
			}
		}
		rows, err := s.HoldingProfiles(c, uid)
		if err != nil {
			return nil, err
		}
		for _, p := range rows {
			profiles[p.AccountId+":"+p.InstrumentId] = p
		}
		preferences, err = s.AssetPreferences(c, uid)
		if err != nil {
			return nil, err
		}
	}
	byID := map[string]investments.EventEffect{}
	for _, e := range replayed.Effects {
		byID[e.EventID] = e
	}
	matches := func(a, i string) bool {
		if (account != "" && a != account) || (instrument != "" && i != instrument) {
			return false
		}
		if len(scopes) == 0 {
			return true
		}
		p := profiles[a+":"+i]
		if p.ExcludeFromTotal {
			return false
		}
		if len(bookIds) == 0 {
			return true
		}
		contains := func(ids []string, value string) bool {
			for _, id := range ids {
				if id == value {
					return true
				}
			}
			return false
		}
		for _, book := range bookIds {
			if (len(p.BookIds) == 0 || contains(p.BookIds, book)) && !contains(preferences.Rules["portfolio:"+a].DisabledBooks, book) {
				return true
			}
		}
		return false
	}
	for _, e := range events {
		if e.Voided {
			continue
		}
		settlementAccount := e.SettlementAccountID
		if settlementAccount == "" {
			settlementAccount = e.AccountID
		}
		primary := matches(e.AccountID, e.InstrumentID)
		settlement := e.SettlementInstrumentID != "" && matches(settlementAccount, e.SettlementInstrumentID)
		match := primary || e.ToAccountID != "" && matches(e.ToAccountID, e.InstrumentID) || settlement
		for _, leg := range e.AdditionalMovements {
			if matches(e.AccountID, leg.InstrumentID) {
				match = true
			}
		}
		if match {
			effect := byID[e.ID]
			if e.SettlementInstrumentID != "" && e.Type == investments.Buy && !settlement {
				effect.RealizedPNL = decimalPointer(decimal.Zero)
			}
			if !primary {
				effect.InvestmentFee = decimalPointer(decimal.Zero)
				if settlement && e.Type == investments.Buy {
					effect.RealizedPNL = effect.SettlementRealizedPNL
				} else {
					effect.RealizedPNL = decimalPointer(decimal.Zero)
				}
			}
			if e.Wallet != nil && instrument != "" && len(e.AdditionalMovements) > 0 {
				effect.RealizedPNL = nil
			}
			result.Items = append(result.Items, InvestmentReportItem{Event: e, Effect: effect})
		}
	}
	rows := []models.WealthSnapshot{}
	if err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Desc("recorded_at").Limit(2000).Find(&rows); err != nil {
		return nil, err
	}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		point := InvestmentReportPoint{At: row.RecordedAt}
		if !row.Invalidated {
			var observation wealthObservation
			if json.Unmarshal([]byte(row.Payload), &observation) == nil && observation.Summary != nil {
				value, profit := decimal.Zero, decimal.Zero
				valueKnown, profitKnown := true, true
				for _, p := range observation.Summary.Positions {
					if !matches(p.AccountID, p.InstrumentID) {
						continue
					}
					if p.MarketValue == nil {
						valueKnown = false
					} else {
						d, _ := decimal.NewFromString(*p.MarketValue)
						value = value.Add(d)
					}
					if p.UnrealizedPNL == nil || p.RealizedPNL == nil {
						profitKnown = false
					} else {
						u, _ := decimal.NewFromString(*p.UnrealizedPNL)
						r, _ := decimal.NewFromString(*p.RealizedPNL)
						profit = profit.Add(u).Add(r)
					}
				}
				if valueKnown {
					point.Value = decimalPointer(value)
				}
				if profitKnown {
					point.Profit = decimalPointer(profit)
				}
			}
		}
		result.History = append(result.History, point)
	}
	// 年化估算使用实际资金占用日，不把价格涨幅当作个人收益。
	// 所有金额、权重、除法保持十进制；转移、校准或缺成本时不给出虚假精度。
	now := time.Now().Unix()
	weighted := decimal.Zero
	capital := decimal.Zero
	known := true
	for _, item := range result.Items {
		e := item.Event
		if e.SettlementInstrumentID != "" {
			known = false
			continue
		}
		var flow decimal.Decimal
		switch e.Type {
		case investments.Opening:
			if e.Cost == nil {
				known = false
				continue
			}
			flow, _ = decimal.NewFromString(*e.Cost)
		case investments.Buy, investments.Sell:
			if e.ExchangeRate == "" {
				known = false
				continue
			}
			amount, _ := decimal.NewFromString(e.Amount)
			fee, _ := decimal.NewFromString(e.Fee)
			rate, _ := decimal.NewFromString(e.ExchangeRate)
			if e.Type == investments.Buy {
				flow = amount.Add(fee).Mul(rate)
			} else {
				flow = amount.Sub(fee).Mul(rate).Neg()
			}
		default:
			known = false
			continue
		}
		capital = capital.Add(flow)
		weighted = weighted.Add(flow.Mul(decimal.NewFromInt(now - e.OccurredAt)).Div(decimal.NewFromInt(86400)))
	}
	summary, err := s.Summary(c, uid, false)
	if err != nil {
		return nil, err
	}
	ending := decimal.Zero
	for _, p := range summary.Positions {
		if !matches(p.AccountID, p.InstrumentID) {
			continue
		}
		if p.MarketValue == nil {
			known = false
		} else {
			v, _ := decimal.NewFromString(*p.MarketValue)
			ending = ending.Add(v)
		}
	}
	if known && weighted.IsPositive() {
		annual := ending.Sub(capital).Mul(decimal.NewFromInt(36500)).DivRound(weighted, 6)
		result.Annualized = decimalPointer(annual)
	}
	return result, nil
}
