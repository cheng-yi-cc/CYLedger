package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type DebtMovementInput struct {
	DebtAccountId string `json:"debtAccountId"`
	CashAccountId string `json:"cashAccountId"`
	Action        string `json:"action"`
	Principal     string `json:"principal"`
	Interest      string `json:"interest"`
	Time          int64  `json:"time"`
	TimeZone      string `json:"timeZone"`
	BookId        string `json:"bookId"`
	Note          string `json:"note"`
	RequestId     string `json:"requestId"`
}
type DebtReport struct {
	AccountId     string `json:"accountId"`
	PaidPrincipal string `json:"paidPrincipal"`
	Interest      string `json:"interest"`
	NextDate      string `json:"nextDate"`
}

func (s *InvestmentService) RecordDebtMovement(c core.Context, uid int64, input DebtMovementInput) (*models.DebtMovement, error) {
	debtID, e1 := strconv.ParseInt(input.DebtAccountId, 10, 64)
	cashID := int64(0)
	var e2 error
	if input.CashAccountId != "" {
		cashID, e2 = strconv.ParseInt(input.CashAccountId, 10, 64)
	}
	zone, e3 := time.LoadLocation(input.TimeZone)
	if uid <= 0 || e1 != nil || e2 != nil || e3 != nil || debtID <= 0 || cashID < 0 || input.Time <= 0 || input.Time > time.Now().Unix()+60 || input.TimeZone == "" || len(input.TimeZone) > 64 || len(input.RequestId) < 8 || len(input.RequestId) > 48 || len(input.BookId) > 64 || utf8.RuneCountInString(input.Note) > 200 || !calendarAmountPattern.MatchString(input.Principal) || !assetAmountPattern.MatchString(input.Interest) {
		return nil, investmentError("请检查借还款金额、账户和日期")
	}
	labels := map[string]string{"borrow": "借入", "lend": "借出", "repay": "还款", "collect": "收回"}
	label := labels[input.Action]
	principal, _ := decimal.NewFromString(input.Principal)
	interest, _ := decimal.NewFromString(input.Interest)
	if label == "" || !principal.IsPositive() || principal.Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) || interest.Abs().Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) || interest.Neg().GreaterThan(principal) || ((input.Action == "borrow" || input.Action == "lend") && !interest.IsZero()) {
		return nil, investmentError("本金须大于零，利息优惠不能超过本金")
	}
	if !interest.IsZero() && cashID == 0 {
		return nil, investmentError("记录利息时请选择实际资金账户")
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(raw)
	row := &models.DebtMovement{Id: fmt.Sprintf("%d:%s", uid, input.RequestId), Uid: uid, DebtAccountId: debtID, Action: input.Action, Digest: hex.EncodeToString(hash[:])}
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var existing models.DebtMovement
		has, e := sess.ID(row.Id).Where("uid=?", uid).Get(&existing)
		if e != nil {
			return e
		}
		if has {
			if existing.Digest != row.Digest {
				return investmentError("请求内容已变化，请重试")
			}
			*row = existing
			return nil
		}
		var debt, cash models.Account
		has, e = sess.ID(debtID).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", uid, false, false, "").Get(&debt)
		if e != nil {
			return e
		}
		expected := models.ACCOUNT_CATEGORY_DEBT
		if input.Action == "lend" || input.Action == "collect" {
			expected = models.ACCOUNT_CATEGORY_RECEIVABLES
		}
		if !has || debt.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || debt.Category != expected || debt.IsReimbursement() {
			return investmentError("借还款类型与往来账户不匹配")
		}
		if input.Action == "repay" && principal.GreaterThan(decimal.New(debt.Balance, -2).Neg()) || input.Action == "collect" && principal.GreaterThan(decimal.New(debt.Balance, -2)) {
			return investmentError("本次本金不能超过剩余待还或待收金额")
		}
		if cashID > 0 {
			has, e = sess.ID(cashID).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", uid, false, false, "").Get(&cash)
			if e != nil {
				return e
			}
			if !has || cash.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT || cash.Currency != debt.Currency || cash.IsReimbursement() || (cash.Category != models.ACCOUNT_CATEGORY_CASH && cash.Category != models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT && cash.Category != models.ACCOUNT_CATEGORY_VIRTUAL && cash.Category != models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT) {
				return investmentError("请选择同币种资金账户")
			}
		}
		book, e := Books.ResolveInSession(sess, uid, input.BookId, false)
		if e != nil {
			return e
		}
		_, offset := time.Unix(input.Time, 0).In(zone).Zone()
		tx := &models.Transaction{Uid: uid, BookId: book, Amount: principal.Shift(2).IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(input.Time), TimezoneUtcOffset: int16(offset / 60), Comment: input.Note}
		debtIsSource := input.Action == "borrow" || input.Action == "collect"
		if cashID > 0 {
			tx.Type = models.TRANSACTION_DB_TYPE_TRANSFER_OUT
			tx.RelatedAccountAmount = tx.Amount
			tx.AccountId = cashID
			tx.RelatedAccountId = debtID
			if debtIsSource {
				tx.AccountId = debtID
				tx.RelatedAccountId = cashID
			}
			tx.CategoryId, e = assetIncomeCategory(sess, uid, book, models.CATEGORY_TYPE_TRANSFER, label)
		} else {
			tx.AccountId = debtID
			tx.Type = models.TRANSACTION_DB_TYPE_INCOME
			kind := models.CATEGORY_TYPE_INCOME
			if debtIsSource {
				tx.Type = models.TRANSACTION_DB_TYPE_EXPENSE
				kind = models.CATEGORY_TYPE_EXPENSE
			}
			tx.ExcludeFromStatistics = true
			tx.CategoryId, e = assetIncomeCategory(sess, uid, book, kind, label+"（外部账户）")
		}
		if e != nil {
			return e
		}
		if e = Transactions.createTransactionInSession(c, sess, tx, nil, nil); e != nil {
			return e
		}
		row.PrincipalTransactionId = tx.TransactionId
		if !interest.IsZero() {
			income := input.Action == "collect"
			if interest.IsNegative() {
				income = !income
			}
			txType, kind := models.TRANSACTION_DB_TYPE_EXPENSE, models.CATEGORY_TYPE_EXPENSE
			if income {
				txType, kind = models.TRANSACTION_DB_TYPE_INCOME, models.CATEGORY_TYPE_INCOME
			}
			name := "借款利息"
			if input.Action == "collect" {
				name = "借出利息"
			}
			if interest.IsNegative() {
				name += "优惠"
			}
			category, e := assetIncomeCategory(sess, uid, book, kind, name)
			if e != nil {
				return e
			}
			fee := &models.Transaction{Uid: uid, BookId: book, AccountId: cashID, Type: txType, CategoryId: category, Amount: interest.Abs().Shift(2).IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(input.Time), TimezoneUtcOffset: int16(offset / 60), Comment: debt.Name + " · " + name}
			if e = Transactions.createTransactionInSession(c, sess, fee, nil, nil); e != nil {
				return e
			}
			row.InterestTransactionId = fee.TransactionId
		}
		_, e = sess.Insert(row)
		return e
	})
	return row, err
}

func (s *InvestmentService) DebtReports(c core.Context, uid int64) ([]DebtReport, error) {
	if uid <= 0 {
		return nil, investmentError("用户无效")
	}
	out := []DebtReport{}
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var accounts []models.Account
		if e := sess.Where("uid=? AND deleted=? AND system_role=?", uid, false, "").In("category", []int{5, 6}).Find(&accounts); e != nil {
			return e
		}
		var txs []models.Transaction
		if e := sess.Where("uid=? AND deleted=?", uid, false).Find(&txs); e != nil {
			return e
		}
		var movements []models.DebtMovement
		if e := sess.Where("uid=?", uid).Find(&movements); e != nil {
			return e
		}
		var due []models.CalendarEvent
		if e := sess.Where("uid=? AND completed=? AND kind=?", uid, false, "repayment").Asc("date").Find(&due); e != nil {
			return e
		}
		byID := map[int64]models.Transaction{}
		for _, tx := range txs {
			byID[tx.TransactionId] = tx
		}
		for _, a := range accounts {
			if a.IsReimbursement() {
				continue
			}
			paid, interest := decimal.Zero, decimal.Zero
			row := DebtReport{AccountId: fmt.Sprint(a.AccountId)}
			if p := a.AssetProfile(); p != nil {
				row.NextDate = p.DebtDueDate
			}
			for _, d := range due {
				if d.AccountId == row.AccountId && (row.NextDate == "" || d.Date < row.NextDate) {
					row.NextDate = d.Date
				}
			}
			for _, tx := range txs {
				if tx.AccountId == a.AccountId && (a.Category == models.ACCOUNT_CATEGORY_DEBT && tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN || a.Category == models.ACCOUNT_CATEGORY_RECEIVABLES && tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT) {
					paid = paid.Add(decimal.New(tx.Amount, -2))
				}
			}
			for _, m := range movements {
				if m.DebtAccountId != a.AccountId {
					continue
				}
				if tx, ok := byID[m.PrincipalTransactionId]; ok && tx.AccountId == a.AccountId && (m.Action == "repay" || m.Action == "collect") && (tx.Type == models.TRANSACTION_DB_TYPE_INCOME || tx.Type == models.TRANSACTION_DB_TYPE_EXPENSE) {
					paid = paid.Add(decimal.New(tx.Amount, -2))
				}
				if tx, ok := byID[m.InterestTransactionId]; ok {
					amount := decimal.New(tx.Amount, -2)
					if a.Category == models.ACCOUNT_CATEGORY_DEBT && tx.Type == models.TRANSACTION_DB_TYPE_INCOME || a.Category == models.ACCOUNT_CATEGORY_RECEIVABLES && tx.Type == models.TRANSACTION_DB_TYPE_EXPENSE {
						amount = amount.Neg()
					}
					interest = interest.Add(amount)
				}
			}
			row.PaidPrincipal = paid.StringFixed(2)
			row.Interest = interest.StringFixed(2)
			out = append(out, row)
		}
		return nil
	})
	return out, err
}
