package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

var depositRatePattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,2})(\.[0-9]{1,6})?$`)

func defaultAssetPreferences() *models.AssetPreferences {
	return &models.AssetPreferences{Revision: "0", Rules: map[string]models.AssetAccountRule{}, Reminders: models.AssetReminderSettings{Credit: true, Debt: true, Deposit: true, MinuteOfDay: 540}}
}
func readAssetPreferences(sess *xorm.Session, uid int64) (*models.AssetPreferences, error) {
	out := defaultAssetPreferences()
	row := new(models.AssetPresentation)
	has, err := sess.ID(uid).Get(row)
	if err != nil {
		return nil, err
	}
	if has {
		if err = json.Unmarshal([]byte(row.Payload), out); err != nil {
			return nil, err
		}
		out.Revision = strconv.FormatInt(row.Revision, 10)
	}
	if out.Rules == nil {
		out.Rules = map[string]models.AssetAccountRule{}
	}
	return out, nil
}
func (s *InvestmentService) AssetPreferences(c core.Context, uid int64) (*models.AssetPreferences, error) {
	var out *models.AssetPreferences
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error { var err error; out, err = readAssetPreferences(sess, uid); return err })
	return out, err
}
func assetRuleOwned(sess *xorm.Session, uid int64, key string) bool {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 || len(parts[1]) > 64 {
		return false
	}
	if parts[0] == "cash" {
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || id <= 0 {
			return false
		}
		has, err := sess.Where("uid=? AND account_id=? AND deleted=? AND system_role=?", uid, id, false, "").Exist(&models.Account{})
		return err == nil && has
	}
	if parts[0] == "portfolio" {
		has, err := sess.Where("uid=? AND id=?", uid, parts[1]).Exist(&models.PortfolioAccount{})
		return err == nil && has
	}
	return false
}
func (s *InvestmentService) SaveAssetPreferences(c core.Context, uid int64, input models.AssetPreferences) (*models.AssetPreferences, error) {
	if uid <= 0 || len(input.Rules) > 2000 || input.Reminders.AdvanceDays < 0 || input.Reminders.AdvanceDays > 30 || input.Reminders.MinuteOfDay < 0 || input.Reminders.MinuteOfDay > 1439 {
		return nil, investmentError("资产设置无效")
	}
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		current, err := readAssetPreferences(sess, uid)
		if err != nil {
			return err
		}
		if current.Revision != input.Revision {
			return investmentError("资产设置已更新，请刷新后重试")
		}
		var books []models.Book
		if err = sess.Where("uid=?", uid).Find(&books); err != nil {
			return err
		}
		valid := map[string]bool{}
		for _, b := range books {
			valid[b.Id] = true
		}
		valid[DefaultBookID(uid)] = true
		for key, rule := range input.Rules {
			if !assetRuleOwned(sess, uid, key) {
				delete(input.Rules, key)
				continue
			}
			if len(rule.DisabledBooks) > len(valid) {
				return investmentError("账本设置无效")
			}
			seen := map[string]bool{}
			for _, id := range rule.DisabledBooks {
				if !valid[id] || seen[id] {
					return investmentError("账本设置无效")
				}
				seen[id] = true
			}
		}
		revision, _ := strconv.ParseInt(current.Revision, 10, 64)
		revision++
		input.Revision = strconv.FormatInt(revision, 10)
		payload, err := json.Marshal(input)
		if err != nil {
			return err
		}
		row := &models.AssetPresentation{Uid: uid, Revision: revision, Payload: string(payload)}
		if current.Revision == "0" {
			_, err = sess.Insert(row)
		} else {
			_, err = sess.ID(uid).AllCols().Update(row)
		}
		return err
	})
	return &input, err
}

func depositAccount(sess *xorm.Session, uid int64, id string) (*models.Account, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return nil, investmentError("请选择资金或定存账户")
	}
	a := new(models.Account)
	has, err := sess.ID(n).Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Get(a)
	if err != nil {
		return nil, err
	}
	if !has || a.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || a.Category.IsLiability() || a.IsReimbursement() {
		return nil, investmentError("请选择资金或定存账户")
	}
	return a, nil
}

// Calendar month/year terms clamp to the final day of the target month.
func depositMaturity(start time.Time, term int, unit string) time.Time {
	if unit == "day" {
		return start.AddDate(0, 0, term)
	}
	months := term
	if unit == "year" {
		months *= 12
	}
	first := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location()).AddDate(0, months, 0)
	last := first.AddDate(0, 1, -1).Day()
	day := start.Day()
	if day > last {
		day = last
	}
	return first.AddDate(0, 0, day-1)
}
func calculateDeposit(input *models.FixedDeposit) error {
	if !calendarAmountPattern.MatchString(input.Principal) || len(input.Principal) > 32 || !depositRatePattern.MatchString(input.AnnualRate) || input.Term < 1 || input.Term > 36500 || !validCalendarDate(input.StartDate) || utf8.RuneCountInString(input.Note) > 200 || (input.Unit != "day" && input.Unit != "month" && input.Unit != "year") {
		return investmentError("请检查定存本金、利率、日期和期限")
	}
	zone, err := time.LoadLocation(input.TimeZone)
	if err != nil {
		return investmentError("会计时区无效")
	}
	at, _ := time.ParseInLocation("2006-01-02", input.StartDate, zone)
	maturity := depositMaturity(at, input.Term, input.Unit)
	if maturity.Year() > 9999 {
		return investmentError("定存期限过长")
	}
	p, _ := decimal.NewFromString(input.Principal)
	r, _ := decimal.NewFromString(input.AnnualRate)
	if !p.IsPositive() || p.Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) || r.GreaterThan(decimal.NewFromInt(100)) {
		return investmentError("定存金额或利率超出范围")
	}
	interest := p.Mul(r).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromInt(int64(input.Term)))
	if input.Unit == "day" {
		interest = interest.DivRound(decimal.NewFromInt(365), 18)
	} else if input.Unit == "month" {
		interest = interest.DivRound(decimal.NewFromInt(12), 18)
	}
	interest = interest.Round(2)
	if interest.Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
		return investmentError("定存收益超出范围")
	}
	input.Principal = p.StringFixed(2)
	input.AnnualRate = r.String()
	input.ExpectedInterest = interest.StringFixed(2)
	input.MaturityDate = maturity.Format("2006-01-02")
	return nil
}
func (s *InvestmentService) FixedDeposits(c core.Context, uid int64) ([]models.FixedDeposit, error) {
	items := make([]models.FixedDeposit, 0)
	err := s.UserDataDB(uid).DoTransaction(c,func(sess *xorm.Session) error {
		if err:=sess.Where("uid=?",uid).OrderBy("maturity_date asc,id asc").Find(&items);err!=nil{return err}
		for i:=range items {
			items[i].ReceivedInterest="0.00"
			if items[i].TransactionId==""{continue}
			var tx models.Transaction
			has,err:=sess.Where("uid=? AND transaction_id=? AND deleted=?",uid,items[i].TransactionId,false).Get(&tx)
			if err!=nil{return err};if has{items[i].ReceivedInterest=decimal.New(tx.Amount,-2).StringFixed(2)}
		}
		return nil
	})
	return items, err
}
func (s *InvestmentService) SaveFixedDeposit(c core.Context, uid int64, input models.FixedDeposit) (*models.FixedDeposit, error) {
	if uid <= 0 || len(input.Id) > 64 || len(input.BookId) > 64 || len(input.CategoryId) > 32 {
		return nil, investmentError("定存设置无效")
	}
	if err := calculateDeposit(&input); err != nil {
		return nil, err
	}
	defer s.lock(uid)()
	input.Uid = uid
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		a, err := depositAccount(sess, uid, input.AccountId)
		if err != nil {
			return err
		}
		input.Currency = a.Currency
		book, err := Books.ResolveInSession(sess, uid, input.BookId, false)
		if err != nil {
			return err
		}
		input.BookId = book
		category, err := strconv.ParseInt(input.CategoryId, 10, 64)
		if input.CategoryId=="" {
			category,err=assetIncomeCategory(sess,uid,book,models.CATEGORY_TYPE_INCOME,"投资收益")
			input.CategoryId=strconv.FormatInt(category,10)
		}
		if err != nil {
			return investmentError("请选择收入分类")
		}
		if err = Transactions.isCategoryValid(sess, &models.Transaction{Uid: uid, BookId: book, Type: models.TRANSACTION_DB_TYPE_INCOME, CategoryId: category}); err != nil {
			return err
		}
		if input.Id != "" {
			old := new(models.FixedDeposit)
			has, err := sess.ID(input.Id).Where("uid=? AND closed=?", uid, false).Get(old)
			if err != nil {
				return err
			}
			if !has || old.Settled {
				return investmentError("定存不存在或已入账，不能再改计息规则")
			}
			input.TransactionId = old.TransactionId
		} else {
			input.Id = "deposit-" + investmentID()
		}
		input.Settled = false
		input.Closed = false
		input.ClosedDate = ""
		input.TransactionId = ""
		// Principal remains part of the account. No expense or balance adjustment.
		exists, err := sess.ID(input.Id).Exist(&models.FixedDeposit{})
		if err != nil {
			return err
		}
		if exists {
			_, err = sess.ID(input.Id).Where("uid=?", uid).AllCols().Update(&input)
		} else {
			_, err = sess.Insert(&input)
		}
		if err != nil {
			return err
		}
		due := &models.CalendarEvent{Id: input.Id, Uid: uid, BookId: book, AccountId: input.AccountId, AccountName: a.Name, Currency: a.Currency, Kind: "deposit", Date: input.MaturityDate, Amount: input.Principal, Note: input.Note}
		exists, err = sess.ID(due.Id).Exist(&models.CalendarEvent{})
		if err != nil {
			return err
		}
		if exists {
			_, err = sess.ID(due.Id).Where("uid=?", uid).AllCols().Update(due)
		} else {
			_, err = sess.Insert(due)
		}
		return err
	})
	return &input, err
}
func (s *InvestmentService) CloseFixedDeposit(c core.Context, uid int64, id string) (bool, error) {
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		row := new(models.FixedDeposit)
		has, err := sess.ID(id).Where("uid=? AND closed=?", uid, false).Get(row)
		if err != nil {
			return err
		}
		if !has {
			return investmentError("定存不存在")
		}
		// Closing keeps the tombstone and any existing income; it never reopens settlement.
		zone, err := time.LoadLocation(row.TimeZone)
		if err != nil {
			return err
		}
		if _, err = sess.ID(id).Where("uid=?", uid).Cols("closed", "closed_date").Update(&models.FixedDeposit{Closed: true, ClosedDate: time.Now().In(zone).Format("2006-01-02")}); err != nil {
			return err
		}
		_, err = sess.ID(id).Where("uid=?", uid).Delete(&models.CalendarEvent{})
		return err
	})
	return err == nil, err
}
func (s *InvestmentService) SyncFixedDeposits(c core.Context, uid int64, now time.Time) (int, error) {
	defer s.lock(uid)()
	created := 0
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var items []models.FixedDeposit
		if err := sess.Where("uid=? AND closed=? AND settled=?", uid, false, false).Find(&items); err != nil {
			return err
		}
		for _, d := range items {
			zone, err := time.LoadLocation(d.TimeZone)
			if err != nil {
				return err
			}
			if d.MaturityDate > now.In(zone).Format("2006-01-02") {
				continue
			}
			a, err := depositAccount(sess, uid, d.AccountId)
			if err != nil {
				continue
			}
			if a.Currency != d.Currency {
				return investmentError("定存账户币种已改变，请处理定存后重试")
			}
			at, err := time.ParseInLocation("2006-01-02", d.MaturityDate, zone)
			if err != nil {
				return err
			}
			_, offset := at.Zone()
			category, _ := strconv.ParseInt(d.CategoryId, 10, 64)
			interest, err := decimal.NewFromString(d.ExpectedInterest)
			if err != nil {
				return err
			}
			if interest.IsPositive() {
				tx := &models.Transaction{Uid: uid, AccountId: a.AccountId, BookId: d.BookId, CategoryId: category, Type: models.TRANSACTION_DB_TYPE_INCOME, Amount: interest.Shift(2).IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Unix()), TimezoneUtcOffset: int16(offset / 60), Comment: fmt.Sprintf("定存到期收益：%s（%s）", a.Name, d.Id)}
				if err = Transactions.createTransactionInSession(c, sess, tx, nil, nil); err != nil {
					return err
				}
				d.TransactionId = strconv.FormatInt(tx.TransactionId, 10)
				created++
			}
			d.Settled = true
			if _, err = sess.ID(d.Id).Where("uid=?", uid).Cols("settled", "transaction_id").Update(&d); err != nil {
				return err
			}
			if _, err = sess.ID(d.Id).Where("uid=?", uid).Cols("completed").Update(&models.CalendarEvent{Completed: true}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}
