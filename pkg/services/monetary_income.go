package services

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/monetaryincome"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type MonetaryIncomeService struct {
	ServiceUsingDB
	locks [64]sync.Mutex
}

var MonetaryIncome = &MonetaryIncomeService{ServiceUsingDB: ServiceUsingDB{container: datastore.Container}}
var ErrMonetaryBinding = errs.NewNormalError(24, 1, 400, "请检查人民币资金账户、货币基金、起始日期和收入分类")
var ErrMonetaryHistory = errs.NewNormalError(24, 2, 409, "已有收益记录，不能改写基金和起始日期；可暂停自动收益")
var ErrMonetaryMove = errs.NewNormalError(24, 3, 409, "账户存在自动收益记录，请保留原账户，通过转账迁移余额")

func (s *MonetaryIncomeService) lock(uid int64) func() {
	m := &s.locks[uint64(uid)%64]
	m.Lock()
	return m.Unlock
}

func monetaryAccount(sess *xorm.Session, uid, id int64) (*models.Account, error) {
	a := new(models.Account)
	has, err := sess.ID(id).Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Get(a)
	if err != nil {
		return nil, err
	}
	if !has || a.Hidden || a.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || a.Currency != "CNY" {
		return nil, ErrMonetaryBinding
	}
	switch a.Category {
	case models.ACCOUNT_CATEGORY_CASH, models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, models.ACCOUNT_CATEGORY_VIRTUAL, models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT:
	default:
		return nil, ErrMonetaryBinding
	}
	return a, nil
}

func (s *MonetaryIncomeService) List(c core.Context, uid int64) ([]models.MonetaryIncomeBinding, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	items := make([]models.MonetaryIncomeBinding, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("account_id").Find(&items)
	if err != nil {
		return nil, err
	}
	for i := range items {
		days := []models.MonetaryIncomeDay{}
		if err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND account_id=?", uid, items[i].AccountId).Asc("date").Find(&days); err != nil {
			return nil, err
		}
		ids := []int64{}
		for _, day := range days {
			ids = append(ids, day.TransactionId)
			items[i].LastPerTenThousand = day.PerTenThousand
		}
		total := decimal.Zero
		for start := 0; start < len(ids); start += 200 {
			end := start + 200
			if end > len(ids) {
				end = len(ids)
			}
			transactions := []models.Transaction{}
			if err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND account_id=? AND deleted=? AND type=?", uid, items[i].AccountId, false, models.TRANSACTION_DB_TYPE_INCOME).In("transaction_id", ids[start:end]).Find(&transactions); err != nil {
				return nil, err
			}
			for _, transaction := range transactions {
				total = total.Add(decimal.NewFromInt(transaction.Amount).Div(decimal.NewFromInt(100)))
			}
		}
		items[i].TotalIncome = total.String()
	}
	return items, nil
}

func (s *MonetaryIncomeService) Save(c core.Context, uid int64, req models.MonetaryIncomeSaveRequest) (*models.MonetaryIncomeBinding, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	id, e1 := strconv.ParseInt(req.AccountId, 10, 64)
	category, e2 := strconv.ParseInt(req.CategoryId, 10, 64)
	if req.CategoryId == "" {
		category, e2 = 0, nil
	}
	zone, e3 := time.LoadLocation(req.TimeZone)
	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	if e1 != nil || e2 != nil || e3 != nil || zone == nil || id <= 0 || category < 0 || len(req.Code) != 6 || len(req.TimeZone) > 64 || req.TimeZone == "" || len(req.BookId) > 64 || !validCalendarDate(req.StartDate) || req.StartDate < "2025-01-01" || req.StartDate > time.Now().In(shanghai).Format("2006-01-02") {
		return nil, ErrMonetaryBinding
	}
	// Resolve against the public catalogue; client-supplied names are not trusted.
	candidates, err := marketquotes.Default.SearchMonetaryFunds(investmentContext(c), req.Code)
	if err != nil {
		return nil, investmentError("暂时无法核验基金，请联网后重试")
	}
	name := ""
	for _, item := range candidates {
		if item.ProviderID == req.Code {
			name = item.Name
			break
		}
	}
	if name == "" {
		return nil, ErrMonetaryBinding
	}
	defer s.lock(uid)()
	item := new(models.MonetaryIncomeBinding)
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := monetaryAccount(sess, uid, id); err != nil {
			return err
		}
		has, err := sess.Where("uid=? AND account_id=?", uid, id).Get(item)
		if err != nil {
			return err
		}
		if req.BookId == "" && has {
			req.BookId = item.BookId
		}
		book, err := Books.ResolveInSession(sess, uid, req.BookId, false)
		if err != nil {
			return err
		}
		if req.CategoryId == "" && has {
			category = item.CategoryId
		}
		if category == 0 {
			category, err = assetIncomeCategory(sess, uid, book, models.CATEGORY_TYPE_INCOME, "投资收益")
			if err != nil {
				return err
			}
		}
		if err := Transactions.isCategoryValid(sess, &models.Transaction{Uid: uid, BookId: book, Type: models.TRANSACTION_DB_TYPE_INCOME, CategoryId: category}); err != nil {
			return err
		}
		if has && (item.Code != req.Code || item.StartDate != req.StartDate) {
			// Processed-day identities survive rebinding and cursor changes.
			// Changing the fund never rewrites an existing income transaction.
			item.NextDate = req.StartDate
		}
		if err := anchorMonetaryOpening(sess, uid, id, req.StartDate, zone); err != nil {
			return err
		}

		if !has {
			item.Id = "income-" + investmentID()
			item.NextDate = req.StartDate
		}
		if item.BookId != book || item.CategoryId != category {
			item.LastTransactionId = 0
		}
		item.Uid, item.AccountId, item.Code, item.Name = uid, id, req.Code, name
		item.StartDate, item.BookId, item.CategoryId, item.TimeZone = req.StartDate, book, category, req.TimeZone
		item.Enabled, item.LastAttempt, item.Status = req.Enabled, 0, "待同步"
		if !item.Enabled {
			item.Status = "已暂停"
		}
		if has {
			_, err = sess.ID(item.Id).Where("uid=?", uid).AllCols().Update(item)
		} else {
			_, err = sess.Insert(item)
		}
		return err
	})
	return item, err
}

func (s *MonetaryIncomeService) Pause(c core.Context, uid, id int64) (bool, error) {
	if uid <= 0 || id <= 0 {
		return false, ErrMonetaryBinding
	}
	defer s.lock(uid)()
	n, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND account_id=?", uid, id).Cols("enabled", "status").Update(&models.MonetaryIncomeBinding{Enabled: false, Status: "已暂停"})
	return n > 0, err
}

type MonetarySyncResult struct {
	Created  int                            `json:"created"`
	Bindings []models.MonetaryIncomeBinding `json:"bindings"`
}

// Sync is explicitly invoked by the authenticated app on open or by its user.
// Network reads finish before a short per-day ledger transaction begins.
func (s *MonetaryIncomeService) Sync(c core.Context, uid, accountID int64, force bool) (*MonetarySyncResult, error) {
	return s.syncAt(c, uid, accountID, force, time.Now())
}

func (s *MonetaryIncomeService) syncAt(c core.Context, uid, accountID int64, force bool, now time.Time) (*MonetarySyncResult, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	defer s.lock(uid)()
	items, err := s.List(c, uid)
	if err != nil {
		return nil, err
	}
	result := &MonetarySyncResult{Bindings: items}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	yesterday := now.In(zone).AddDate(0, 0, -1).Format("2006-01-02")
	for i := range result.Bindings {
		b := &result.Bindings[i]
		if !b.Enabled || (accountID > 0 && b.AccountId != accountID) || (!force && now.Unix()-b.LastAttempt < 3600) {
			continue
		}
		b.LastAttempt = now.Unix()
		status := "已同步至 " + yesterday
		if b.NextDate <= yesterday {
			start, err := time.Parse("2006-01-02", b.NextDate)
			if err != nil {
				return nil, err
			}
			end := start.AddDate(0, 0, 89).Format("2006-01-02")
			if end > yesterday {
				end = yesterday
			}
			yields, fetchErr := marketquotes.Default.MonetaryYields(investmentContext(c), b.Code, b.NextDate, end)
			if fetchErr != nil {
				status = "收益数据暂不可用，稍后重试"
			} else {
				byDay := map[string]string{}
				for _, y := range yields {
					byDay[y.Date] = y.PerTenThousand
				}
				for b.NextDate <= end {
					rate, ok := byDay[b.NextDate]
					if !ok {
						status = "等待 " + b.NextDate + " 的万份收益"
						break
					}
					n, err := s.settleDay(c, uid, b.Id, b.NextDate, rate)
					if err != nil {
						status = "该日未入账，请检查账户、账本和分类"
						if err == monetaryincome.ErrCalendar {
							status = err.Error()
						}
						if err == errNegativeMonetaryYield {
							status = err.Error()
						}
						break
					}
					result.Created += n
					date, _ := time.Parse("2006-01-02", b.NextDate)
					b.NextDate = date.AddDate(0, 0, 1).Format("2006-01-02")
					status = "已同步至 " + date.Format("2006-01-02")
				}
			}
		}
		b.Status = status
		_, err = s.UserDataDB(uid).NewSession(c).ID(b.Id).Where("uid=?", uid).Cols("last_attempt", "status").Update(b)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

var errNegativeMonetaryYield = fmt.Errorf("该日万份收益为负，请核对平台实际金额并手工处理")

func (s *MonetaryIncomeService) settleDay(c core.Context, uid int64, bindingID, date, rate string) (int, error) {
	created := 0
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		b := new(models.MonetaryIncomeBinding)
		has, err := sess.ID(bindingID).Where("uid=? AND enabled=?", uid, true).Get(b)
		if err != nil {
			return err
		}
		if !has || b.NextDate != date {
			return ErrMonetaryHistory
		}
		account, err := monetaryAccount(sess, uid, b.AccountId)
		if err != nil {
			return err
		}
		yield, err := decimal.NewFromString(rate)
		if err != nil {
			return err
		}
		if yield.IsNegative() {
			return errNegativeMonetaryYield
		}
		dayID := fmt.Sprintf("%d:%d:%s", uid, b.AccountId, date)
		exists, err := sess.ID(dayID).Exist(&models.MonetaryIncomeDay{})
		if err != nil {
			return err
		}
		if !exists {
			var txs []models.Transaction
			if err := sess.Where("uid=? AND account_id=? AND deleted=?", uid, b.AccountId, false).OrderBy("transaction_time asc").Find(&txs); err != nil {
				return err
			}
			flows := make([]monetaryincome.Flow, 0, len(txs))
			for i, tx := range txs {
				minor := tx.Amount
				switch tx.Type {
				case models.TRANSACTION_DB_TYPE_MODIFY_BALANCE:
					minor = tx.RelatedAccountAmount
				case models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
					minor = -minor
				}
				flows = append(flows, monetaryincome.Flow{At: utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), Minor: minor, Opening: i == 0 && tx.Type == models.TRANSACTION_DB_TYPE_MODIFY_BALANCE})
			}
			principal, err := monetaryincome.Principal(date, flows)
			if err != nil {
				return err
			}
			var deposits []models.FixedDeposit
			if err := sess.Where("uid=? AND account_id=? AND start_date<=? AND maturity_date>?", uid, fmt.Sprint(b.AccountId), date, date).Find(&deposits); err != nil {
				return err
			}
			for _, d := range deposits {
				if d.Closed && (d.ClosedDate == "" || d.ClosedDate <= date) {
					continue
				}
				reserved, err := decimal.NewFromString(d.Principal)
				if err != nil {
					return err
				}
				principal = principal.Sub(reserved)
			}
			if principal.IsNegative() {
				principal = decimal.Zero
			}
			amount := monetaryincome.Income(principal, yield)
			minor := amount.Shift(2)
			if minor.GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
				return ErrMonetaryBinding
			}
			row := &models.MonetaryIncomeDay{Id: dayID, Uid: uid, AccountId: b.AccountId, Date: date, Code: b.Code, Principal: principal.StringFixed(2), PerTenThousand: yield.String(), Amount: amount.StringFixed(2)}
			// Reserve the permanent identity in the same atomic transaction as cash.
			if _, err = sess.Insert(row); err != nil {
				return err
			}
			if amount.IsPositive() {
				zone, err := time.LoadLocation(b.TimeZone)
				if err != nil {
					return err
				}
				at, err := time.ParseInLocation("2006-01-02", date, zone)
				if err != nil {
					return err
				}
				at = at.AddDate(0, 0, 1)
				_, offset := at.Zone()
				tx := &models.Transaction{Uid: uid, AccountId: b.AccountId, BookId: b.BookId, CategoryId: b.CategoryId, Type: models.TRANSACTION_DB_TYPE_INCOME, Amount: minor.IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Unix()), TimezoneUtcOffset: int16(offset / 60), Comment: date + " " + account.Name + "收益发放（" + b.Code + "）"}
				tags := []int64{}
				if b.LastTransactionId > 0 {
					previous := new(models.Transaction)
					ok, err := sess.ID(b.LastTransactionId).Where("uid=? AND deleted=? AND account_id=? AND type=?", uid, false, b.AccountId, models.TRANSACTION_DB_TYPE_INCOME).Get(previous)
					if err != nil {
						return err
					}
					if ok {
						tx.BookId, tx.CategoryId, tx.HideAmount = previous.BookId, previous.CategoryId, previous.HideAmount
						tx.ExcludeFromStatistics = previous.ExcludeFromStatistics
						var indexes []models.TransactionTagIndex
						if err = sess.Where("uid=? AND transaction_id=? AND deleted=?", uid, previous.TransactionId, false).Find(&indexes); err != nil {
							return err
						}
						for _, index := range indexes {
							tags = append(tags, index.TagId)
						}
					}
				}
				if err = Transactions.createTransactionInSession(c, sess, tx, tags, nil); err != nil {
					return err
				}
				row.TransactionId = tx.TransactionId
				if _, err = sess.ID(row.Id).Cols("transaction_id").Update(row); err != nil {
					return err
				}
				b.LastTransactionId = tx.TransactionId
				created = 1
			}
		}
		d, _ := time.Parse("2006-01-02", date)
		b.NextDate = d.AddDate(0, 0, 1).Format("2006-01-02")
		_, err = sess.ID(b.Id).Where("uid=?", uid).Cols("next_date", "last_transaction_id").Update(b)
		return err
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}

// Bulk deletion pauses the rule but retains processed-day tombstones. Neither
// later sync nor re-enabling may resurrect deleted income transactions.
func pauseMonetaryIncome(sess *xorm.Session, uid int64, accountIDs []int64) error {
	q := sess.Where("uid=?", uid)
	if len(accountIDs) > 0 {
		q = q.In("account_id", accountIDs)
	}
	_, err := q.Cols("enabled", "status").Update(&models.MonetaryIncomeBinding{Enabled: false, Status: "账务已清理，自动收益已暂停"})
	return err
}

func guardMonetaryMove(sess *xorm.Session, uid int64, ids []int64) error {
	linked, err := sess.Where("uid=?", uid).In("account_id", ids).Exist(&models.MonetaryIncomeBinding{})
	if err != nil {
		return err
	}
	if linked {
		return ErrMonetaryMove
	}
	return nil
}

// The chosen start date asserts that the opening balance was already held.
// Move only the opening anchor when necessary so next-day income can precede
// the time the user created the account. No amount or later flow is changed.
func anchorMonetaryOpening(sess *xorm.Session, uid, accountID int64, date string, zone *time.Location) error {
	var opening models.Transaction
	has, err := sess.Where("uid=? AND account_id=? AND deleted=?", uid, accountID, false).Asc("transaction_time").Get(&opening)
	if err != nil || !has || opening.Type != models.TRANSACTION_DB_TYPE_MODIFY_BALANCE {
		return err
	}
	start, err := time.ParseInLocation("2006-01-02", date, zone)
	if err != nil {
		return err
	}
	if utils.GetUnixTimeFromTransactionTime(opening.TransactionTime) < start.Unix() {
		return nil
	}
	at := utils.GetMinTransactionTimeFromUnixTime(start.Unix() - 1)
	var last models.Transaction
	if _, err = sess.Where("uid=? AND transaction_time>=? AND transaction_time<=?", uid, at, utils.GetMaxTransactionTimeFromUnixTime(start.Unix()-1)).Desc("transaction_time").Get(&last); err != nil {
		return err
	}
	if last.TransactionTime >= at {
		at = last.TransactionTime + 1
	}
	if at >= utils.GetMaxTransactionTimeFromUnixTime(start.Unix()-1) {
		return errs.ErrTooMuchTransactionInOneSecond
	}
	opening.TransactionTime = at
	if _, err = sess.ID(opening.TransactionId).Where("uid=?", uid).Cols("transaction_time").Update(&opening); err != nil {
		return err
	}
	return InvalidateWealthSnapshots(sess, uid, start.Unix()-1)
}
