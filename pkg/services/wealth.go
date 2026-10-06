package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type InvestmentValuationQuote struct {
	marketquotes.Quote
	FXRate       string `json:"fxRate"`
	FXDate       string `json:"fxDate"`
	FXSource     string `json:"fxSource"`
	FXReceivedAt int64  `json:"fxReceivedAt"`
	FXState      string `json:"fxState"`
}
type ValuedPosition struct {
	investments.Position
	MarketValue   *string                   `json:"marketValue"`
	UnrealizedPNL *string                   `json:"unrealizedPnl"`
	Quote         *InvestmentValuationQuote `json:"quote"`
}
type WealthCashAccount struct {
	ExcludedFromTotal bool                 `json:"excludedFromTotal"`
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Currency          string               `json:"currency"`
	Balance           string               `json:"balance"`
	Value             *string              `json:"value"`
	Liability         bool                 `json:"liability"`
	FX                *marketquotes.FXRate `json:"fx,omitempty"`
}
type WealthSummary struct {
	FXRates         []marketquotes.FXRate `json:"fxRates"`
	BaseCurrency    string                `json:"baseCurrency"`
	NetAssets       *string               `json:"netAssets"`
	ValuedAssets    string                `json:"valuedAssets"`
	CashAssets      string                `json:"cashAssets"`
	InvestmentValue string                `json:"investmentValue"`
	Liabilities     string                `json:"liabilities"`
	MissingPrices   int                   `json:"missingPrices"`
	StalePrices     int                   `json:"stalePrices"`
	UnrealizedPNL   *string               `json:"unrealizedPnl"`
	RealizedPNL     *string               `json:"realizedPnl"`
	CostComplete    bool                  `json:"costComplete"`
	CashAccounts    []WealthCashAccount   `json:"cashAccounts"`
	Positions       []ValuedPosition      `json:"positions"`
}

type wealthObservation struct {
	Summary *WealthSummary             `json:"summary"`
	Quotes  []InvestmentValuationQuote `json:"quotes"`
	FX      []marketquotes.FXRate      `json:"fx"`
}

func decimalPointer(d decimal.Decimal) *string { v := d.String(); return &v }

func withValuationFX(q InvestmentValuationQuote, rates map[string]marketquotes.FXRate) InvestmentValuationQuote {
	q.FXRate, q.FXDate, q.FXSource, q.FXState, q.FXReceivedAt = "", "", "", "", 0
	if q.Currency == "CNY" {
		q.FXRate = "1"
	} else if fx, ok := rates[q.Currency]; ok {
		q.FXRate, q.FXDate, q.FXSource, q.FXState, q.FXReceivedAt = fx.Rate, fx.Date, fx.Source, fx.State, fx.ReceivedAt
	}
	return q
}

func (s *InvestmentService) quotesInSession(sess *xorm.Session, uid int64) ([]InvestmentValuationQuote, error) {
	quotes := make([]marketquotes.Quote, 0)
	for _, preset := range investmentPresets {
		q, ok := marketquotes.Default.Get(preset.Id)
		if !ok {
			q = marketquotes.Quote{InstrumentID: preset.Id, Currency: "USD", State: marketquotes.StateUnavailable}
		}
		quotes = append(quotes, q)
	}
	var own []models.InvestmentInstrument
	if err := sess.Where("uid=?", uid).Find(&own); err != nil {
		return nil, err
	}
	for _, instrument := range own {
		if instrument.Provider == "" {
			continue
		}
		binding := instrumentBinding(instrument)
		if err := marketquotes.Default.Register(binding); err != nil {
			continue
		}
		q, ok := marketquotes.Default.Get(binding.Key())
		if !ok {
			q = marketquotes.Quote{Currency: binding.Currency, State: marketquotes.StateUnavailable}
		}
		q.InstrumentID = instrument.Id
		quotes = append(quotes, q)
	}
	fxs := marketquotes.Default.FX()
	fxByCurrency := make(map[string]marketquotes.FXRate)
	for _, f := range fxs {
		if f.Quote == "CNY" {
			fxByCurrency[f.Base] = f
		}
	}
	result := make([]InvestmentValuationQuote, 0, len(quotes))
	for _, q := range quotes {
		result = append(result, withValuationFX(InvestmentValuationQuote{Quote: q}, fxByCurrency))
	}
	var manual []models.InvestmentQuote
	if err := sess.Where("uid=?", uid).Find(&manual); err != nil {
		return nil, err
	}
	for _, row := range manual {
		var q InvestmentValuationQuote
		if err := json.Unmarshal([]byte(row.Payload), &q); err != nil {
			return nil, err
		}
		// The manual unit price remains in its original currency. Live valuation
		// uses the latest FX; historical rebuilds use the FX saved in snapshots.
		q = withValuationFX(q, fxByCurrency)
		found := false
		for i, auto := range result {
			if auto.InstrumentID == q.InstrumentID {
				result[i] = q
				found = true
				break
			}
		}
		if !found {
			result = append(result, q)
		}
	}
	return result, nil
}
func (s *InvestmentService) Quotes(c core.Context, uid int64) ([]InvestmentValuationQuote, error) {
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	return s.quotesInSession(sess, uid)
}
func (s *InvestmentService) ManualQuote(c core.Context, uid int64, id, price string, asOf int64, currency string) (any, error) {
	if currency == "" {
		currency = "CNY" // Existing API clients and saved CNY quotes keep their meaning.
	}
	if currency != "CNY" && currency != "USD" {
		return nil, investmentError("手动报价仅支持人民币或美元")
	}
	if asOf <= 0 || asOf > time.Now().Unix()+60 {
		return nil, investmentError("请填写有效的报价时间")
	}
	if err := investments.ValidateDecimal(price); err != nil {
		return nil, investmentError("价格最多支持 18 位小数")
	}
	d, err := decimal.NewFromString(price)
	if err != nil || !d.IsPositive() {
		return nil, investmentError("价格必须大于零")
	}
	instruments, err := s.Instruments(c, uid)
	if err != nil {
		return nil, err
	}
	found := false
	for _, v := range instruments {
		if v.Id == id {
			if currency == "USD" && v.Type != "CRYPTO" {
				return nil, investmentError("仅加密货币支持美元手动报价")
			}
			found = true
			break
		}
	}
	if !found {
		return nil, investmentError("资产不存在")
	}
	q := InvestmentValuationQuote{Quote: marketquotes.Quote{InstrumentID: id, Price: d.String(), Currency: currency, Source: "手动估值", SourceTime: asOf, ReceivedAt: time.Now().Unix(), State: "manual"}}
	if currency == "CNY" {
		q.FXRate = "1"
	}
	payload, _ := json.Marshal(q)
	defer s.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		row := &models.InvestmentQuote{Id: fmt.Sprintf("%d:%s", uid, id), Uid: uid, InstrumentId: id, Payload: string(payload)}
		has, e := sess.ID(row.Id).Exist(&models.InvestmentQuote{})
		if e != nil {
			return e
		}
		if has {
			_, e = sess.ID(row.Id).Cols("payload").Update(row)
		} else {
			_, e = sess.Insert(row)
		}
		return e
	})
	return q, err
}
func (s *InvestmentService) RemoveManualQuote(c core.Context, uid int64, id string) error {
	_, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND instrument_id=?", uid, id).Delete(&models.InvestmentQuote{})
	return err
}

func buildWealth(result *investments.Result, cash []models.Account, quotes []InvestmentValuationQuote, fxs []marketquotes.FXRate) *WealthSummary {
	out := &WealthSummary{BaseCurrency: "CNY", CostComplete: true, RealizedPNL: result.RealizedPNL, CashAccounts: make([]WealthCashAccount, 0), Positions: make([]ValuedPosition, 0)}
	out.FXRates = append([]marketquotes.FXRate{}, fxs...)
	quoteMap := map[string]InvestmentValuationQuote{}
	for _, q := range quotes {
		quoteMap[q.InstrumentID] = q
	}
	fxMap := map[string]string{"CNY": "1"}
	fxDetails := map[string]marketquotes.FXRate{}
	for _, fx := range fxs {
		if fx.Quote == "CNY" && fx.Rate != "" {
			fxMap[fx.Base] = fx.Rate
			fxDetails[fx.Base] = fx
		}
	}
	assets, debts, investment, pnl := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	for _, account := range cash {
		if account.SystemRole != "" || account.Deleted || account.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
			continue
		}
		balance := decimal.New(account.Balance, -2)
		included := !account.ExcludedFromTotal()
		item := WealthCashAccount{ID: fmt.Sprint(account.AccountId), Name: account.Name, Currency: account.Currency, Balance: balance.String(), Liability: account.Category.IsLiability(), ExcludedFromTotal: !included}
		if fx, ok := fxDetails[account.Currency]; ok {
			item.FX = &fx
			if included && !balance.IsZero() && fx.State == "stale" {
				out.StalePrices++
			}
		}
		rate, ok := fxMap[account.Currency]
		if !ok && !balance.IsZero() {
			if included {
				out.MissingPrices++
			}
		} else {
			f := decimal.NewFromInt(1)
			if ok {
				f, _ = decimal.NewFromString(rate)
			}
			value := balance.Mul(f)
			item.Value = decimalPointer(value)
			if !included {
				// The balance remains available in the account's own detail page.
			} else if value.IsNegative() {
				debts = debts.Sub(value)
			} else {
				assets = assets.Add(value)
			}
		}
		out.CashAccounts = append(out.CashAccounts, item)
	}
	for _, position := range result.Positions {
		v := ValuedPosition{Position: position}
		quantity, _ := decimal.NewFromString(position.Quantity)
		if quantity.IsZero() {
			v.MarketValue = decimalPointer(decimal.Zero)
			v.UnrealizedPNL = decimalPointer(decimal.Zero)
			out.Positions = append(out.Positions, v)
			continue
		}
		if !position.CostKnown {
			out.CostComplete = false
		}
		q, ok := quoteMap[position.InstrumentID]
		if ok {
			v.Quote = &q
		}
		if !ok || q.Price == "" || q.FXRate == "" {
			out.MissingPrices++
			out.CostComplete = false
		} else {
			price, e1 := decimal.NewFromString(q.Price)
			fx, e2 := decimal.NewFromString(q.FXRate)
			if e1 != nil || e2 != nil || !price.IsPositive() || !fx.IsPositive() {
				out.MissingPrices++
				out.CostComplete = false
			} else {
				value := quantity.Mul(price).Mul(fx)
				v.MarketValue = decimalPointer(value)
				investment = investment.Add(value)
				if q.State == "stale" || q.FXState == "stale" {
					out.StalePrices++
				}
				if position.CostKnown && position.Cost != nil {
					cost, _ := decimal.NewFromString(*position.Cost)
					profit := value.Sub(cost)
					v.UnrealizedPNL = decimalPointer(profit)
					pnl = pnl.Add(profit)
				}
			}
		}
		out.Positions = append(out.Positions, v)
	}
	out.CashAssets = assets.String()
	out.Liabilities = debts.String()
	out.InvestmentValue = investment.String()
	total := assets.Sub(debts).Add(investment)
	out.ValuedAssets = total.String()
	if out.MissingPrices == 0 {
		out.NetAssets = decimalPointer(total)
	}
	if out.CostComplete {
		out.UnrealizedPNL = decimalPointer(pnl)
	}
	return out
}

// Reuse upstream currency providers for cash currencies beyond USD/CNY. Query
// before the wealth transaction because custom rates may use the same SQLite DB.
var InvestmentCashFXProvider func(core.Context, int64) (*models.LatestExchangeRateResponse, error)

func (s *InvestmentService) valuationFX(c core.Context, uid int64) []marketquotes.FXRate {
	result := marketquotes.Default.FX()
	var cash []models.Account
	if s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Find(&cash) != nil {
		return result
	}
	needed := false
	for _, a := range cash {
		if a.Currency != "CNY" && a.Currency != "USD" && a.Balance != 0 {
			needed = true
			break
		}
	}
	if !needed {
		return result
	}
	if InvestmentCashFXProvider == nil {
		return result
	}
	rates, err := InvestmentCashFXProvider(c, uid)
	if err != nil || rates == nil {
		return result
	}
	byCurrency := map[string]decimal.Decimal{rates.BaseCurrency: decimal.NewFromInt(1)}
	for _, r := range rates.ExchangeRates {
		d, err := decimal.NewFromString(r.Rate)
		if err == nil && d.IsPositive() {
			byCurrency[r.Currency] = d
		}
	}
	cny, ok := byCurrency["CNY"]
	if !ok {
		return result
	}
	state := "delayed"
	if rates.UpdateTime <= 0 || time.Now().Unix()-rates.UpdateTime > 7*86400 {
		state = "stale"
	}
	for currency, rate := range byCurrency {
		if currency == "CNY" || currency == "USD" {
			continue
		}
		result = append(result, marketquotes.FXRate{Base: currency, Quote: "CNY", Rate: cny.DivRound(rate, 36).String(), Source: rates.DataSource, Date: time.Unix(rates.UpdateTime, 0).UTC().Format("2006-01-02"), ReceivedAt: time.Now().Unix(), State: state})
	}
	return result
}

func (s *InvestmentService) Summary(c core.Context, uid int64, save bool) (*WealthSummary, error) {
	fxs := s.valuationFX(c, uid)
	defer s.lock(uid)()
	var out *WealthSummary
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		result, err := replayInvestments(events)
		if err != nil {
			return err
		}
		var cash []models.Account
		if err = sess.Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Find(&cash); err != nil {
			return err
		}
		quotes, err := s.quotesInSession(sess, uid)
		if err != nil {
			return err
		}
		fx := fxs
		out = buildWealth(result, cash, quotes, fx)
		if !save {
			return nil
		}
		now := time.Now().Unix()
		exists, err := sess.Where("uid=? AND recorded_at>? AND invalidated=?", uid, now-300, false).Exist(&models.WealthSnapshot{})
		if err != nil || exists {
			return err
		}
		payload, _ := json.Marshal(struct {
			Summary *WealthSummary             `json:"summary"`
			Quotes  []InvestmentValuationQuote `json:"quotes"`
			FX      []marketquotes.FXRate      `json:"fx"`
		}{out, quotes, fx})
		_, err = sess.Insert(&models.WealthSnapshot{Id: investmentID(), Uid: uid, RecordedAt: now, NetAssets: out.NetAssets, ValuedAssets: out.ValuedAssets, Complete: out.MissingPrices == 0, Payload: string(payload)})
		return err
	})
	return out, err
}
func (s *InvestmentService) History(c core.Context, uid int64) ([]models.WealthSnapshot, error) {
	if err := s.rebuildHistory(c, uid); err != nil {
		return nil, err
	}
	rows := make([]models.WealthSnapshot, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("recorded_at desc").Limit(2000).Find(&rows)
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	for i := range rows {
		if rows[i].Invalidated {
			rows[i].NetAssets = nil
			rows[i].Complete = false
		}
	}
	return rows, err
}

// Rebuild historical observations using only their saved prices/rates. This never
// backfills a period with today's prices or assumes today's quantity existed then.
func (s *InvestmentService) rebuildHistory(c core.Context, uid int64) error {
	defer s.lock(uid)()
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var rows []models.WealthSnapshot
		if err := sess.Where("uid=? AND invalidated=?", uid, true).Find(&rows); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		events, err := readInvestmentEvents(sess, uid)
		if err != nil {
			return err
		}
		var allAccounts []models.Account
		if err = sess.Where("uid=? AND system_role=?", uid, "").Find(&allAccounts); err != nil {
			return err
		}
		var transactions []models.Transaction
		if err = sess.Where("uid=? AND deleted=?", uid, false).OrderBy("transaction_time asc").Find(&transactions); err != nil {
			return err
		}
		for _, row := range rows {
			var observation wealthObservation
			if json.Unmarshal([]byte(row.Payload), &observation) != nil || observation.Summary == nil {
				continue
			}
			facts := make([]InvestmentEvent, 0)
			for _, e := range events {
				if e.OccurredAt <= row.RecordedAt {
					facts = append(facts, e)
				}
			}
			result, err := replayInvestments(facts)
			if err != nil {
				continue
			}
			balances := map[int64]int64{}
			for _, tx := range transactions {
				if tx.TransactionTime/1000 > row.RecordedAt {
					continue
				}
				switch tx.Type {
				case models.TRANSACTION_DB_TYPE_MODIFY_BALANCE:
					balances[tx.AccountId] = tx.Amount
				case models.TRANSACTION_DB_TYPE_INCOME, models.TRANSACTION_DB_TYPE_TRANSFER_IN:
					balances[tx.AccountId] += tx.Amount
				case models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
					balances[tx.AccountId] -= tx.Amount
				}
			}
			claims, err := reimbursementBalancesAt(sess, uid, row.RecordedAt)
			if err != nil {
				return err
			}
			cash := make([]models.Account, 0)
			for _, a := range allAccounts {
				balance, exists := balances[a.AccountId]
				if !exists && a.CreatedUnixTime > row.RecordedAt {
					continue
				}
				if a.Deleted && a.DeletedUnixTime <= row.RecordedAt {
					continue
				}
				a.Deleted = false
				a.Balance = balance
				if a.IsReimbursement() {
					a.Balance = claims[a.AccountId]
				}
				cash = append(cash, a)
			}
			rebuilt := buildWealth(result, cash, observation.Quotes, observation.FX)
			observation.Summary = rebuilt
			payload, _ := json.Marshal(observation)
			row.NetAssets = rebuilt.NetAssets
			row.ValuedAssets = rebuilt.ValuedAssets
			row.Complete = rebuilt.MissingPrices == 0
			row.Invalidated = false
			row.Payload = string(payload)
			if _, err = sess.ID(row.Id).Where("uid=?", uid).Cols("net_assets", "valued_assets", "complete", "invalidated", "payload").Update(&row); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *InvestmentService) Export(c core.Context, uid int64) (string, error) {
	events, err := s.Events(c, uid)
	if err != nil {
		return "", err
	}
	result, err := replayInvestments(events)
	if err != nil {
		return "", err
	}
	effectMap := map[string]investments.EventEffect{}
	for _, e := range result.Effects {
		effectMap[e.EventID] = e
	}
	var b strings.Builder
	b.WriteString("\ufeff")
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"记录ID", "类型", "投资账户ID", "转入投资账户ID", "资产ID", "数量", "成交金额", "手续费", "期初成本CNY", "结算资产ID", "结算投资账户ID", "资金账户ID", "历史折算率", "发生时间UTC", "版本", "已撤销", "备注", "现金变动", "已实现净损益CNY"})
	ptr := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	safe := func(v string) string {
		if len(v) > 0 && strings.ContainsAny(v[:1], "=+-@\t\r") {
			return "'" + v
		}
		return v
	}
	for _, e := range events {
		effect := effectMap[e.ID]
		_ = w.Write([]string{e.ID, e.Type, e.AccountID, e.ToAccountID, e.InstrumentID, e.Quantity, e.Amount, e.Fee, ptr(e.Cost), e.SettlementInstrumentID, e.SettlementAccountID, e.CashAccountID, e.ExchangeRate, time.Unix(e.OccurredAt, 0).UTC().Format(time.RFC3339), fmt.Sprint(e.Version), fmt.Sprint(e.Voided), safe(e.Note), effect.CashDelta, ptr(effect.RealizedPNL)})
	}
	w.Flush()
	return b.String(), w.Error()
}

var marketStart sync.Once

func (s *InvestmentService) StartMarketCache() {
	marketStart.Do(func() {
		var bound []models.InvestmentInstrument
		if s.UserDataDB(0).NewSession(nil).Where("provider<>?", "").Find(&bound) == nil {
			for _, item := range bound {
				_ = marketquotes.Default.Register(instrumentBinding(item))
			}
		}
		var rows []models.InvestmentQuote
		_ = s.UserDataDB(0).NewSession(nil).Where("uid=?", 0).Find(&rows)
		for _, r := range rows {
			var saved struct {
				Quotes []marketquotes.Quote  `json:"quotes"`
				FX     []marketquotes.FXRate `json:"fx"`
			}
			if json.Unmarshal([]byte(r.Payload), &saved) == nil {
				marketquotes.Default.Restore(saved.Quotes, saved.FX)
			}
		}
		marketquotes.Default.Start(context.Background())
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			counter := 0
			for range ticker.C {
				payload, _ := json.Marshal(struct {
					Quotes []marketquotes.Quote  `json:"quotes"`
					FX     []marketquotes.FXRate `json:"fx"`
				}{marketquotes.Default.Quotes(), marketquotes.Default.FX()})
				_ = s.UserDataDB(0).DoTransaction(nil, func(sess *xorm.Session) error {
					row := &models.InvestmentQuote{Id: "shared-market-cache", Uid: 0, InstrumentId: "shared", Payload: string(payload)}
					has, e := sess.ID(row.Id).Exist(&models.InvestmentQuote{})
					if e != nil {
						return e
					}
					if has {
						_, e = sess.ID(row.Id).Cols("payload").Update(row)
					} else {
						_, e = sess.Insert(row)
					}
					return e
				})
				counter++
				if counter%30 == 0 {
					var settings []models.InvestmentSettings
					if s.UserDataDB(0).NewSession(nil).Find(&settings) == nil {
						for _, setting := range settings {
							_, _ = s.Summary(nil, setting.Uid, true)
						}
					}
				}
			}
		}()
	})
}
