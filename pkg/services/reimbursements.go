package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type ReimbursementReceiptView struct {
	Id            string `json:"id"`
	TransactionId string `json:"transactionId"`
	AccountId     string `json:"accountId"`
	Amount        string `json:"amount"`
	Time          int64  `json:"time"`
	Comment       string `json:"comment"`
}
type ReimbursementClaimView struct {
	TransactionId   string                     `json:"transactionId"`
	AccountId       string                     `json:"accountId"`
	SourceAccountId string                     `json:"sourceAccountId"`
	BookId          string                     `json:"bookId"`
	CategoryId      string                     `json:"categoryId"`
	Currency        string                     `json:"currency"`
	Amount          string                     `json:"amount"`
	Paid            string                     `json:"paid"`
	Pending         string                     `json:"pending"`
	Time            int64                      `json:"time"`
	Comment         string                     `json:"comment"`
	Closed          bool                       `json:"closed"`
	Ended           bool                       `json:"ended"`
	Receipts        []ReimbursementReceiptView `json:"receipts"`
}

func reimbursementPayments(sess *xorm.Session, uid, expenseID int64) ([]models.ReimbursementReceipt, map[int64]models.Transaction, error) {
	var rows []models.ReimbursementReceipt
	if err := sess.Where("uid=? AND expense_id=?", uid, expenseID).Find(&rows); err != nil {
		return nil, nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.IncomeId)
	}
	result := map[int64]models.Transaction{}
	if len(ids) == 0 {
		return rows, result, nil
	}
	var txs []models.Transaction
	if err := sess.Where("uid=? AND deleted=?", uid, false).In("transaction_id", ids).Find(&txs); err != nil {
		return nil, nil, err
	}
	for _, tx := range txs {
		result[tx.TransactionId] = tx
	}
	return rows, result, nil
}

func reimbursementPaid(sess *xorm.Session, uid, expenseID, exceptIncome int64) (int64, error) {
	_, txs, err := reimbursementPayments(sess, uid, expenseID)
	if err != nil {
		return 0, err
	}
	var paid int64
	for _, tx := range txs {
		if tx.TransactionId == exceptIncome {
			continue
		}
		var ok bool
		paid, ok = utils.AddInt64(paid, tx.Amount)
		if !ok {
			return 0, errs.ErrAccountBalanceOverflow
		}
	}
	return paid, nil
}

func validateReimbursementTransaction(sess *xorm.Session, tx, old *models.Transaction) error {
	if old != nil {
		tx.ReimbursementReceiptId = old.ReimbursementReceiptId
		tx.ReimbursementClosedAt = old.ReimbursementClosedAt
		if old.ReimbursementAccountId > 0 {
			paid, err := reimbursementPaid(sess, tx.Uid, old.TransactionId, 0)
			if err != nil {
				return err
			}
			if paid > 0 && (tx.ReimbursementAccountId != old.ReimbursementAccountId || tx.Type != models.TRANSACTION_DB_TYPE_EXPENSE || tx.Amount < paid) {
				return investmentError("已有报销到账，修改前请先处理报销记录，支出不能小于已报金额")
			}
		}
	}
	if tx.ReimbursementAccountId > 0 {
		if tx.Type != models.TRANSACTION_DB_TYPE_EXPENSE || tx.Amount <= 0 || tx.InvestmentEventId != "" {
			return investmentError("只有普通正数支出可以申请报销")
		}
		var destination, source models.Account
		has, err := sess.ID(tx.ReimbursementAccountId).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", tx.Uid, false, false, "").Get(&destination)
		if err != nil {
			return err
		}
		if !has || !destination.IsReimbursement() {
			return investmentError("请选择有效的报销账户")
		}
		has, err = sess.ID(tx.AccountId).Where("uid=? AND deleted=? AND system_role=?", tx.Uid, false, "").Get(&source)
		if err != nil {
			return err
		}
		if !has || source.IsReimbursement() || source.Currency != destination.Currency {
			return investmentError("支出账户与报销账户须使用相同币种")
		}
	}
	if old != nil && tx.ReimbursementReceiptId != "" {
		var receipt models.ReimbursementReceipt
		has, err := sess.ID(tx.ReimbursementReceiptId).Where("uid=? AND income_id=?", tx.Uid, tx.TransactionId).Get(&receipt)
		if err != nil {
			return err
		}
		if !has {
			return investmentError("报销记录不存在")
		}
		var expense models.Transaction
		has, err = sess.ID(receipt.ExpenseId).Where("uid=? AND deleted=?", tx.Uid, false).Get(&expense)
		if err != nil {
			return err
		}
		if !has || tx.Type != models.TRANSACTION_DB_TYPE_INCOME || tx.Amount <= 0 || tx.ReimbursementAccountId != 0 {
			return investmentError("报销到账须保留为正数到账记录")
		}
		paid, err := reimbursementPaid(sess, tx.Uid, expense.TransactionId, tx.TransactionId)
		if err != nil {
			return err
		}
		if tx.Amount > expense.Amount-paid {
			return investmentError("报销到账金额不能超过剩余可报金额")
		}
		var cash, source models.Account
		if _, err = sess.ID(tx.AccountId).Where("uid=? AND deleted=?", tx.Uid, false).Get(&cash); err != nil {
			return err
		}
		if _, err = sess.ID(expense.AccountId).Where("uid=?", tx.Uid).Get(&source); err != nil {
			return err
		}
		if cash.IsReimbursement() || cash.Currency != source.Currency || tx.TransactionTime < expense.TransactionTime {
			return investmentError("请检查到账账户币种和日期")
		}
		tx.ExcludeFromStatistics = true
	}
	return nil
}

// at==0 derives the current ledger balance; historical valuation uses the
// expense, receipt and closure times, never today's pending amount.
func reimbursementBalancesAt(sess *xorm.Session, uid, at int64) (map[int64]int64, error) {
	var claims []models.Transaction
	if err := sess.Where("uid=? AND deleted=? AND reimbursement_account_id>0", uid, false).Find(&claims); err != nil {
		return nil, err
	}
	balances := map[int64]int64{}
	for _, claim := range claims {
		if at > 0 && utils.GetUnixTimeFromTransactionTime(claim.TransactionTime) > at {
			continue
		}
		if claim.ReimbursementClosedAt > 0 && (at == 0 || claim.ReimbursementClosedAt <= at) {
			continue
		}
		_, payments, err := reimbursementPayments(sess, uid, claim.TransactionId)
		if err != nil {
			return nil, err
		}
		remaining := claim.Amount
		for _, tx := range payments {
			if at == 0 || utils.GetUnixTimeFromTransactionTime(tx.TransactionTime) <= at {
				remaining -= tx.Amount
			}
		}
		if remaining < 0 {
			return nil, investmentError("已报金额大于原支出，请核对报销记录")
		}
		n, ok := utils.AddInt64(balances[claim.ReimbursementAccountId], remaining)
		if !ok {
			return nil, errs.ErrAccountBalanceOverflow
		}
		balances[claim.ReimbursementAccountId] = n
	}
	return balances, nil
}

func refreshReimbursementBalances(sess *xorm.Session, uid int64) error {
	balances, err := reimbursementBalancesAt(sess, uid, 0)
	if err != nil {
		return err
	}
	var accounts []models.Account
	if err = sess.Where("uid=? AND deleted=? AND category=? AND system_role=?", uid, false, models.ACCOUNT_CATEGORY_RECEIVABLES, "").Find(&accounts); err != nil {
		return err
	}
	for _, a := range accounts {
		if !a.IsReimbursement() {
			continue
		}
		if _, err = sess.ID(a.AccountId).Where("uid=?", uid).Cols("balance", "updated_unix_time").Update(&models.Account{Balance: balances[a.AccountId], UpdatedUnixTime: time.Now().Unix()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *InvestmentService) ReimbursementClaims(c core.Context, uid int64) ([]ReimbursementClaimView, error) {
	out := make([]ReimbursementClaimView, 0)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var claims []models.Transaction
		if err := sess.Where("uid=? AND deleted=? AND reimbursement_account_id>0", uid, false).Desc("transaction_time").Find(&claims); err != nil {
			return err
		}
		for _, tx := range claims {
			var a models.Account
			if _, err := sess.ID(tx.AccountId).Where("uid=?", uid).Get(&a); err != nil {
				return err
			}
			row := ReimbursementClaimView{TransactionId: fmt.Sprint(tx.TransactionId), AccountId: fmt.Sprint(tx.ReimbursementAccountId), SourceAccountId: fmt.Sprint(tx.AccountId), BookId: tx.BookId, CategoryId: fmt.Sprint(tx.CategoryId), Currency: a.Currency, Amount: decimal.New(tx.Amount, -2).StringFixed(2), Time: utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), Comment: tx.Comment, Ended: tx.ReimbursementClosedAt > 0, Receipts: []ReimbursementReceiptView{}}
			receipts, payments, err := reimbursementPayments(sess, uid, tx.TransactionId)
			if err != nil {
				return err
			}
			var paid int64
			for _, r := range receipts {
				income, ok := payments[r.IncomeId]
				if !ok {
					continue
				}
				paid += income.Amount
				row.Receipts = append(row.Receipts, ReimbursementReceiptView{Id: r.Id, TransactionId: fmt.Sprint(r.IncomeId), AccountId: fmt.Sprint(income.AccountId), Amount: decimal.New(income.Amount, -2).StringFixed(2), Time: utils.GetUnixTimeFromTransactionTime(income.TransactionTime), Comment: income.Comment})
			}
			row.Closed = row.Ended || paid >= tx.Amount
			row.Paid = decimal.New(paid, -2).StringFixed(2)
			row.Pending = decimal.New(tx.Amount-paid, -2).StringFixed(2)
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

type ReimbursementReceiveRequest struct {
	ExpenseId string `json:"expenseId"`
	AccountId string `json:"accountId"`
	Amount    string `json:"amount"`
	Time      int64  `json:"time"`
	TimeZone  string `json:"timeZone"`
	Comment   string `json:"comment"`
	RequestId string `json:"requestId"`
}

func (s *InvestmentService) ReceiveReimbursement(c core.Context, uid int64, input ReimbursementReceiveRequest) (*models.ReimbursementReceipt, error) {
	expenseID, e1 := strconv.ParseInt(input.ExpenseId, 10, 64)
	cashID, e2 := strconv.ParseInt(input.AccountId, 10, 64)
	zone, e3 := time.LoadLocation(input.TimeZone)
	if uid <= 0 || e1 != nil || e2 != nil || e3 != nil || zone == nil || expenseID <= 0 || cashID <= 0 || !assetAmountPattern.MatchString(input.Amount) || len(input.Amount) > 20 || input.Time <= 0 || len(input.RequestId) < 8 || len(input.RequestId) > 64 || utf8.RuneCountInString(input.Comment) > 200 {
		return nil, investmentError("请检查报销到账金额、账户和日期")
	}
	amount, _ := decimal.NewFromString(input.Amount)
	if !amount.IsPositive() {
		return nil, investmentError("到账金额必须大于零")
	}
	minor := amount.Shift(2).IntPart()
	data, _ := json.Marshal(input)
	hash := sha256.Sum256(data)
	digest := hex.EncodeToString(hash[:])
	defer s.lock(uid)()
	row := new(models.ReimbursementReceipt)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		has, err := sess.Where("uid=? AND request_id=?", uid, input.RequestId).Get(row)
		if err != nil {
			return err
		}
		if has {
			if row.RequestDigest != digest {
				return investmentError("请求已使用，请刷新后重试")
			}
			return nil
		}
		var expense models.Transaction
		has, err = sess.ID(expenseID).Where("uid=? AND deleted=? AND reimbursement_account_id>0", uid, false).Get(&expense)
		if err != nil {
			return err
		}
		if !has || expense.ReimbursementClosedAt > 0 {
			return investmentError("请选择尚未结束的待报销账单")
		}
		paid, err := reimbursementPaid(sess, uid, expenseID, 0)
		if err != nil {
			return err
		}
		if minor > expense.Amount-paid {
			return investmentError("到账金额不能超过剩余可报金额")
		}
		var cash, source models.Account
		has, err = sess.ID(cashID).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", uid, false, false, "").Get(&cash)
		if err != nil {
			return err
		}
		if !has || cash.IsReimbursement() || cash.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
			return investmentError("请选择有效的到账账户")
		}
		if _, err = sess.ID(expense.AccountId).Where("uid=?", uid).Get(&source); err != nil {
			return err
		}
		if source.Currency != cash.Currency {
			return investmentError("到账账户须与报销账单使用相同币种")
		}
		if input.Time < utils.GetUnixTimeFromTransactionTime(expense.TransactionTime) {
			return investmentError("到账日期不能早于支出日期")
		}
		category, err := assetIncomeCategory(sess, uid, expense.BookId, models.CATEGORY_TYPE_INCOME, "报销到账")
		if err != nil {
			return err
		}
		_, offset := time.Unix(input.Time, 0).In(zone).Zone()
		row.Id = "reimb-" + investmentID()
		row.Uid = uid
		row.ExpenseId = expenseID
		row.RequestId = input.RequestId
		row.RequestUid = uid
		row.RequestDigest = digest
		tx := &models.Transaction{Uid: uid, BookId: expense.BookId, AccountId: cashID, CategoryId: category, Type: models.TRANSACTION_DB_TYPE_INCOME, Amount: minor, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(input.Time), TimezoneUtcOffset: int16(offset / 60), Comment: input.Comment, ExcludeFromStatistics: true, ReimbursementReceiptId: row.Id}
		if tx.Comment == "" {
			tx.Comment = "报销到账"
		}
		if err = Transactions.createTransactionInSession(c, sess, tx, nil, nil); err != nil {
			return err
		}
		row.IncomeId = tx.TransactionId
		if _, err = sess.Insert(row); err != nil {
			return err
		}
		return refreshReimbursementBalances(sess, uid)
	})
	return row, err
}

func (s *InvestmentService) SetReimbursementState(c core.Context, uid, expenseID int64, action string) (bool, error) {
	if uid <= 0 || expenseID <= 0 || (action != "end" && action != "reopen" && action != "unassign") {
		return false, investmentError("报销操作无效")
	}
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var tx models.Transaction
		has, err := sess.ID(expenseID).Where("uid=? AND deleted=? AND reimbursement_account_id>0", uid, false).Get(&tx)
		if err != nil {
			return err
		}
		if !has {
			return errs.ErrTransactionNotFound
		}
		tx.ReimbursementClosedAt = 0
		if action == "end" {
			tx.ReimbursementClosedAt = time.Now().Unix()
		}
		if action == "unassign" {
			paid, err := reimbursementPaid(sess, uid, expenseID, 0)
			if err != nil {
				return err
			}
			if paid > 0 {
				return investmentError("请先删除报销到账记录")
			}
			tx.ReimbursementAccountId = 0
		}
		if _, err = sess.ID(expenseID).Where("uid=?", uid).Cols("reimbursement_closed_at", "reimbursement_account_id").Update(&tx); err != nil {
			return err
		}
		if err = InvalidateWealthSnapshots(sess, uid, 0); err != nil {
			return err
		}
		return refreshReimbursementBalances(sess, uid)
	})
	return err == nil, err
}

// Automatic asset entries choose an existing usable leaf or create a named
// category in the same transaction. Users can subsequently edit that category.
func assetIncomeCategory(sess *xorm.Session, uid int64, book string, kind models.TransactionCategoryType, name string) (int64, error) {
	var categories []models.TransactionCategory
	if err := sess.Where("uid=? AND deleted=? AND hidden=? AND type=? AND parent_category_id>0 AND name=?", uid, false, false, kind, name).Find(&categories); err != nil {
		return 0, err
	}
	for _, cat := range categories {
		if len(cat.BookIds) == 0 || slices.Contains(cat.BookIds, book) {
			var parent models.TransactionCategory
			has, err := sess.ID(cat.ParentCategoryId).Where("uid=? AND deleted=? AND hidden=?", uid, false, false).Get(&parent)
			if err != nil {
				return 0, err
			}
			if has && (len(parent.BookIds) == 0 || slices.Contains(parent.BookIds, book)) {
				return cat.CategoryId, nil
			}
		}
	}
	now := time.Now().Unix()
	parent := &models.TransactionCategory{Uid: uid, CategoryId: TransactionCategories.GenerateUuid(uuid.UUID_TYPE_CATEGORY), Name: name, Type: kind, Icon: 2100, Color: "48aa98", BookIds: []string{}, CreatedUnixTime: now, UpdatedUnixTime: now}
	if parent.CategoryId <= 0 {
		return 0, errs.ErrSystemIsBusy
	}
	if _, err := sess.Insert(parent); err != nil {
		return 0, err
	}
	child := *parent
	child.CategoryId = TransactionCategories.GenerateUuid(uuid.UUID_TYPE_CATEGORY)
	child.ParentCategoryId = parent.CategoryId
	if child.CategoryId <= 0 {
		return 0, errs.ErrSystemIsBusy
	}
	if _, err := sess.Insert(&child); err != nil {
		return 0, err
	}
	return child.CategoryId, nil
}
