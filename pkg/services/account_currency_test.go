package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAccountCurrencyOnlyChangesBeforeAnyFinancialHistory(t *testing.T) {
	f := newInvestmentDBFixture(t)
	account := &models.Account{AccountId: 9001, Uid: f.uid, Name: "空账户", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_VIRTUAL, Currency: "CNY"}
	_, err := f.engine.Insert(account)
	require.NoError(t, err)
	editable, err := Accounts.CurrencyEditable(nil, account)
	require.NoError(t, err)
	require.True(t, editable)
	changed := *account
	changed.Currency = "USD"
	require.NoError(t, Accounts.ModifyAccounts(nil, account, []*models.Account{&changed}, nil, nil, nil, true, time.UTC))
	var saved models.Account
	_, err = f.engine.ID(account.AccountId).Get(&saved)
	require.NoError(t, err)
	require.Equal(t, "USD", saved.Currency)
	require.Zero(t, saved.Balance)
	// Even deleted zero-balance history is denominated in the old currency.
	_, err = f.engine.Insert(&models.Transaction{TransactionId: 9002, Uid: f.uid, AccountId: account.AccountId, Deleted: true, Type: models.TRANSACTION_DB_TYPE_EXPENSE, Amount: 100, TransactionTime: 1})
	require.NoError(t, err)
	editable, err = Accounts.CurrencyEditable(nil, &saved)
	require.NoError(t, err)
	require.False(t, editable)
	changed = saved
	changed.Currency = "CNY"
	require.Error(t, Accounts.ModifyAccounts(nil, &saved, []*models.Account{&changed}, nil, nil, nil, true, time.UTC))
	_, err = f.engine.ID(account.AccountId).Get(&saved)
	require.NoError(t, err)
	require.Equal(t, "USD", saved.Currency)
	var funded models.Account
	_, err = f.engine.ID(f.bankID).Get(&funded)
	require.NoError(t, err)
	editable, err = Accounts.CurrencyEditable(nil, &funded)
	require.NoError(t, err)
	require.False(t, editable)
}

func TestAccountCurrencyPreservesIncomingSharedCreditLimit(t *testing.T) {
	f := newInvestmentDBFixture(t)
	main := &models.Account{AccountId: 9101, Uid: f.uid, Name: "共享额度主卡", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Currency: "CNY"}
	other := &models.Account{AccountId: 9102, Uid: f.uid, Name: "关联信用卡", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Currency: "CNY", Extend: &models.AccountExtend{AssetProfile: &models.AccountAssetProfile{SharedLimitAccount: "9101"}}}
	_, err := f.engine.Insert(main, other)
	require.NoError(t, err)
	allowed, err := Accounts.CurrencyEditable(nil, main)
	require.NoError(t, err)
	require.False(t, allowed)
	changed := *main
	changed.Currency = "USD"
	require.Error(t, Accounts.ModifyAccounts(nil, main, []*models.Account{&changed}, nil, nil, nil, true, time.UTC))
}
