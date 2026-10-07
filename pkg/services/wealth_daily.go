package services

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

// Daily results reuse the cost engine but never replace acquisition costs or
// create ledger events. Public history lookup runs outside the database lock.
func enrichDailyWealth(sess *xorm.Session, uid int64, events []InvestmentEvent, quotes []InvestmentValuationQuote, fx []marketquotes.FXRate, out *WealthSummary, now time.Time) error {
	setting := models.InvestmentSettings{TimeZone: "Asia/Shanghai"}
	if _, err := sess.Where("uid=?", uid).Get(&setting); err != nil {
		return err
	}
	zone, err := time.LoadLocation(setting.TimeZone)
	if err != nil || setting.TimeZone == "" {
		zone, _ = time.LoadLocation("Asia/Shanghai")
	}
	local := now.In(zone)
	at := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone).Unix()
	var own []models.InvestmentInstrument
	if err := sess.Where("uid=?", uid).Find(&own); err != nil {
		return err
	}
	ids := map[string]string{}
	for _, p := range investmentPresets {
		ids[p.Id] = p.Id
	}
	for _, p := range own {
		if p.Type == "CRYPTO" && p.Provider != "" {
			ids[p.Id] = instrumentBinding(p).Key()
		}
	}
	prices := map[string]string{}
	references := map[string]marketquotes.DayQuote{}
	used := map[string]bool{}
	for _, p := range out.Positions {
		used[p.InstrumentID] = true
	}
	for _, q := range quotes {
		id, ok := ids[q.InstrumentID]
		if !ok || !used[q.InstrumentID] || q.State == "manual" {
			continue
		}
		if reference, found := marketquotes.Default.DayReference(id, at); found {
			prices[q.InstrumentID] = decimal.RequireFromString(reference.Price).Mul(decimal.RequireFromString(reference.FXRate)).String()
			references[q.InstrumentID] = reference
		}
	}
	facts := make([]investments.Event, 0, len(events))
	adjusted := map[string]bool{}
	for _, e := range events {
		facts = append(facts, e.Event)
		if !e.Voided && e.OccurredAt >= at && (e.Type == investments.Opening || e.Type == investments.Adjust) {
			adjusted[e.InstrumentID] = true
		}
	}
	day, err := investments.ReplayDay(facts, at, prices)
	if err != nil {
		return err
	}
	valued := buildWealth(day, nil, quotes, fx)
	byPosition := map[string]ValuedPosition{}
	for _, p := range valued.Positions {
		byPosition[p.AccountID+":"+p.InstrumentID] = p
	}
	for i := range out.Positions {
		p := &out.Positions[i]
		p.DayStart = at
		if reference, ok := references[p.InstrumentID]; ok {
			p.DailyReference = &reference
		}
		p.DailyReason = "缺少会计时区零点的历史报价或汇率"
		if _, ok := ids[p.InstrumentID]; !ok {
			p.DailyReason = "该资产暂无零点历史报价"
			continue
		}
		if adjusted[p.InstrumentID] {
			p.DailyReason = "今日有期初录入或持仓校准，收益暂不可比"
			continue
		}
		d := byPosition[p.AccountID+":"+p.InstrumentID]
		if d.Quantity != "0" && (d.Quote == nil || d.Quote.State == marketquotes.StateStale || d.Quote.State == marketquotes.StateUnavailable || d.Quote.State == "manual" || d.Quote.FXState == "stale") {
			p.DailyReason = "当前报价或汇率不可用"
			continue
		}
		if d.UnrealizedPNL != nil && d.RealizedPNL != nil {
			p.DailyPNL = decimalPointer(decimal.RequireFromString(*d.UnrealizedPNL).Add(decimal.RequireFromString(*d.RealizedPNL)))
			p.DailyReason = ""
		}
	}
	return nil
}
