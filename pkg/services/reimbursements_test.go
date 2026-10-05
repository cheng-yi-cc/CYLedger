package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"xorm.io/xorm"
)

func reimbursementFixture(t *testing.T) (*investmentDBFixture, *models.Transaction) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Account{AccountId: 8401, Uid: f.uid, Name: "测试报销", Category: models.ACCOUNT_CATEGORY_RECEIVABLES, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Currency: "CNY", Extend: &models.AccountExtend{AssetProfile: &models.AccountAssetProfile{Kind: "reimbursement"}}})
	require.NoError(t, err)
	require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
	var category models.TransactionCategory
	_, err = f.engine.Where("uid=? AND type=? AND parent_category_id>0", f.uid, models.CATEGORY_TYPE_EXPENSE).Get(&category)
	require.NoError(t, err)
	tx := &models.Transaction{Uid: f.uid, AccountId: f.bankID, Type: models.TRANSACTION_DB_TYPE_EXPENSE, Amount: 10000, CategoryId: category.CategoryId, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 1), ReimbursementAccountId: 8401}
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	return f, tx
}
func reimbursementBalance(t *testing.T, f *investmentDBFixture) int64 {
	var a models.Account
	_, err := f.engine.ID(8401).Get(&a)
	require.NoError(t, err)
	return a.Balance
}
func TestReimbursementPartialReceiptIdempotenceHistoricalBalanceAndDeletion(t *testing.T) {
	f, tx := reimbursementFixture(t)
	require.Equal(t, int64(1990000), f.balance())
	require.Equal(t, int64(10000), reimbursementBalance(t, f))
	req := ReimbursementReceiveRequest{ExpenseId: fmt.Sprint(tx.TransactionId), AccountId: fmt.Sprint(f.bankID), Amount: "40.00", Time: f.at + 20, TimeZone: "Asia/Shanghai", RequestId: "receipt-request-1"}
	receipt, err := Investments.ReceiveReimbursement(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, int64(1994000), f.balance())
	require.Equal(t, int64(6000), reimbursementBalance(t, f))
	same, err := Investments.ReceiveReimbursement(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, receipt.Id, same.Id)
	require.Equal(t, int64(1994000), f.balance())
	req.Amount = "70.00"
	req.RequestId = "receipt-request-2"
	_, err = Investments.ReceiveReimbursement(nil, f.uid, req)
	require.Error(t, err)
	_, err = Investments.ReceiveReimbursement(nil, f.uid+1, req)
	require.Error(t, err)
	_, err = f.engine.Transaction(func(sess *xorm.Session) (any, error) {
		before, e := reimbursementBalancesAt(sess, f.uid, f.at+10)
		require.NoError(t, e)
		require.Equal(t, int64(10000), before[8401])
		after, e := reimbursementBalancesAt(sess, f.uid, f.at+30)
		require.NoError(t, e)
		require.Equal(t, int64(6000), after[8401])
		return nil, nil
	})
	require.NoError(t, err)
	income, expense, err := Transactions.GetAccountsTotalIncomeAndExpense(nil, f.uid, f.at, f.at+60, nil, nil, time.UTC, false)
	require.NoError(t, err)
	require.Empty(t, income)
	require.Empty(t, expense)
	require.Error(t, Transactions.DeleteTransaction(nil, f.uid, tx.TransactionId))
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, receipt.IncomeId))
	require.Equal(t, int64(10000), reimbursementBalance(t, f))
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, tx.TransactionId))
	require.Zero(t, reimbursementBalance(t, f))
	require.Equal(t, int64(2000000), f.balance())
}

func TestReimbursementEndReopenAndCascadeAccountDeletion(t *testing.T) {
	f, tx := reimbursementFixture(t)
	_, err := Investments.SetReimbursementState(nil, f.uid, tx.TransactionId, "end")
	require.NoError(t, err)
	require.Zero(t, reimbursementBalance(t, f))
	_, err = Investments.SetReimbursementState(nil, f.uid, tx.TransactionId, "reopen")
	require.NoError(t, err)
	require.Equal(t, int64(10000), reimbursementBalance(t, f))
	_, err = Investments.ReceiveReimbursement(nil, f.uid, ReimbursementReceiveRequest{ExpenseId: fmt.Sprint(tx.TransactionId), AccountId: fmt.Sprint(f.bankID), Amount: "40", Time: f.at + 10, TimeZone: "Asia/Shanghai", RequestId: "receipt-test-cascade"})
	require.NoError(t, err)
	require.Error(t, Accounts.DeleteAccount(nil, f.uid, 8401))
	input := AccountDeletionInput{ID: "8401", Kind: "cash", DeleteRelated: true}
	preview, err := Investments.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	require.Equal(t, 2, preview.TransactionCount)
	input.Token = preview.Token
	_, err = Investments.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	claims, err := Investments.ReimbursementClaims(nil, f.uid)
	require.NoError(t, err)
	require.Empty(t, claims)
}

func TestMonetaryPrincipalExcludesOnlyActiveFixedDeposits(t *testing.T) {
	f, b := monetaryFixture(t)
	for _, d := range []models.FixedDeposit{
		{Id: "active", Principal: "5000", StartDate: "2026-09-01", MaturityDate: "2026-10-01"},
		{Id: "future", Principal: "3000", StartDate: "2026-10-01", MaturityDate: "2026-11-01"},
		{Id: "matured", Principal: "2000", StartDate: "2026-08-01", MaturityDate: "2026-09-29"},
		{Id: "closed", Principal: "1000", StartDate: "2026-09-01", MaturityDate: "2026-10-01", Closed: true, ClosedDate: "2026-09-20"},
		{Id: "closed-later", Principal: "1000", StartDate: "2026-09-01", MaturityDate: "2026-10-01", Closed: true, ClosedDate: "2026-09-30"},
	} {
		d.Uid = f.uid
		d.AccountId = fmt.Sprint(f.bankID)
		_, err := f.engine.Insert(&d)
		require.NoError(t, err)
	}
	n, err := MonetaryIncome.settleDay(nil, f.uid, b.Id, b.NextDate, "1")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var day models.MonetaryIncomeDay
	_, err = f.engine.Where("uid=?", f.uid).Get(&day)
	require.NoError(t, err)
	require.Equal(t, "14000.00", day.Principal)
	require.Equal(t, "1.40", day.Amount)
}
