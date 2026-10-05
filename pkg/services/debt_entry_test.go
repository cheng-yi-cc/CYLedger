package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestDebtRepaymentInterestDiscountAndExternalPrincipalAreAtomic(t *testing.T) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Account{AccountId: 8601, Uid: f.uid, Name: "测试借款", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_DEBT, Currency: "CNY"})
	require.NoError(t, err)
	require.NoError(t, Transactions.CreateTransaction(nil, &models.Transaction{Uid: f.uid, AccountId: 8601, Type: models.TRANSACTION_DB_TYPE_MODIFY_BALANCE, Amount: -100000, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at - 30)}, nil, nil))
	input := DebtMovementInput{DebtAccountId: "8601", CashAccountId: fmt.Sprint(f.bankID), Action: "repay", Principal: "200", Interest: "10", Time: f.at + 1, TimeZone: "Asia/Shanghai", RequestId: "debt-payment-001"}
	row, err := Investments.RecordDebtMovement(nil, f.uid, input)
	require.NoError(t, err)
	require.NotZero(t, row.InterestTransactionId)
	require.Equal(t, int64(1979000), f.balance())
	same, err := Investments.RecordDebtMovement(nil, f.uid, input)
	require.NoError(t, err)
	require.Equal(t, row.Id, same.Id)
	require.Equal(t, int64(1979000), f.balance())
	input.Principal = "100"
	input.Interest = "-5"
	input.RequestId = "debt-payment-002"
	_, err = Investments.RecordDebtMovement(nil, f.uid, input)
	require.NoError(t, err)
	require.Equal(t, int64(1969500), f.balance())
	input.Principal = "800"
	input.RequestId = "debt-payment-003"
	_, err = Investments.RecordDebtMovement(nil, f.uid, input)
	require.Error(t, err)
	require.Equal(t, int64(1969500), f.balance())
	input.Principal = "100"
	input.Interest = "0"
	input.CashAccountId = ""
	_, err = Investments.RecordDebtMovement(nil, f.uid, input)
	require.NoError(t, err)
	require.Equal(t, int64(1969500), f.balance())
	reports, err := Investments.DebtReports(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "400.00", reports[0].PaidPrincipal)
	require.Equal(t, "5.00", reports[0].Interest)
	deletion := AccountDeletionInput{ID: "8601", Kind: "cash", DeleteRelated: true}
	preview, err := Investments.DeleteAssetAccount(nil, f.uid, deletion, true)
	require.NoError(t, err)
	deletion.Token = preview.Token
	_, err = Investments.DeleteAssetAccount(nil, f.uid, deletion, false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
}

// Debt entry uses the same atomic double-entry transfers as the cash ledger.
func TestDebtEntryInstallmentsKeepPrincipalTagsAndIncomeSeparate(t *testing.T) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(
		&models.Account{AccountId: 8101, Uid: f.uid, Name: "borrowed", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_DEBT, Currency: "CNY"},
		&models.Account{AccountId: 8102, Uid: f.uid, Name: "lent", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_RECEIVABLES, Currency: "CNY"},
		&models.TransactionTag{TagId: 8201, Uid: f.uid, Name: "test tag"})
	require.NoError(t, err)
	require.NoError(t, TransactionCategories.EnsurePersonalCategories(nil, f.uid))
	var category models.TransactionCategory
	_, err = f.engine.Where("uid=? AND type=? AND parent_category_id>0", f.uid, models.CATEGORY_TYPE_TRANSFER).Get(&category)
	require.NoError(t, err)
	initialCash := f.balance()
	for index, movement := range []struct{ source, destination, amount int64 }{
		{8101, f.bankID, 10000}, {f.bankID, 8101, 3000}, {f.bankID, 8101, 2000},
		{f.bankID, 8102, 8000}, {8102, f.bankID, 2500},
	} {
		transaction := &models.Transaction{Uid: f.uid, AccountId: movement.source, RelatedAccountId: movement.destination,
			Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, CategoryId: category.CategoryId,
			Amount: movement.amount, RelatedAccountAmount: movement.amount, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + int64(index+1))}
		require.NoError(t, Transactions.CreateTransaction(nil, transaction, []int64{8201}, nil))
		tags, err := TransactionTags.GetAllTagIdsOfTransactions(nil, f.uid, []int64{transaction.TransactionId})
		require.NoError(t, err)
		require.Contains(t, tags[transaction.TransactionId], int64(8201))
	}
	var borrowed, lent models.Account
	_, err = f.engine.ID(8101).Get(&borrowed)
	require.NoError(t, err)
	_, err = f.engine.ID(8102).Get(&lent)
	require.NoError(t, err)
	require.Equal(t, int64(-5000), borrowed.Balance)
	require.Equal(t, int64(5500), lent.Balance)
	require.Equal(t, initialCash-500, f.balance())
	income, expense, err := Transactions.GetAccountsTotalIncomeAndExpense(nil, f.uid, f.at, f.at+60, nil, nil, time.UTC, false)
	require.NoError(t, err)
	for _, total := range income {
		require.Zero(t, total.Sign())
	}
	for _, total := range expense {
		require.Zero(t, total.Sign())
	}
}
