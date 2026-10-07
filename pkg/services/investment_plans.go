package services

import (
	"encoding/json"

	"strconv"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

func investmentDate(value string) bool {
	t, e := time.Parse("2006-01-02", value)
	return e == nil && len(value) == 10 && t.Year() >= 2000 && t.Year() <= 2200
}
func investmentTime(value string) bool {
	_, e := time.Parse("15:04", value)
	return e == nil && len(value) == 5
}
func (s *InvestmentService) validateFundTarget(sess *xorm.Session, uid int64, account, instrument, cash, book string) (string, error) {
	e := InvestmentEvent{Event: investments.Event{Type: investments.Buy, AccountID: account, InstrumentID: instrument, OccurredAt: time.Now().Unix(), Amount: "1", Fee: "0", ExchangeRate: "1"}, CashAccountID: cash}
	if err := s.validateEvent(sess, uid, &e); err != nil {
		return "", err
	}
	a := new(models.InvestmentInstrument)
	has, err := sess.Where("uid=? AND id=? AND type=?", uid, instrument, "FUND").Get(a)
	if err != nil {
		return "", err
	}
	if !has {
		return "", investmentError("基金确认和定投仅用于基金持仓")
	}
	id, _ := strconv.ParseInt(cash, 10, 64)
	bank := new(models.Account)
	if _, err = sess.Where("uid=? AND account_id=?", uid, id).Get(bank); err != nil {
		return "", err
	}
	if bank.Currency != "CNY" {
		return "", investmentError("基金确认请选择人民币付款或收款账户")
	}
	return Books.ResolveInSession(sess, uid, book, false)
}

func (s *InvestmentService) InvestmentPlans(c core.Context, uid int64) ([]models.InvestmentPlan, error) {
	items := []models.InvestmentPlan{}
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).Asc("start_date").Find(&items)
	return items, err
}
func (s *InvestmentService) SaveInvestmentPlan(c core.Context, uid int64, p models.InvestmentPlan) (*models.InvestmentPlan, error) {
	if !investmentDate(p.StartDate) || (p.EndDate != "" && (!investmentDate(p.EndDate) || p.EndDate < p.StartDate)) || !investmentTime(p.Time) || len(p.Note) > 1000 {
		return nil, investmentError("请核对定投日期、时间和备注")
	}
	if _, err := time.LoadLocation(p.TimeZone); err != nil || p.TimeZone == "" {
		return nil, investmentError("请选择有效时区")
	}
	if p.Cycle != "daily" && p.Cycle != "weekly" && p.Cycle != "biweekly" && p.Cycle != "monthly" {
		return nil, investmentError("请选择每天、每周、每两周或每月")
	}
	amount, err := investmentDecimal(p.Amount, "定投金额", true)
	if err != nil {
		return nil, err
	}
	if !amount.IsPositive() || amount.Exponent() < -2 {
		return nil, investmentError("定投金额应大于零且最多两位小数")
	}
	fee, err := investmentDecimal(p.FeePercent, "手续费比例", true)
	if err != nil {
		return nil, err
	}
	if fee.GreaterThan(decimal.NewFromInt(100)) {
		return nil, investmentError("手续费比例不能超过100%")
	}
	defer s.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var err error
		p.BookId, err = s.validateFundTarget(sess, uid, p.AccountId, p.InstrumentId, p.CashAccountId, p.BookId)
		if err != nil {
			return err
		}
		old := new(models.InvestmentPlan)
		has := false
		if p.Id != "" {
			has, err = sess.Where("uid=? AND id=?", uid, p.Id).Get(old)
			if err != nil {
				return err
			}
			if !has || old.Deleted || old.Version != p.Version {
				return ErrInvestmentConflict
			}
		}
		p.Uid = uid
		p.Version++
		p.Amount = amount.String()
		p.FeePercent = fee.String()
		if has {
			p.NextDate = old.NextDate
			if p.StartDate != old.StartDate || p.Cycle != old.Cycle {
				p.NextDate = p.StartDate
			}
			_, err = sess.Where("uid=? AND id=? AND version=?", uid, p.Id, old.Version).AllCols().Update(&p)
		} else {
			p.Id = investmentID()
			p.NextDate = p.StartDate
			_, err = sess.Insert(&p)
		}
		if err != nil {
			return err
		}
		if p.Deleted {
			_, err = sess.Where("uid=? AND plan_id=? AND status=?", uid, p.Id, "pending").Cols("status", "error").Update(&models.InvestmentOrder{Status: "cancelled", Error: "定投已删除"})
		} else {
			status := ""
			if p.Paused {
				status = "定投已暂停"
			}
			_, err = sess.Where("uid=? AND plan_id=? AND status=?", uid, p.Id, "pending").Cols("error").Update(&models.InvestmentOrder{Error: status})
		}
		return err
	})
	return &p, err
}

func nextInvestmentDate(date, cycle, start string) string {
	t, _ := time.Parse("2006-01-02", date)
	switch cycle {
	case "daily":
		t = t.AddDate(0, 0, 1)
	case "weekly":
		t = t.AddDate(0, 0, 7)
	case "biweekly":
		t = t.AddDate(0, 0, 14)
	case "monthly":
		anchor, _ := time.Parse("2006-01-02", start)
		next := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		last := next.AddDate(0, 1, -1).Day()
		day := anchor.Day()
		if day > last {
			day = last
		}
		t = time.Date(next.Year(), next.Month(), day, 0, 0, 0, 0, time.UTC)
	}
	return t.Format("2006-01-02")
}

func (s *InvestmentService) InvestmentOrders(c core.Context, uid int64) ([]models.InvestmentOrder, error) {
	items := []models.InvestmentOrder{}
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("CASE WHEN status='pending' THEN 0 ELSE 1 END, trade_date DESC, id DESC").Limit(1000).Find(&items)
	return items, err
}

func (s *InvestmentService) SaveInvestmentOrder(c core.Context, uid int64, o models.InvestmentOrder, key string) (*models.InvestmentOrder, error) {
	if len(key) < 8 || len(key) > 128 || !investmentDate(o.TradeDate) || (o.ConfirmDate != "" && (!investmentDate(o.ConfirmDate) || o.ConfirmDate < o.TradeDate)) || !investmentTime(o.Time) || len(o.Note) > 1000 {
		return nil, investmentError("请核对申请日期和确认日期")
	}
	if o.Type != investments.Buy && o.Type != investments.Sell {
		return nil, investmentError("待确认记录只支持基金买入或卖出")
	}
	if _, err := time.LoadLocation(o.TimeZone); err != nil || o.TimeZone == "" {
		return nil, investmentError("请选择有效时区")
	}
	if o.Fee == "" {
		o.Fee = "0"
	}
	if o.FeePercent == "" {
		o.FeePercent = "0"
	}
	for label, value := range map[string]string{"金额": o.Amount, "份额": o.Quantity, "手续费": o.Fee, "费率": o.FeePercent} {
		if value != "" {
			if _, err := investmentDecimal(value, label, true); err != nil {
				return nil, err
			}
		}
	}
	if o.Type == investments.Buy && o.Amount == "" && o.Quantity == "" || o.Type == investments.Sell && o.Quantity == "" {
		return nil, investmentError("买入请填写金额或份额，卖出请填写份额")
	}
	for _, value := range []string{o.Amount, o.Quantity} {
		if value != "" {
			d, _ := decimal.NewFromString(value)
			if !d.IsPositive() {
				return nil, investmentError("金额和份额必须大于零")
			}
		}
	}
	if o.Amount != "" && o.Quantity != "" {
		return nil, investmentError("金额与份额只能选择一种确认方式")
	}
	for _, value := range []string{o.Amount, o.Fee} {
		if value != "" {
			d, _ := decimal.NewFromString(value)
			if d.Exponent() < -2 {
				return nil, investmentError("人民币金额和手续费最多两位小数")
			}
		}
	}
	percent, _ := decimal.NewFromString(o.FeePercent)
	if percent.GreaterThan(decimal.NewFromInt(100)) {
		return nil, investmentError("手续费比例不能超过100%")
	}
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var err error
		o.BookId, err = s.validateFundTarget(sess, uid, o.AccountId, o.InstrumentId, o.CashAccountId, o.BookId)
		if err != nil {
			return err
		}
		old := new(models.InvestmentOrder)
		has, err := sess.Where("uid=? AND request_key=?", uid, key).Get(old)
		if err != nil {
			return err
		}
		if has {
			a, b := o, *old
			a.Id = b.Id
			a.Uid = b.Uid
			a.RequestKey = b.RequestKey
			a.Version = b.Version
			a.Status = b.Status
			a.LastAttempt = b.LastAttempt
			a.Error = b.Error
			a.Price = b.Price
			a.PriceDate = b.PriceDate
			a.EventId = b.EventId
			rawA, _ := json.Marshal(a)
			rawB, _ := json.Marshal(b)
			if string(rawA) != string(rawB) {
				return ErrInvestmentConflict
			}
			o = *old
			return nil
		}
		o.Id = investmentID()
		o.Uid = uid
		o.RequestKey = key
		o.PlanId = ""
		o.Status = "pending"
		o.EventId = ""
		o.Error = ""
		o.Price = ""
		o.PriceDate = ""
		o.LastAttempt = 0
		o.Version = 1
		_, err = sess.Insert(&o)
		return err
	})
	return &o, err
}

func (s *InvestmentService) CancelInvestmentOrder(c core.Context, uid int64, id string, version int) error {
	defer s.lock(uid)()
	n, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND id=? AND version=? AND status=?", uid, id, version, "pending").Cols("status", "version", "error").Update(&models.InvestmentOrder{Status: "cancelled", Version: version + 1})
	if err == nil && n != 1 {
		return ErrInvestmentConflict
	}
	return err
}

type InvestmentOrderConfirmation struct {
	Id      string `json:"id"`
	Version int    `json:"version"`
	Price   string `json:"price"`
	Date    string `json:"date"`
}

func (s *InvestmentService) ConfirmInvestmentOrder(c core.Context, uid int64, input InvestmentOrderConfirmation) (*InvestmentPreview, error) {
	return s.completeInvestmentOrder(c, uid, input, "手动确认", time.Now())
}

func (s *InvestmentService) completeInvestmentOrder(c core.Context, uid int64, input InvestmentOrderConfirmation, source string, now time.Time) (*InvestmentPreview, error) {
	price, err := investmentDecimal(input.Price, "确认净值", true)
	if err != nil {
		return nil, err
	}
	if !price.IsPositive() || !investmentDate(input.Date) {
		return nil, investmentError("请填写有效确认净值和日期")
	}
	defer s.lock(uid)()
	var result *InvestmentPreview
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		o := new(models.InvestmentOrder)
		has, err := sess.Where("uid=? AND id=?", uid, input.Id).Get(o)
		if err != nil {
			return err
		}
		if !has || o.Version != input.Version || o.Status != "pending" {
			return ErrInvestmentConflict
		}
		if o.PlanId != "" {
			p := new(models.InvestmentPlan)
			has, err := sess.Where("uid=? AND id=? AND paused=? AND deleted=?", uid, o.PlanId, false, false).Get(p)
			if err != nil {
				return err
			}
			if !has {
				return investmentError("定投已暂停或删除")
			}
		}
		zone, err := time.LoadLocation(o.TimeZone)
		if err != nil {
			return err
		}
		date := o.ConfirmDate
		if date == "" {
			date = o.TradeDate
		}
		if input.Date < date || input.Date > now.In(zone).Format("2006-01-02") {
			return investmentError("净值日期不能早于确认日期或晚于今天")
		}
		fee, _ := decimal.NewFromString(o.Fee)
		percent, _ := decimal.NewFromString(o.FeePercent)
		if percent.IsNegative() || percent.GreaterThan(decimal.NewFromInt(100)) {
			return investmentError("费率应在0到100之间")
		}
		var q, amount decimal.Decimal
		if o.Quantity != "" {
			q, err = investmentDecimal(o.Quantity, "份额", true)
			if err != nil {
				return err
			}
			amount = q.Mul(price).Round(2)
			if !percent.IsZero() {
				fee = amount.Mul(percent).DivRound(decimal.NewFromInt(100), 2)
			}
		} else {
			gross, err := investmentDecimal(o.Amount, "买入金额", true)
			if err != nil {
				return err
			}
			if percent.IsPositive() {
				amount = gross.DivRound(decimal.NewFromInt(1).Add(percent.Div(decimal.NewFromInt(100))), 2)
				fee = gross.Sub(amount)
			} else {
				amount = gross.Sub(fee)
			}
			q = amount.DivRound(price, 18).Truncate(2)
		}
		if !q.IsPositive() || !amount.IsPositive() || fee.IsNegative() || fee.Exponent() < -2 {
			return investmentError("净值或金额过小，无法形成有效份额")
		}
		at, _ := time.ParseInLocation("2006-01-02 15:04", input.Date+" "+o.Time, zone)
		if at.After(now) {
			return investmentError("尚未到定投或确认时间")
		}
		e := InvestmentEvent{Event: investments.Event{Type: o.Type, AccountID: o.AccountId, InstrumentID: o.InstrumentId, Quantity: q.String(), Amount: amount.String(), Fee: fee.String(), ExchangeRate: "1", OccurredAt: at.Unix(), Note: o.Note}, BookID: o.BookId, CashAccountID: o.CashAccountId, Fund: &FundConfirmation{TradeDate: o.TradeDate, ConfirmDate: input.Date, Price: price.String(), PriceDate: input.Date, Source: source, OrderId: o.Id}}
		result, err = s.mutateInvestmentInSession(c, sess, uid, e, "fund-order:"+o.Id, "create", false, nil)
		if err != nil {
			return err
		}
		o.Status = "completed"
		o.EventId = result.Event.ID
		o.Price = price.String()
		o.PriceDate = input.Date
		o.Error = ""
		o.Version++
		_, err = sess.Where("uid=? AND id=? AND version=?", uid, o.Id, input.Version).Cols("status", "event_id", "price", "price_date", "error", "version").Update(o)
		return err
	})
	return result, err
}

type InvestmentSyncResult struct {
	Created int `json:"created"`
	Pending int `json:"pending"`
}

func (s *InvestmentService) SyncInvestmentPlans(c core.Context, uid int64, force bool) (*InvestmentSyncResult, error) {
	return s.syncInvestmentPlansAt(c, uid, force, time.Now())
}

func (s *InvestmentService) syncInvestmentPlansAt(c core.Context, uid int64, force bool, now time.Time) (*InvestmentSyncResult, error) {
	// 先生成永久的每期指令，再在事务外读取对应日期的公开净值。
	unlock := s.lock(uid)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		plans := []models.InvestmentPlan{}
		if err := sess.Where("uid=? AND paused=? AND deleted=?", uid, false, false).Find(&plans); err != nil {
			return err
		}
		generated := 0
		for _, p := range plans {
			zone, err := time.LoadLocation(p.TimeZone)
			if err != nil {
				return err
			}
			today := now.In(zone).Format("2006-01-02")
			date := p.NextDate
			if date == "" {
				date = p.StartDate
			}
			for date <= today && (p.EndDate == "" || date <= p.EndDate) && generated < 200 {
				at, _ := time.ParseInLocation("2006-01-02 15:04", date+" "+p.Time, zone)
				if at.After(now) {
					break
				}
				key := "plan:" + p.Id + ":" + date
				exists, err := sess.Where("uid=? AND request_key=?", uid, key).Exist(&models.InvestmentOrder{})
				if err != nil {
					return err
				}
				if !exists {
					o := models.InvestmentOrder{Id: investmentID(), Uid: uid, RequestKey: key, PlanId: p.Id, AccountId: p.AccountId, InstrumentId: p.InstrumentId, CashAccountId: p.CashAccountId, BookId: p.BookId, Type: investments.Buy, Amount: p.Amount, Fee: "0", FeePercent: p.FeePercent, TradeDate: date, Time: p.Time, TimeZone: p.TimeZone, Note: p.Note, Status: "pending", Version: 1}
					if _, err = sess.Insert(&o); err != nil {
						return err
					}
				}
				date = nextInvestmentDate(date, p.Cycle, p.StartDate)
				generated++
			}
			if date != p.NextDate {
				if _, err = sess.Where("uid=? AND id=?", uid, p.Id).Cols("next_date").Update(&models.InvestmentPlan{NextDate: date}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	unlock()
	if err != nil {
		return nil, err
	}
	plans, err := s.InvestmentPlans(c, uid)
	if err != nil {
		return nil, err
	}
	activeIDs := []string{""}
	for _, p := range plans {
		if !p.Paused {
			activeIDs = append(activeIDs, p.Id)
		}
	}
	orders := []models.InvestmentOrder{}
	query := s.UserDataDB(uid).NewSession(c).Where("uid=? AND status=?", uid, "pending").In("plan_id", activeIDs)
	if !force {
		query = query.And("last_attempt<=?", now.Unix()-3600)
	}
	if err = query.Asc("trade_date", "id").Limit(200).Find(&orders); err != nil {
		return nil, err
	}
	pendingCount, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND status=?", uid, "pending").Count(&models.InvestmentOrder{})
	if err != nil {
		return nil, err
	}
	assets, err := s.Instruments(c, uid)
	if err != nil {
		return nil, err
	}
	byID := map[string]models.InvestmentInstrument{}
	for _, a := range assets {
		byID[a.Id] = a
	}
	result := &InvestmentSyncResult{Pending: int(pendingCount)}
	deadline := time.Now().Add(15 * time.Second)
	fetches := 0
	cache := map[string][]marketquotes.FundNAV{}
	for _, o := range orders {
		if time.Now().After(deadline) {
			break
		}
		if !force && now.Unix()-o.LastAttempt < 3600 {
			continue
		}
		asset := byID[o.InstrumentId]
		status := "等待对应日期的基金净值，可手动确认"
		zone, e := time.LoadLocation(o.TimeZone)
		if e != nil {
			continue
		}
		today := now.In(zone).Format("2006-01-02")
		from := o.ConfirmDate
		if from == "" {
			from = o.TradeDate
			if o.Time >= "15:00" {
				d, _ := time.Parse("2006-01-02", from)
				from = d.AddDate(0, 0, 1).Format("2006-01-02")
			}
		}
		if from > today {
			continue
		}
		if asset.Provider == "eastmoney" && asset.Market == "CN_FUND" {
			start, _ := time.Parse("2006-01-02", from)
			to := start.AddDate(0, 0, 30).Format("2006-01-02")
			if to > today {
				to = today
			}
			key := asset.ProviderID + ":" + from + ":" + to
			navs, ok := cache[key]
			var fetchErr error
			if !ok {
				if fetches >= 6 {
					break
				}
				fetches++
				navs, fetchErr = marketquotes.Default.FundHistory(investmentContext(c), asset.ProviderID, from, to)
				cache[key] = navs
			}
			if fetchErr != nil {
				status = "净值暂时无法获取，可稍后重试或手动确认"
			} else if len(navs) > 0 {
				_, e = s.completeInvestmentOrder(c, uid, InvestmentOrderConfirmation{Id: o.Id, Version: o.Version, Price: navs[0].Price, Date: navs[0].Date}, "天天基金已公布净值", now)
				if e == nil {
					result.Created++
					result.Pending--
					continue
				}
				status = e.Error()
			}
		}
		if len(status) > 300 {
			status = "本期尚未入账，请检查账户和基金净值"
		}
		_, err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND id=? AND version=? AND status=?", uid, o.Id, o.Version, "pending").Cols("last_attempt", "error").Update(&models.InvestmentOrder{LastAttempt: now.Unix(), Error: status})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
