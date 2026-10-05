package services

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type CreditStatement struct {
	Month          string   `json:"month"`
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	StatementDate  string   `json:"statementDate"`
	DueDate        string   `json:"dueDate"`
	Charges        string   `json:"charges"`
	Fees           string   `json:"fees"`
	FutureFees     string   `json:"futureFees"`
	Remaining      string   `json:"remaining"`
	Unbilled       bool     `json:"unbilled"`
	TransactionIds []string `json:"transactionIds"`
}
type CreditReport struct {
	AccountId    string            `json:"accountId"`
	AccountName  string            `json:"accountName"`
	Currency     string            `json:"currency"`
	Outstanding  string            `json:"outstanding"`
	Statements   []CreditStatement `json:"statements"`
	AnnualFee    string            `json:"annualFee"`
	AnnualDate   string            `json:"annualDate"`
	AnnualSpend  string            `json:"annualSpend"`
	AnnualCount  int               `json:"annualCount"`
	AnnualWaived bool              `json:"annualWaived"`
}

func creditDate(year int, month time.Month, day int, zone *time.Location) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, zone)
	last := first.AddDate(0, 1, -1).Day()
	if day <= 0 || day > last {
		day = last
	}
	return first.AddDate(0, 0, day-1)
}
func statementEnd(a models.Account, at time.Time) time.Time {
	day := 0
	if a.Extend != nil && a.Extend.CreditCardStatementDate != nil {
		day = *a.Extend.CreditCardStatementDate
	}
	end := creditDate(at.Year(), at.Month(), day, at.Location()).AddDate(0, 0, 1).Add(-time.Second)
	if a.AssetProfile() != nil && a.AssetProfile().StatementNextCycle {
		end = end.AddDate(0, 0, -1)
	}
	return end
}
func creditCycle(a models.Account, at time.Time) (string, time.Time, time.Time, time.Time) {
	month := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, at.Location())
	end := statementEnd(a, month)
	if at.After(end) {
		month = month.AddDate(0, 1, 0)
		end = statementEnd(a, month)
	}
	previous := statementEnd(a, month.AddDate(0, -1, 0))
	statement := end.Add(time.Second)
	if a.AssetProfile() != nil && a.AssetProfile().StatementNextCycle {
	} else {
		statement = statement.AddDate(0, 0, -1)
	}
	return month.Format("2006-01"), previous.Add(time.Second), end, statement
}
func creditDueDate(a models.Account, statement time.Time) string {
	p := a.AssetProfile()
	if p == nil {
		return ""
	}
	if p.RepaymentAfterDays > 0 {
		return statement.AddDate(0, 0, p.RepaymentAfterDays).Format("2006-01-02")
	}
	if p.RepaymentDay <= 0 {
		return ""
	}
	due := creditDate(statement.Year(), statement.Month(), p.RepaymentDay, statement.Location())
	if !due.After(statement) {
		due = creditDate(statement.Year(), statement.Month()+1, p.RepaymentDay, statement.Location())
	}
	return due.Format("2006-01-02")
}

func creditReportInSession(sess *xorm.Session, uid int64, a models.Account, now time.Time) (CreditReport, error) {
	report := CreditReport{AccountId: fmt.Sprint(a.AccountId), AccountName: a.Name, Currency: a.Currency, Outstanding: decimal.Max(decimal.Zero, decimal.New(a.Balance, -2).Neg()).StringFixed(2), Statements: []CreditStatement{}, AnnualFee: "0", AnnualSpend: "0"}
	var txs []models.Transaction
	if err := sess.Where("uid=? AND account_id=? AND deleted=?", uid, a.AccountId, false).Asc("transaction_time").Find(&txs); err != nil {
		return report, err
	}
	var plans []models.CreditInstallment
	if err := sess.Where("uid=? AND account_id=?", uid, a.AccountId).Find(&plans); err != nil {
		return report, err
	}
	txMap := map[string]models.Transaction{}
	feeIDs := map[string]bool{}
	for _, tx := range txs {
		txMap[fmt.Sprint(tx.TransactionId)] = tx
	}
	for _, p := range plans {
		if p.Data != nil {
			for _, payment := range p.Data.Payments {
				if payment.FeeTransactionId != "" {
					feeIDs[payment.FeeTransactionId] = true
				}
			}
		}
	}
	periods := map[string]*CreditStatement{}
	get := func(at time.Time) *CreditStatement {
		month, start, end, statement := creditCycle(a, at)
		if v := periods[month]; v != nil {
			return v
		}
		v := &CreditStatement{Month: month, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"), StatementDate: statement.Format("2006-01-02"), DueDate: creditDueDate(a, statement), Charges: "0", Fees: "0", FutureFees: "0", Remaining: "0", Unbilled: now.Before(end), TransactionIds: []string{}}
		periods[month] = v
		return v
	}
	add := func(field *string, amount decimal.Decimal) {
		old, _ := decimal.NewFromString(*field)
		*field = old.Add(amount).StringFixed(2)
	}
	for _, tx := range txs {
		at := time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(now.Location())
		row := get(at)
		row.TransactionIds = append(row.TransactionIds, fmt.Sprint(tx.TransactionId))
		if feeIDs[fmt.Sprint(tx.TransactionId)] {
			continue
		}
		amount := decimal.New(tx.Amount, -2)
		switch tx.Type {
		case models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
			add(&row.Charges, amount)
		case models.TRANSACTION_DB_TYPE_MODIFY_BALANCE:
			if tx.RelatedAccountAmount < 0 {
				add(&row.Charges, decimal.New(tx.RelatedAccountAmount, -2).Neg())
			}
		}
	}
	for _, plan := range plans {
		if plan.Data == nil || plan.Closed {
			continue
		}
		principal, _ := decimal.NewFromString(plan.Data.Principal)
		var origin *CreditStatement
		if plan.ExpenseId > 0 {
			if tx, ok := txMap[fmt.Sprint(plan.ExpenseId)]; ok {
				origin = get(time.Unix(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), 0).In(now.Location()))
			}
		}
		if plan.StatementMonth != "" {
			origin = periods[plan.StatementMonth]
		}
		if origin != nil {
			add(&origin.Charges, principal.Neg())
		}
		for _, payment := range plan.Data.Payments {
			at, err := time.ParseInLocation("2006-01-02", payment.Date, now.Location())
			if err != nil {
				return report, err
			}
			row := get(at)
			part, _ := decimal.NewFromString(payment.Principal)
			add(&row.Charges, part)
			if payment.Accrued {
				if tx, ok := txMap[payment.FeeTransactionId]; ok {
					add(&row.Fees, decimal.New(tx.Amount, -2))
				}
			} else {
				fee, _ := decimal.NewFromString(payment.Fee)
				add(&row.FutureFees, fee)
			}
		}
	}
	get(now)
	keys := make([]string, 0, len(periods))
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	remaining, _ := decimal.NewFromString(report.Outstanding)
	for _, key := range keys {
		row := periods[key]
		charges, _ := decimal.NewFromString(row.Charges)
		fees, _ := decimal.NewFromString(row.Fees)
		rowDebt := decimal.Max(decimal.Zero, charges.Add(fees))
		unpaid := decimal.Min(remaining, rowDebt)
		remaining = remaining.Sub(unpaid)
		row.Remaining = unpaid.StringFixed(2)
		report.Statements = append(report.Statements, *row)
	}
	if p := a.AssetProfile(); p != nil && p.AnnualFeeDate != "" {
		due, err := time.ParseInLocation("2006-01-02", p.AnnualFeeDate, now.Location())
		if err != nil {
			return report, err
		}
		for due.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())) {
			due = creditDate(due.Year()+1, due.Month(), due.Day(), now.Location())
		}
		report.AnnualDate = due.Format("2006-01-02")
		report.AnnualFee = p.AnnualFee
		from := due.AddDate(-1, 0, 0).Unix()
		spend := decimal.Zero
		for _, tx := range txs {
			at := utils.GetUnixTimeFromTransactionTime(tx.TransactionTime)
			if at >= from && at <= now.Unix() && tx.Type == models.TRANSACTION_DB_TYPE_EXPENSE && tx.Amount > 0 && !feeIDs[fmt.Sprint(tx.TransactionId)] {
				spend = spend.Add(decimal.New(tx.Amount, -2))
				report.AnnualCount++
			}
		}
		report.AnnualSpend = spend.StringFixed(2)
		amount, _ := decimal.NewFromString(p.AnnualWaiverAmount)
		report.AnnualWaived = p.AnnualWaiverCount > 0 && report.AnnualCount >= p.AnnualWaiverCount || amount.IsPositive() && spend.GreaterThanOrEqual(amount)
	}
	return report, nil
}
func (s *InvestmentService) CreditReports(c core.Context, uid int64, accountID, zoneName string) ([]CreditReport, error) {
	zone, err := time.LoadLocation(zoneName)
	if uid <= 0 || err != nil || zoneName == "" || len(zoneName) > 64 {
		return nil, investmentError("会计时区无效")
	}
	out := []CreditReport{}
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var accounts []models.Account
		q := sess.Where("uid=? AND category=? AND deleted=? AND system_role=?", uid, models.ACCOUNT_CATEGORY_CREDIT_CARD, false, "")
		if accountID != "" {
			id, e := strconv.ParseInt(accountID, 10, 64)
			if e != nil || id <= 0 {
				return investmentError("账户无效")
			}
			q = q.And("account_id=?", id)
		}
		if e := q.Find(&accounts); e != nil {
			return e
		}
		for _, a := range accounts {
			row, e := creditReportInSession(sess, uid, a, time.Now().In(zone))
			if e != nil {
				return e
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}
