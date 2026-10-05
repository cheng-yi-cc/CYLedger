package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCreditScheduleMonthEndsAndExactRemainders(t *testing.T) {
	for _, method := range []string{"monthly", "first_fee", "immediate_fee", "balloon"} {
		for _, remainder := range []string{"first", "last", "except_first", "except_last"} {
			d, e := makeCreditSchedule(CreditInstallmentInput{Principal: "100.01", TotalFee: "10.01", Periods: 3, FirstDate: "2026-01-31", Method: method, Remainder: remainder, TimeZone: "Asia/Shanghai"}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			require.NoError(t, e)
			p, f := decimal.Zero, decimal.Zero
			for _, payment := range d.Payments {
				a, _ := decimal.NewFromString(payment.Principal)
				b, _ := decimal.NewFromString(payment.Fee)
				p = p.Add(a)
				f = f.Add(b)
			}
			require.Equal(t, "100.01", p.String())
			require.Equal(t, "10.01", f.String())
			require.Equal(t, "2026-02-28", d.Payments[len(d.Payments)-2].Date)
			require.Equal(t, "2026-03-31", d.Payments[len(d.Payments)-1].Date)
		}
	}
	zone, _ := time.LoadLocation("Asia/Shanghai")
	day := 31
	a := models.Account{Extend: &models.AccountExtend{CreditCardStatementDate: &day, AssetProfile: &models.AccountAssetProfile{RepaymentAfterDays: 20}}}
	month, _, _, statement := creditCycle(a, time.Date(2026, 2, 28, 10, 0, 0, 0, zone))
	require.Equal(t, "2026-02", month)
	require.Equal(t, "2026-03-20", creditDueDate(a, statement))
	a.Extend.AssetProfile.StatementNextCycle = true
	month, _, _, _ = creditCycle(a, time.Date(2026, 2, 28, 0, 0, 0, 0, zone))
	require.Equal(t, "2026-03", month)
}

func TestCreditInstallmentPrincipalFeesIdempotenceAndRepayment(t *testing.T) {
	f := newInvestmentDBFixture(t)
	zone, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(zone)
	_, e := f.engine.Insert(&models.Account{AccountId: 8501, Uid: f.uid, Name: "测试信贷", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Currency: "CNY"})
	require.NoError(t, e)
	require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
	var category models.TransactionCategory
	_, e = f.engine.Where("uid=? AND type=? AND parent_category_id>0", f.uid, models.CATEGORY_TYPE_EXPENSE).Get(&category)
	require.NoError(t, e)
	expense := &models.Transaction{Uid: f.uid, AccountId: 8501, Type: models.TRANSACTION_DB_TYPE_EXPENSE, Amount: 10000, CategoryId: category.CategoryId, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at)}
	require.NoError(t, Transactions.CreateTransaction(nil, expense, nil, nil))
	input := CreditInstallmentInput{AccountId: "8501", ExpenseId: fmt.Sprint(expense.TransactionId), Principal: "100", TotalFee: "6", Periods: 3, FirstDate: now.Format("2006-01-02"), Method: "monthly", Remainder: "first", TimeZone: "Asia/Shanghai", RequestId: "credit-plan-test-01"}
	plan, e := Investments.SaveCreditInstallment(nil, f.uid, input)
	require.NoError(t, e)
	same, e := Investments.SaveCreditInstallment(nil, f.uid, input)
	require.NoError(t, e)
	require.Equal(t, plan.Id, same.Id)
	n, e := Investments.SyncCreditInstallments(nil, f.uid, now)
	require.NoError(t, e)
	require.Equal(t, 1, n)
	n, e = Investments.SyncCreditInstallments(nil, f.uid, now)
	require.NoError(t, e)
	require.Zero(t, n)
	var a models.Account
	_, e = f.engine.ID(8501).Get(&a)
	require.NoError(t, e)
	require.Equal(t, int64(-10200), a.Balance)
	report, e := Investments.CreditReports(nil, f.uid, "8501", "Asia/Shanghai")
	require.NoError(t, e)
	require.Equal(t, "102.00", report[0].Outstanding)
	remaining := decimal.Zero
	for _, row := range report[0].Statements {
		v, _ := decimal.NewFromString(row.Remaining)
		remaining = remaining.Add(v)
	}
	require.Equal(t, "102", remaining.String())
	require.Error(t, Transactions.DeleteTransaction(nil, f.uid, expense.TransactionId))
	modified := *expense
	modified.Amount = 5000
	require.Error(t, Transactions.ModifyTransaction(nil, &modified, false, 0, nil, nil, nil, nil))
	var transferCategory models.TransactionCategory
	_, e = f.engine.Where("uid=? AND type=? AND parent_category_id>0", f.uid, models.CATEGORY_TYPE_TRANSFER).Get(&transferCategory)
	require.NoError(t, e)
	repayment := &models.Transaction{Uid: f.uid, AccountId: f.bankID, RelatedAccountId: 8501, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, CategoryId: transferCategory.CategoryId, Amount: 3500, RelatedAccountAmount: 3500, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(now.Unix())}
	require.NoError(t, Transactions.CreateTransaction(nil, repayment, nil, nil))
	report, e = Investments.CreditReports(nil, f.uid, "8501", "Asia/Shanghai")
	require.NoError(t, e)
	require.Equal(t, "67.00", report[0].Outstanding)
	rows, e := Investments.CreditInstallments(nil, f.uid)
	require.NoError(t, e)
	_, e = Investments.CloseCreditInstallment(nil, f.uid, plan.Id, fmt.Sprint(rows[0].Version))
	require.NoError(t, e)
	n, e = Investments.SyncCreditInstallments(nil, f.uid, now.AddDate(1, 0, 0))
	require.NoError(t, e)
	require.Zero(t, n)
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, expense.TransactionId))
}

func TestAssetAdjustmentConcurrencyIdempotenceAndStatistics(t *testing.T) {
	f := newInvestmentDBFixture(t)
	input := AssetAdjustmentInput{AccountId: fmt.Sprint(f.bankID), Balance: "21000.25", ExpectedBalance: "20000", TimeZone: "Asia/Shanghai", RequestId: "adjust-test-0001"}
	row, e := Investments.AdjustAssetBalance(nil, f.uid, input)
	require.NoError(t, e)
	require.Equal(t, int64(2100025), f.balance())
	same, e := Investments.AdjustAssetBalance(nil, f.uid, input)
	require.NoError(t, e)
	require.Equal(t, row.Id, same.Id)
	require.Equal(t, int64(2100025), f.balance())
	input.RequestId = "adjust-test-0002"
	_, e = Investments.AdjustAssetBalance(nil, f.uid, input)
	require.Error(t, e)
	var tx models.Transaction
	_, e = f.engine.ID(row.TransactionId).Get(&tx)
	require.NoError(t, e)
	require.True(t, tx.ExcludeFromStatistics)
	input.ExpectedBalance = "21000.25"
	input.Balance = "20000"
	input.CountInStatistics = true
	row, e = Investments.AdjustAssetBalance(nil, f.uid, input)
	require.NoError(t, e)
	tx = models.Transaction{}
	_, e = f.engine.ID(row.TransactionId).Get(&tx)
	require.NoError(t, e)
	require.False(t, tx.ExcludeFromStatistics)
	require.Equal(t, int64(2000000), f.balance())
}
