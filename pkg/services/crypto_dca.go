package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

const cryptoDCAPausedBalance = "余额不足，已自动暂停；补足后请手动恢复"

var errCryptoDCAInsufficient = errors.New("crypto DCA insufficient balance")

type CryptoDCAInfo struct {
	PlanId string `json:"planId"`
	Date   string `json:"date"`
	marketquotes.CryptoHistoricalQuote
}

type CryptoDCAAlert struct {
	AccountId           string `json:"accountId"`
	AccountName         string `json:"accountName"`
	PaymentInstrumentId string `json:"paymentInstrumentId"`
	Balance             string `json:"balance"`
	Required            string `json:"required"`
	Paused              bool   `json:"paused"`
}

type CryptoDCAState struct {
	Plans   []models.CryptoDCAPlan `json:"plans"`
	Days    []models.CryptoDCADay  `json:"days"`
	Alerts  []CryptoDCAAlert       `json:"alerts"`
	Created int                    `json:"created"`
}

func cryptoDCAScheduled(p models.CryptoDCAPlan, date string) (time.Time, error) {
	zone, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return time.Time{}, investmentError("定投时区无效")
	}
	wanted := date + " " + p.DailyTime
	at, err := time.ParseInLocation("2006-01-02 15:04", wanted, zone)
	if err != nil {
		return time.Time{}, investmentError("定投日期或时间无效")
	}
	if at.Format("2006-01-02 15:04") != wanted {
		// A nonexistent local time on a DST transition runs at the first
		// available minute after the gap. The date key still permits only once.
		day, _ := time.ParseInLocation("2006-01-02", date, zone)
		for at = day; at.Before(day.AddDate(0, 0, 1)); at = at.Add(time.Minute) {
			if at.Format("2006-01-02 15:04") >= wanted {
				return at, nil
			}
		}
		return time.Time{}, investmentError("该日期在定投时区中不存在")
	}
	return at, nil
}

func cryptoDCANextDate(date string) string {
	d, _ := time.Parse("2006-01-02", date)
	return d.AddDate(0, 0, 1).Format("2006-01-02")
}

func cryptoDCAFutureDate(p models.CryptoDCAPlan, now time.Time) string {
	zone, _ := time.LoadLocation(p.TimeZone)
	date := now.In(zone).Format("2006-01-02")
	if date < p.StartDate {
		date = p.StartDate
	}
	at, err := cryptoDCAScheduled(p, date)
	if err == nil && !at.After(now) {
		date = cryptoDCANextDate(date)
	}
	return date
}

func (s *InvestmentService) SaveCryptoDCA(c core.Context, uid int64, req models.CryptoDCASaveRequest) (*models.CryptoDCAPlan, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if req.Id == "" && (len(req.RequestKey) < 8 || len(req.RequestKey) > 64) {
		return nil, investmentError("新定投计划缺少有效请求标识")
	}
	if len(req.Id) > 64 || len(req.AccountId) > 64 || len(req.BookId) > 64 || len(req.TimeZone) > 64 || req.TimeZone == "" || !marketquotes.SupportsCryptoDCA(req.PaymentInstrumentId, req.InstrumentId) || investments.ValidateDecimal(req.Amount) != nil || !validCalendarDate(req.StartDate) || req.StartDate < "2015-01-01" || len(req.DailyTime) != 5 {
		return nil, investmentError("请检查定投账户、币种、数量、日期和时间")
	}
	amount, _ := decimal.NewFromString(req.Amount)
	if !amount.IsPositive() {
		return nil, investmentError("每日定投数量必须大于零")
	}
	p := &models.CryptoDCAPlan{Id: req.Id, Uid: uid, AccountId: req.AccountId, InstrumentId: req.InstrumentId, PaymentInstrumentId: req.PaymentInstrumentId, Amount: amount.String(), DailyTime: req.DailyTime, StartDate: req.StartDate, NextDate: req.StartDate, TimeZone: req.TimeZone, BookId: req.BookId, Enabled: true, Revision: 1, Status: "已启用"}
	if _, err := cryptoDCAScheduled(*p, p.StartDate); err != nil {
		return nil, err
	}
	defer s.lock(uid)()
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if req.Id == "" {
			prior := new(models.InvestmentIdempotency)
			has, err := sess.Where("uid=? AND request_key=?", uid, "dca-plan:"+req.RequestKey).Get(prior)
			if err != nil {
				return err
			}
			if has {
				if prior.Digest != digest {
					return ErrInvestmentConflict
				}
				return json.Unmarshal([]byte(prior.Response), p)
			}
		}
		a := new(models.PortfolioAccount)
		has, err := sess.Where("uid=? AND id=?", uid, p.AccountId).Get(a)
		if err != nil {
			return err
		}
		if !has || a.Kind != "EXCHANGE" {
			return investmentError("每日定投仅支持当前用户的加密货币交易所账户")
		}
		p.BookId, err = Books.ResolveInSession(sess, uid, p.BookId, false)
		if err != nil {
			return err
		}
		if p.Id == "" {
			count, err := sess.Where("uid=?", uid).Count(&models.CryptoDCAPlan{})
			if err != nil {
				return err
			}
			if count >= 100 {
				return investmentError("定投计划数量已达上限")
			}
			p.Id = investmentID()
			if _, err = sess.Insert(p); err != nil {
				return err
			}
			response, _ := json.Marshal(p)
			_, err = sess.Insert(&models.InvestmentIdempotency{Id: investmentID(), Uid: uid, RequestKey: "dca-plan:" + req.RequestKey, Digest: digest, Response: string(response)})
			return err
		}
		old := new(models.CryptoDCAPlan)
		has, err = sess.Where("uid=? AND id=?", uid, p.Id).Get(old)
		if err != nil {
			return err
		}
		if !has || old.Revision != req.Revision {
			return ErrInvestmentConflict
		}
		if p.AccountId != old.AccountId {
			return investmentError("请在原交易所账户内调整定投")
		}
		if p.InstrumentId != old.InstrumentId || p.PaymentInstrumentId != old.PaymentInstrumentId {
			return investmentError("更换买入或支付币种请新建定投计划")
		}
		p.Revision = old.Revision + 1
		p.NextDate = cryptoDCAFutureDate(*p, time.Now())
		// Editing applies only to future occurrences. Successful dates, including
		// voided events, are never generated again even after moving today's time.
		for {
			exists, err := sess.ID(p.Id + ":" + p.NextDate).Exist(&models.CryptoDCADay{})
			if err != nil {
				return err
			}
			if !exists {
				break
			}
			p.NextDate = cryptoDCANextDate(p.NextDate)
		}
		if _, err = sess.Where("uid=? AND plan_id=? AND status=?", uid, p.Id, "pending").Cols("status", "message").Update(&models.CryptoDCADay{Status: "skipped", Message: "调整计划后停止旧安排"}); err != nil {
			return err
		}
		n, err := sess.Where("uid=? AND id=? AND revision=?", uid, p.Id, req.Revision).AllCols().Update(p)
		if err == nil && n != 1 {
			return ErrInvestmentConflict
		}
		return err
	})
	return p, err
}

func (s *InvestmentService) SetCryptoDCAEnabled(c core.Context, uid int64, id string, revision int64, enabled bool) (*models.CryptoDCAPlan, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	defer s.lock(uid)()
	p := new(models.CryptoDCAPlan)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		has, err := sess.Where("uid=? AND id=?", uid, id).Get(p)
		if err != nil {
			return err
		}
		if !has || p.Revision != revision {
			return ErrInvestmentConflict
		}
		if p.Enabled == enabled {
			return nil
		}
		if enabled {
			has, err = sess.Where("uid=? AND id=? AND kind=?", uid, p.AccountId, "EXCHANGE").Exist(&models.PortfolioAccount{})
			if err != nil {
				return err
			}
			if !has {
				return investmentError("交易所账户不存在")
			}
			p.NextDate = cryptoDCAFutureDate(*p, time.Now())
			for {
				exists, err := sess.ID(p.Id + ":" + p.NextDate).Exist(&models.CryptoDCADay{})
				if err != nil {
					return err
				}
				if !exists {
					break
				}
				p.NextDate = cryptoDCANextDate(p.NextDate)
			}
			p.Status = "已恢复，从下次计划时间继续"
		} else {
			p.Status = "已手动暂停"
		}
		p.Enabled = enabled
		p.Revision++
		p.LastAttempt = 0
		if _, err = sess.Where("uid=? AND plan_id=? AND status=?", uid, p.Id, "pending").Cols("status", "message").Update(&models.CryptoDCADay{Status: "skipped", Message: "暂停期间不补买"}); err != nil {
			return err
		}
		n, err := sess.Where("uid=? AND id=? AND revision=?", uid, id, revision).AllCols().Update(p)
		if err == nil && n != 1 {
			return ErrInvestmentConflict
		}
		return err
	})
	return p, err
}

func (s *InvestmentService) CryptoDCAState(c core.Context, uid int64, now time.Time) (*CryptoDCAState, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	out := &CryptoDCAState{Plans: []models.CryptoDCAPlan{}, Days: []models.CryptoDCADay{}, Alerts: []CryptoDCAAlert{}}
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	var accounts []models.PortfolioAccount
	if err := sess.Where("uid=? AND kind=?", uid, "EXCHANGE").Find(&accounts); err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, a := range accounts {
		names[a.Id] = a.Name
	}
	var plans []models.CryptoDCAPlan
	if err := sess.Where("uid=?", uid).Asc("id").Find(&plans); err != nil {
		return nil, err
	}
	for _, p := range plans {
		if _, ok := names[p.AccountId]; ok {
			out.Plans = append(out.Plans, p)
		}
	}
	if len(out.Plans) == 0 {
		return out, nil
	}
	if err := sess.Where("uid=? AND status=?", uid, "pending").Asc("scheduled_at", "id").Limit(100).Find(&out.Days); err != nil {
		return nil, err
	}
	var recent []models.CryptoDCADay
	if err := sess.Where("uid=? AND status<>?", uid, "pending").Desc("scheduled_at", "id").Limit(100).Find(&recent); err != nil {
		return nil, err
	}
	out.Days = append(out.Days, recent...)
	events, err := readInvestmentEvents(sess, uid)
	if err != nil {
		return nil, err
	}
	replay, err := replayInvestments(events)
	if err != nil {
		return nil, err
	}
	balances := map[string]string{}
	for _, pos := range replay.Positions {
		balances[pos.AccountID+":"+pos.InstrumentID] = pos.Quantity
	}
	pools := map[string]*CryptoDCAAlert{}
	for _, p := range out.Plans {
		if !p.Enabled && p.Status != cryptoDCAPausedBalance {
			continue
		}
		key := p.AccountId + ":" + p.PaymentInstrumentId
		alert := pools[key]
		if alert == nil {
			balance := balances[key]
			if balance == "" {
				balance = "0"
			}
			alert = &CryptoDCAAlert{AccountId: p.AccountId, AccountName: names[p.AccountId], PaymentInstrumentId: p.PaymentInstrumentId, Balance: balance, Required: "0"}
			pools[key] = alert
		}
		if !p.Enabled {
			alert.Paused = true
			continue
		}
		zone, err := time.LoadLocation(p.TimeZone)
		if err != nil {
			return nil, err
		}
		end := now.In(zone).AddDate(0, 0, 3)
		date := cryptoDCAFutureDate(p, now)
		amount, _ := decimal.NewFromString(p.Amount)
		required, _ := decimal.NewFromString(alert.Required)
		for i := 0; i < 4; i++ {
			at, err := cryptoDCAScheduled(p, date)
			if err != nil {
				return nil, err
			}
			if at.After(end) {
				break
			}
			required = required.Add(amount)
			date = cryptoDCANextDate(date)
		}
		alert.Required = required.String()
	}
	for _, alert := range pools {
		balance, _ := decimal.NewFromString(alert.Balance)
		required, _ := decimal.NewFromString(alert.Required)
		if alert.Paused || balance.LessThan(required) {
			out.Alerts = append(out.Alerts, *alert)
		}
	}
	sort.Slice(out.Alerts, func(i, j int) bool {
		return out.Alerts[i].AccountId+out.Alerts[i].PaymentInstrumentId < out.Alerts[j].AccountId+out.Alerts[j].PaymentInstrumentId
	})
	return out, nil
}

// Creating pending occurrences never changes holdings. Network calls take
// place outside DB transactions; terms are fixed on each occurrence.
func (s *InvestmentService) prepareCryptoDCADays(c core.Context, uid int64, now time.Time) error {
	defer s.lock(uid)()
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var plans []models.CryptoDCAPlan
		if err := sess.Where("uid=? AND enabled=?", uid, true).Asc("id").Find(&plans); err != nil {
			return err
		}
		for _, p := range plans {
			has, err := sess.Where("uid=? AND id=? AND kind=?", uid, p.AccountId, "EXCHANGE").Exist(&models.PortfolioAccount{})
			if err != nil {
				return err
			}
			if !has {
				continue
			}
			for i := 0; i < 100; i++ {
				at, err := cryptoDCAScheduled(p, p.NextDate)
				if err != nil {
					return err
				}
				if at.Add(time.Minute).After(now) {
					break
				}
				day := &models.CryptoDCADay{Id: p.Id + ":" + p.NextDate, Uid: uid, PlanId: p.Id, Date: p.NextDate, ScheduledAt: at.Unix(), PlanRevision: p.Revision, AccountId: p.AccountId, InstrumentId: p.InstrumentId, PaymentInstrumentId: p.PaymentInstrumentId, Amount: p.Amount, BookId: p.BookId, Status: "pending", Message: "等待历史行情"}
				exists, err := sess.ID(day.Id).Exist(&models.CryptoDCADay{})
				if err != nil {
					return err
				}
				if !exists {
					if _, err = sess.Insert(day); err != nil {
						return err
					}
				}
				p.NextDate = cryptoDCANextDate(p.NextDate)
			}
			if _, err := sess.Where("uid=? AND id=? AND revision=? AND enabled=?", uid, p.Id, p.Revision, true).Cols("next_date").Update(&p); err != nil {
				return err
			}
		}
		return nil
	})
}

// Check available units at the scheduled time and at every later event, so a
// backfill cannot consume funds that a later recorded withdrawal already used.
func cryptoDCACheckBalance(events []InvestmentEvent, day models.CryptoDCADay) error {
	result, err := replayInvestments(events)
	if err != nil {
		return err
	}
	times := map[string]int64{}
	for _, e := range events {
		times[e.ID] = e.OccurredAt
	}
	balance := decimal.Zero
	amount, _ := decimal.NewFromString(day.Amount)
	checked := false
	for _, effect := range result.Effects {
		if times[effect.EventID] > day.ScheduledAt && !checked {
			if balance.LessThan(amount) {
				return errCryptoDCAInsufficient
			}
			checked = true
		}
		for _, leg := range effect.Legs {
			if leg.AccountID == day.AccountId && leg.InstrumentID == day.PaymentInstrumentId {
				delta, _ := decimal.NewFromString(leg.QuantityDelta)
				balance = balance.Add(delta)
			}
		}
		if checked && balance.LessThan(amount) {
			return errCryptoDCAInsufficient
		}
	}
	if balance.LessThan(amount) {
		return errCryptoDCAInsufficient
	}
	return nil
}

func (s *InvestmentService) markCryptoDCADay(c core.Context, uid int64, day models.CryptoDCADay, message string, pause bool) error {
	defer s.lock(uid)()
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		p := new(models.CryptoDCAPlan)
		has, err := sess.Where("uid=? AND id=? AND revision=? AND enabled=?", uid, day.PlanId, day.PlanRevision, true).Get(p)
		if err != nil {
			return err
		}
		if !has {
			return nil
		}
		if pause {
			p.Enabled = false
			p.Revision++
			p.Status = cryptoDCAPausedBalance
			if _, err = sess.Where("uid=? AND plan_id=? AND status=?", uid, p.Id, "pending").Cols("status", "message").Update(&models.CryptoDCADay{Status: "skipped", Message: "余额不足，计划已暂停，未发生扣款"}); err != nil {
				return err
			}
		} else {
			if _, err = sess.Where("uid=? AND id=? AND status=?", uid, day.Id, "pending").Cols("message", "last_attempt").Update(&models.CryptoDCADay{Message: message, LastAttempt: time.Now().Unix()}); err != nil {
				return err
			}
			p.Status = "有待处理定投，请查看执行记录"
		}
		p.LastAttempt = time.Now().Unix()
		_, err = sess.Where("uid=? AND id=? AND revision=?", uid, p.Id, day.PlanRevision).Cols("enabled", "revision", "status", "last_attempt").Update(p)
		return err
	})
}

func (s *InvestmentService) SyncCryptoDCA(c core.Context, uid int64, force bool) (*CryptoDCAState, error) {
	return s.syncCryptoDCAAt(c, uid, force, time.Now(), marketquotes.Default.HistoricalCryptoQuote)
}

func (s *InvestmentService) syncCryptoDCAAt(c core.Context, uid int64, force bool, now time.Time, quote func(context.Context, string, string, int64) (*marketquotes.CryptoHistoricalQuote, error)) (*CryptoDCAState, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	lock := &s.dcaSyncLocks[uint64(uid)%64]
	if !lock.TryLock() {
		return s.CryptoDCAState(c, uid, now)
	}
	defer lock.Unlock()
	if err := s.prepareCryptoDCADays(c, uid, now); err != nil {
		return nil, err
	}
	var days []models.CryptoDCADay
	sess := s.UserDataDB(uid).NewSession(c)
	cutoff := now.Add(-15 * time.Minute).Unix()
	if force {
		cutoff = now.Add(-30 * time.Second).Unix()
	}
	err := sess.Where("uid=? AND status=? AND last_attempt<=?", uid, "pending", cutoff).Asc("scheduled_at", "id").Limit(30).Find(&days)
	sess.Close()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(investmentContext(c), 20*time.Second)
	defer cancel()
	created := 0
	for _, day := range days {
		if ctx.Err() != nil {
			break
		}
		sess := s.UserDataDB(uid).NewSession(c)
		active, e := sess.Where("uid=? AND id=? AND revision=? AND enabled=?", uid, day.PlanId, day.PlanRevision, true).Exist(&models.CryptoDCAPlan{})
		events, readErr := readInvestmentEvents(sess, uid)
		sess.Close()
		if e != nil {
			return nil, e
		}
		if readErr != nil {
			return nil, readErr
		}
		if !active {
			continue
		}
		if e = cryptoDCACheckBalance(events, day); e != nil {
			if !errors.Is(e, errCryptoDCAInsufficient) {
				return nil, e
			}
			if e = s.markCryptoDCADay(c, uid, day, "", true); e != nil {
				return nil, e
			}
			continue
		}
		q, e := quote(ctx, day.PaymentInstrumentId, day.InstrumentId, day.ScheduledAt)
		if e != nil {
			if err = s.markCryptoDCADay(c, uid, day, "历史分钟行情或人民币汇率暂不可用，稍后重试", false); err != nil {
				return nil, err
			}
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if q == nil || q.PriceTime != day.ScheduledAt || !validDCAQuote(*q) {
			return nil, investmentError("定投历史行情无效")
		}
		amount, _ := decimal.NewFromString(day.Amount)
		pay, _ := decimal.NewFromString(q.PaymentPrice)
		target, _ := decimal.NewFromString(q.TargetPrice)
		fx, _ := decimal.NewFromString(q.FXRate)
		quantity := amount.Mul(pay).DivRound(target, 36).Truncate(18)
		if !quantity.IsPositive() {
			if err = s.markCryptoDCADay(c, uid, day, "买入数量小于最小精度，请调整计划数量", false); err != nil {
				return nil, err
			}
			continue
		}
		event := InvestmentEvent{Event: investments.Event{Type: investments.Buy, AccountID: day.AccountId, InstrumentID: day.InstrumentId, SettlementInstrumentID: day.PaymentInstrumentId, SettlementAccountID: day.AccountId, Quantity: quantity.String(), Amount: day.Amount, Fee: "0", ExchangeRate: pay.Mul(fx).Round(18).String(), OccurredAt: day.ScheduledAt, Note: "每日定投 · 公开行情参考记账"}, BookID: day.BookId, DCA: &CryptoDCAInfo{PlanId: day.PlanId, Date: day.Date, CryptoHistoricalQuote: *q}}
		_, e = s.mutate(c, uid, event, "crypto-dca:"+day.Id, "create", false, &day)
		if e == nil {
			created++
			continue
		}
		if errors.Is(e, ErrInvestmentConflict) {
			continue
		}
		if err = s.markCryptoDCADay(c, uid, day, "账本已有流水或账本设置冲突，请核对后重试", errors.Is(e, errCryptoDCAInsufficient)); err != nil {
			return nil, err
		}
	}
	out, err := s.CryptoDCAState(c, uid, now)
	if out != nil {
		out.Created = created
	}
	return out, err
}

func validDCAQuote(q marketquotes.CryptoHistoricalQuote) bool {
	for _, v := range []string{q.PaymentPrice, q.TargetPrice, q.FXRate} {
		if investments.ValidateDecimal(v) != nil {
			return false
		}
		d, _ := decimal.NewFromString(v)
		if !d.IsPositive() {
			return false
		}
	}
	return q.Source != "" && q.FXSource != "" && validCalendarDate(q.FXDate) && q.FXDate < time.Unix(q.PriceTime, 0).UTC().Format("2006-01-02")
}

// Called only from the investment mutation transaction after the write lock.
func validateCryptoDCAExecution(sess *xorm.Session, uid int64, day *models.CryptoDCADay, events []InvestmentEvent) error {
	current := new(models.CryptoDCADay)
	has, err := sess.Where("uid=? AND id=? AND status=?", uid, day.Id, "pending").Get(current)
	if err != nil {
		return err
	}
	if !has || *current != *day {
		return ErrInvestmentConflict
	}
	has, err = sess.Where("uid=? AND id=? AND enabled=? AND revision=?", uid, day.PlanId, true, day.PlanRevision).Exist(&models.CryptoDCAPlan{})
	if err != nil {
		return err
	}
	if !has {
		return ErrInvestmentConflict
	}
	return cryptoDCACheckBalance(events, *day)
}

func completeCryptoDCADay(sess *xorm.Session, uid int64, day *models.CryptoDCADay, eventID string) error {
	n, err := sess.Where("uid=? AND id=? AND status=?", uid, day.Id, "pending").Cols("status", "message", "event_id", "last_attempt").Update(&models.CryptoDCADay{Status: "done", Message: "已按历史参考价入账", EventId: eventID, LastAttempt: time.Now().Unix()})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInvestmentConflict
	}
	pending, err := sess.Where("uid=? AND plan_id=? AND status=?", uid, day.PlanId, "pending").Exist(&models.CryptoDCADay{})
	if err != nil {
		return err
	}
	status := "已启用"
	if pending {
		status = "有待处理定投，请查看执行记录"
	}
	_, err = sess.Where("uid=? AND id=?", uid, day.PlanId).Cols("status", "last_attempt").Update(&models.CryptoDCAPlan{Status: status, LastAttempt: time.Now().Unix()})
	return err
}
