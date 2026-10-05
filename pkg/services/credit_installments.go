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

type CreditInstallmentInput struct {
	Id             string                            `json:"id"`
	Version        string                            `json:"version"`
	AccountId      string                            `json:"accountId"`
	ExpenseId      string                            `json:"expenseId"`
	StatementMonth string                            `json:"statementMonth"`
	Principal      string                            `json:"principal"`
	TotalFee       string                            `json:"totalFee"`
	Periods        int                               `json:"periods"`
	FirstDate      string                            `json:"firstDate"`
	Method         string                            `json:"method"`
	Remainder      string                            `json:"remainder"`
	BookId         string                            `json:"bookId"`
	TimeZone       string                            `json:"timeZone"`
	Note           string                            `json:"note"`
	RequestId      string                            `json:"requestId"`
	Payments       []models.CreditInstallmentPayment `json:"payments"`
}

func splitCreditAmount(amount decimal.Decimal, n int, where string) []decimal.Decimal {
	cents := amount.Shift(2).IntPart()
	part, rest := cents/int64(n), cents%int64(n)
	values := make([]decimal.Decimal, n)
	for i := range values {
		values[i] = decimal.New(part, -2)
	}
	if where == "first" {
		values[0] = values[0].Add(decimal.New(rest, -2))
	} else if where == "last" {
		values[n-1] = values[n-1].Add(decimal.New(rest, -2))
	} else {
		start, end := 0, n
		if n > 1 {
			if where == "except_first" {
				start = 1
			} else {
				end = n - 1
			}
		}
		for i := start; rest > 0; i++ {
			index := start + (i-start)%(end-start)
			values[index] = values[index].Add(decimal.New(1, -2))
			rest--
		}
	}
	return values
}
func makeCreditSchedule(input CreditInstallmentInput, now time.Time) (*models.CreditInstallmentData, error) {
	zone, e := time.LoadLocation(input.TimeZone)
	if e != nil || input.TimeZone == "" || len(input.TimeZone) > 64 || input.Periods < 1 || input.Periods > 500 || !validCalendarDate(input.FirstDate) || len(input.Principal) > 20 || !assetAmountPattern.MatchString(input.Principal) || len(input.TotalFee) > 20 || !assetAmountPattern.MatchString(input.TotalFee) || len(input.BookId) > 64 || utf8.RuneCountInString(input.Note) > 200 {
		return nil, investmentError("请检查分期本金、服务费、期数和日期")
	}
	if input.Method != "monthly" && input.Method != "first_fee" && input.Method != "immediate_fee" && input.Method != "balloon" {
		return nil, investmentError("分期方式无效")
	}
	if input.Remainder != "first" && input.Remainder != "last" && input.Remainder != "except_first" && input.Remainder != "except_last" {
		return nil, investmentError("尾差处理方式无效")
	}
	principal, _ := decimal.NewFromString(input.Principal)
	fee, _ := decimal.NewFromString(input.TotalFee)
	if !principal.IsPositive() || fee.IsNegative() || principal.Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) || fee.Shift(2).GreaterThan(decimal.NewFromInt(models.MaximumTransactionAmount)) {
		return nil, investmentError("本金须大于零，服务费不能为负数")
	}
	first, _ := time.ParseInLocation("2006-01-02", input.FirstDate, zone)
	data := &models.CreditInstallmentData{Principal: principal.StringFixed(2), TotalFee: fee.StringFixed(2), Periods: input.Periods, FirstDate: input.FirstDate, Method: input.Method, Remainder: input.Remainder, BookId: input.BookId, TimeZone: input.TimeZone, Note: input.Note, Payments: []models.CreditInstallmentPayment{}}
	parts, fees := splitCreditAmount(principal, input.Periods, input.Remainder), splitCreditAmount(fee, input.Periods, input.Remainder)
	for i := 0; i < input.Periods; i++ {
		at := creditDate(first.Year(), first.Month()+time.Month(i), first.Day(), zone)
		if at.Year() > 9999 {
			return nil, investmentError("分期期限过长")
		}
		p, f := parts[i], fees[i]
		if input.Method == "balloon" {
			p = decimal.Zero
			if i == input.Periods-1 {
				p = principal
			}
		}
		if input.Method == "first_fee" || input.Method == "immediate_fee" {
			f = decimal.Zero
			if input.Method == "first_fee" && i == 0 {
				f = fee
			}
		}
		data.Payments = append(data.Payments, models.CreditInstallmentPayment{Date: at.Format("2006-01-02"), Principal: p.StringFixed(2), Fee: f.StringFixed(2)})
	}
	if input.Method == "immediate_fee" && fee.IsPositive() {
		data.Payments = append([]models.CreditInstallmentPayment{{Date: now.In(zone).Format("2006-01-02"), Principal: "0.00", Fee: fee.StringFixed(2)}}, data.Payments...)
	}
	if len(input.Payments) > 0 {
		if len(input.Payments) != len(data.Payments) {
			return nil, investmentError("每期金额数量与期数不一致")
		}
		ptotal, ftotal := decimal.Zero, decimal.Zero
		for i, p := range input.Payments {
			if !validCalendarDate(p.Date) || len(p.Principal) > 20 || len(p.Fee) > 20 || !calendarAmountPattern.MatchString(p.Principal) || !calendarAmountPattern.MatchString(p.Fee) {
				return nil, investmentError("请检查每期日期和金额")
			}
			if i > 0 && p.Date < input.Payments[i-1].Date {
				return nil, investmentError("还款日期须按先后顺序排列")
			}
			principal, _ := decimal.NewFromString(p.Principal)
			fee, _ := decimal.NewFromString(p.Fee)
			ptotal = ptotal.Add(principal)
			ftotal = ftotal.Add(fee)
			data.Payments[i] = models.CreditInstallmentPayment{Date: p.Date, Principal: principal.StringFixed(2), Fee: fee.StringFixed(2)}
		}
		if !ptotal.Equal(principal) || !ftotal.Equal(fee) {
			return nil, investmentError("每期金额合计必须等于本金和总服务费")
		}
	}
	return data, nil
}

func (s *InvestmentService) PreviewCreditInstallment(input CreditInstallmentInput) (*models.CreditInstallmentData, error) {
	return makeCreditSchedule(input, time.Now())
}
func (s *InvestmentService) CreditInstallments(c core.Context, uid int64) ([]models.CreditInstallment, error) {
	rows := []models.CreditInstallment{}
	if uid <= 0 {
		return nil, investmentError("用户无效")
	}
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Desc("id").Find(&rows)
	return rows, err
}
func (s *InvestmentService) SaveCreditInstallment(c core.Context, uid int64, input CreditInstallmentInput) (*models.CreditInstallment, error) {
	data, err := makeCreditSchedule(input, time.Now())
	if err != nil {
		return nil, err
	}
	id, e1 := strconv.ParseInt(input.AccountId, 10, 64)
	expenseID := int64(0)
	var e2 error
	if input.ExpenseId != "" && input.ExpenseId != "0" {
		expenseID, e2 = strconv.ParseInt(input.ExpenseId, 10, 64)
	}
	if uid <= 0 || e1 != nil || e2 != nil || id <= 0 || expenseID < 0 || len(input.Id) > 64 || len(input.RequestId) < 8 || len(input.RequestId) > 48 || len(input.Version) > 20 || (expenseID == 0 && (len(input.StatementMonth) != 7 || !validCalendarDate(input.StatementMonth+"-01"))) || (expenseID > 0 && input.StatementMonth != "") {
		return nil, investmentError("请选择原消费或待分期账单")
	}
	raw, _ := json.Marshal(input)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	row := &models.CreditInstallment{Id: input.Id, Uid: uid, AccountId: id, ExpenseId: expenseID, StatementMonth: input.StatementMonth, Data: data}
	defer s.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if row.Id == "" {
			row.Id = fmt.Sprintf("%d:%s", uid, input.RequestId)
			var existing models.CreditInstallment
			has, e := sess.ID(row.Id).Get(&existing)
			if e != nil {
				return e
			}
			if has {
				if existing.Uid != uid || existing.RequestDigest != digest {
					return investmentError("请求已使用，请刷新后重试")
				}
				*row = existing
				return nil
			}
			row.RequestDigest = digest
			row.Version = 1
		} else {
			var old models.CreditInstallment
			has, e := sess.ID(row.Id).Where("uid=? AND closed=?", uid, false).Get(&old)
			if e != nil {
				return e
			}
			if !has || fmt.Sprint(old.Version) != input.Version {
				return investmentError("分期已变化，请刷新后重试")
			}
			if old.AccountId != id || old.ExpenseId != expenseID || old.StatementMonth != input.StatementMonth {
				return investmentError("已有分期不能更换原始账单")
			}
			if old.Data != nil {
				for i, p := range old.Data.Payments {
					if !p.Accrued {
						continue
					}
					if i >= len(data.Payments) || p.Date != data.Payments[i].Date || p.Principal != data.Payments[i].Principal || p.Fee != data.Payments[i].Fee {
						return investmentError("已到期的分期不能修改，请编辑实际费用账单")
					}
					data.Payments[i] = p
				}
			}
			row.RequestDigest = old.RequestDigest
			row.Version = old.Version + 1
		}
		var account models.Account
		has, e := sess.ID(id).Where("uid=? AND deleted=? AND hidden=? AND system_role=?", uid, false, false, "").Get(&account)
		if e != nil {
			return e
		}
		if !has || (account.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && account.Category != models.ACCOUNT_CATEGORY_DEBT) || account.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
			return investmentError("请选择信贷或借入账户")
		}
		var txs []models.Transaction
		if e = sess.Where("uid=? AND account_id=? AND deleted=?", uid, id, false).Find(&txs); e != nil {
			return e
		}
		var others []models.CreditInstallment
		if e = sess.Where("uid=? AND account_id=? AND closed=? AND id<>?", uid, id, false, row.Id).Find(&others); e != nil {
			return e
		}
		available := decimal.Zero
		originDate := ""
		zone, _ := time.LoadLocation(data.TimeZone)
		for _, tx := range txs {
			at := time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(zone)
			month, _, _, _ := creditCycle(account, at)
			selected := expenseID > 0 && tx.TransactionId == expenseID || expenseID == 0 && month == input.StatementMonth
			if !selected {
				continue
			}
			amount := decimal.New(tx.Amount, -2)
			if tx.Type == models.TRANSACTION_DB_TYPE_EXPENSE || tx.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT {
				available = available.Add(amount)
				if originDate == "" || at.Format("2006-01-02") < originDate {
					originDate = at.Format("2006-01-02")
				}
				if input.BookId == "" {
					data.BookId = tx.BookId
				}
			}
			if expenseID == 0 && tx.Type == models.TRANSACTION_DB_TYPE_MODIFY_BALANCE && tx.RelatedAccountAmount < 0 {
				available = available.Sub(decimal.New(tx.RelatedAccountAmount, -2))
				if originDate == "" {
					originDate = at.Format("2006-01-02")
				}
			}
		}
		for _, plan := range others {
			if plan.Data == nil {
				continue
			}
			same := plan.ExpenseId == expenseID && plan.StatementMonth == input.StatementMonth
			// A bill-level plan and its expense-level plans share one principal pool.
			if !same {
				for _, tx := range txs {
					if tx.TransactionId != plan.ExpenseId && tx.TransactionId != expenseID {
						continue
					}
					month, _, _, _ := creditCycle(account, time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(zone))
					if expenseID == 0 && tx.TransactionId == plan.ExpenseId && month == input.StatementMonth || plan.ExpenseId == 0 && tx.TransactionId == expenseID && month == plan.StatementMonth {
						same = true
					}
				}
			}
			if same {
				p, _ := decimal.NewFromString(plan.Data.Principal)
				available = available.Sub(p)
			}
		}
		principal, _ := decimal.NewFromString(data.Principal)
		if originDate == "" || principal.GreaterThan(available) {
			return investmentError("分期本金不能超过原账单尚未分期的金额")
		}
		if input.Id == "" {
			report, e := creditReportInSession(sess, uid, account, time.Now().In(zone))
			if e != nil {
				return e
			}
			remaining, _ := decimal.NewFromString(report.Outstanding)
			if expenseID == 0 {
				remaining = decimal.Zero
				for _, statement := range report.Statements {
					if statement.Month == input.StatementMonth {
						remaining, _ = decimal.NewFromString(statement.Remaining)
					}
				}
			}
			if principal.GreaterThan(remaining) {
				return investmentError("分期本金不能超过当前待还金额")
			}
		}
		for _, p := range data.Payments {
			if p.Date < originDate {
				return investmentError("分期日期不能早于原账单")
			}
		}
		book, e := Books.ResolveInSession(sess, uid, data.BookId, false)
		if e != nil {
			return e
		}
		data.BookId = book
		if input.Id == "" {
			_, e = sess.Insert(row)
		} else {
			_, e = sess.ID(row.Id).Where("uid=?", uid).AllCols().Update(row)
		}
		return e
	})
	return row, err
}

func (s *InvestmentService) CloseCreditInstallment(c core.Context, uid int64, id, version string) (bool, error) {
	if uid <= 0 || id == "" || len(id) > 64 {
		return false, investmentError("分期无效")
	}
	defer s.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var plan models.CreditInstallment
		has, e := sess.ID(id).Where("uid=?", uid).Get(&plan)
		if e != nil {
			return e
		}
		if !has || fmt.Sprint(plan.Version) != version {
			return investmentError("分期已变化，请刷新后重试")
		}
		if plan.Closed {
			return nil
		}
		plan.Closed = true
		plan.Version++
		_, e = sess.ID(id).Where("uid=?", uid).Cols("closed", "version").Update(&plan)
		return e
	})
	return err == nil, err
}

func (s *InvestmentService) SyncCreditInstallments(c core.Context, uid int64, now time.Time) (int, error) {
	defer s.lock(uid)()
	created := 0
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var plans []models.CreditInstallment
		if e := sess.Where("uid=? AND closed=?", uid, false).Find(&plans); e != nil {
			return e
		}
		for _, plan := range plans {
			if plan.Data == nil {
				continue
			}
			zone, e := time.LoadLocation(plan.Data.TimeZone)
			if e != nil {
				return e
			}
			var a models.Account
			has, e := sess.ID(plan.AccountId).Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Get(&a)
			if e != nil {
				return e
			}
			if !has {
				continue
			}
			changed := false
			for i, p := range plan.Data.Payments {
				if p.Accrued || p.Date > now.In(zone).Format("2006-01-02") {
					continue
				}
				fee, e := decimal.NewFromString(p.Fee)
				if e != nil {
					return e
				}
				if fee.IsPositive() {
					at, _ := time.ParseInLocation("2006-01-02", p.Date, zone)
					var first models.Transaction
					has, e := sess.Where("uid=? AND account_id=? AND deleted=?", uid, a.AccountId, false).Asc("transaction_time").Get(&first)
					if e != nil {
						return e
					}
					if has && at.Unix() <= utils.GetUnixTimeFromTransactionTime(first.TransactionTime) {
						at = time.Unix(utils.GetUnixTimeFromTransactionTime(first.TransactionTime)+1, 0).In(zone)
					}
					if at.After(now) {
						continue
					}
					category, e := assetIncomeCategory(sess, uid, plan.Data.BookId, models.CATEGORY_TYPE_EXPENSE, "分期服务费")
					if e != nil {
						return e
					}
					_, offset := at.Zone()
					tx := &models.Transaction{Uid: uid, AccountId: a.AccountId, BookId: plan.Data.BookId, Type: models.TRANSACTION_DB_TYPE_EXPENSE, CategoryId: category, Amount: fee.Shift(2).IntPart(), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Unix()), TimezoneUtcOffset: int16(offset / 60), Comment: fmt.Sprintf("分期服务费：%s（第%d期）", plan.Data.Note, i+1)}
					if e = Transactions.createTransactionInSession(c, sess, tx, nil, nil); e != nil {
						return e
					}
					p.FeeTransactionId = fmt.Sprint(tx.TransactionId)
					created++
				}
				p.Accrued = true
				plan.Data.Payments[i] = p
				changed = true
			}
			if changed {
				plan.Version++
				if _, e = sess.ID(plan.Id).Where("uid=?", uid).Cols("data", "version").Update(&plan); e != nil {
					return e
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}

func guardCreditInstallmentTransaction(sess *xorm.Session, tx, old *models.Transaction, deleting bool) error {
	var plans []models.CreditInstallment
	if e := sess.Where("uid=? AND closed=?", old.Uid, false).Find(&plans); e != nil {
		return e
	}
	total := decimal.Zero
	for _, plan := range plans {
		if plan.Data == nil {
			continue
		}
		if plan.StatementMonth != "" && plan.AccountId == old.AccountId && (old.Type == models.TRANSACTION_DB_TYPE_EXPENSE || old.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT || old.Type == models.TRANSACTION_DB_TYPE_MODIFY_BALANCE) {
			var a models.Account
			if _, e := sess.ID(old.AccountId).Where("uid=?", old.Uid).Get(&a); e != nil {
				return e
			}
			zone, e := time.LoadLocation(plan.Data.TimeZone)
			if e != nil {
				return e
			}
			month, _, _, _ := creditCycle(a, time.Unix(utils.GetUnixTimeFromTransactionTime(old.TransactionTime), 0).In(zone))
			if month == plan.StatementMonth && (deleting || tx.AccountId != old.AccountId || tx.Type != old.Type || tx.Amount != old.Amount || tx.TransactionTime != old.TransactionTime) {
				return investmentError("本期账单已有分期，请先结束分期再修改金额或日期")
			}
		}
		if plan.ExpenseId == old.TransactionId {
			if deleting {
				return investmentError("这笔账单已有分期，请先结束分期")
			}
			if tx.AccountId != old.AccountId || tx.Type != old.Type {
				return investmentError("已有分期的账单不能更换账户或收支类型")
			}
			p, _ := decimal.NewFromString(plan.Data.Principal)
			total = total.Add(p)
			zone, _ := time.LoadLocation(plan.Data.TimeZone)
			if zone != nil {
				date := time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(zone).Format("2006-01-02")
				for _, payment := range plan.Data.Payments {
					if payment.Date < date {
						return investmentError("原账单日期不能晚于分期日期")
					}
				}
			}
		}
		for _, payment := range plan.Data.Payments {
			if payment.FeeTransactionId != fmt.Sprint(old.TransactionId) || deleting {
				continue
			}
			if tx.Type != models.TRANSACTION_DB_TYPE_EXPENSE || tx.AccountId != old.AccountId || tx.Amount < 0 {
				return investmentError("分期服务费须保留在原信贷账户中")
			}
		}
	}
	if !deleting && total.GreaterThan(decimal.New(tx.Amount, -2)) {
		return investmentError("账单金额不能小于已分期的本金")
	}
	return nil
}
