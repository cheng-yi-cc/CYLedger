package services

import (
	"fmt"
	"testing"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
)

func deletionPreview(t *testing.T, f *investmentDBFixture, input AccountDeletionInput) *AccountDeletionPreview {
	t.Helper()
	preview, err := f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	require.NotEmpty(t, preview.Token)
	return preview
}

func TestAccountDeletionEmptyOwnershipAndSystemProtection(t *testing.T) {
	f := newInvestmentDBFixture(t)
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio"}
	p := deletionPreview(t, f, input)
	require.Zero(t, p.InvestmentCount)
	items, err := f.s.Accounts(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, items, 1, "preview must not delete")
	_, err = f.s.DeleteAssetAccount(nil, f.uid+1, input, true)
	require.ErrorIs(t, err, errs.ErrAccountNotFound)
	input.Token = p.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	items, err = f.s.Accounts(nil, f.uid)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = f.s.Mutate(nil, f.uid, f.event(investments.Buy, "0.01", "100", "0", 1), "deleted-account-write", "create", false)
	require.Error(t, err)
}

func TestAccountDeletionCashTransferAndRelatedData(t *testing.T) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Account{AccountId: 1003, Uid: f.uid, Name: "美元账户", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "USD"})
	require.NoError(t, err)
	_, err = f.engine.Insert(&models.TransactionCategory{CategoryId: 500, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, Name: "转账"}, &models.TransactionCategory{CategoryId: 501, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 500, Name: "账户互转"})
	require.NoError(t, err)
	tx := &models.Transaction{Uid: f.uid, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, CategoryId: 501, AccountId: f.bankID, RelatedAccountId: 1003, Amount: 10000, RelatedAccountAmount: 1400, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 1)}
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	_, err = f.engine.Insert(&models.TransactionTagIndex{Uid: f.uid, TransactionId: tx.TransactionId, TagId: 1}, &models.TransactionPictureInfo{Uid: f.uid, TransactionId: tx.TransactionId, PictureId: 1}, &models.TransactionTemplate{Uid: f.uid, TemplateId: 1, AccountId: 1003, RelatedAccountId: f.bankID, TemplateType: models.TRANSACTION_TEMPLATE_TYPE_SCHEDULE}, &models.MonetaryIncomeBinding{Uid: f.uid, AccountId: f.bankID, Enabled: true})
	require.NoError(t, err)
	input := AccountDeletionInput{ID: fmt.Sprint(f.bankID), Kind: "cash"}
	p := deletionPreview(t, f, input)
	require.Equal(t, 2, p.TransactionCount)
	require.Equal(t, 1, p.TemplateCount)
	require.Contains(t, p.AffectedAccounts, "美元账户")
	require.Equal(t, int64(1990000), f.balance(), "preview must not reverse transfer")
	input.Token = p.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.Error(t, err, "related data requires explicit consent")
	input.DeleteRelated = true
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	var bank, other models.Account
	_, err = f.engine.ID(f.bankID).Get(&bank)
	require.NoError(t, err)
	require.True(t, bank.Deleted)
	require.Zero(t, bank.Balance)
	_, err = f.engine.ID(1003).Get(&other)
	require.NoError(t, err)
	require.False(t, other.Deleted)
	require.Zero(t, other.Balance)
	for _, bean := range []any{&models.Transaction{}, &models.TransactionTagIndex{}, &models.TransactionPictureInfo{}, &models.TransactionTemplate{}} {
		count, err := f.engine.Where("uid=? AND deleted=?", f.uid, false).Count(bean)
		require.NoError(t, err)
		require.Zero(t, count)
	}
	var binding models.MonetaryIncomeBinding
	_, err = f.engine.Where("uid=? AND account_id=?", f.uid, f.bankID).Get(&binding)
	require.NoError(t, err)
	require.False(t, binding.Enabled)
}

func TestAccountDeletionPortfolioBatchReplayAndAudit(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "100", "1", 1), "delete-batch-buy-1")
	f.create(f.event(investments.Buy, "0.02", "200", "2", 2), "delete-batch-buy-2")
	f.create(f.event(investments.Sell, "0.015", "180", "1", 3), "delete-batch-sell")
	f.quote("10000")
	f.summary(true)
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio", DeleteRelated: true}
	p := deletionPreview(t, f, input)
	require.Equal(t, 3, p.InvestmentCount)
	require.Empty(t, p.BlockedReason)
	input.Token = p.Token
	_, err := f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance(), "reverse all settlement flows once")
	require.Empty(t, f.summary(false).Positions)
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, events, 3)
	for _, event := range events {
		require.True(t, event.Voided)
		require.Equal(t, 2, event.Version)
	}
	require.Equal(t, int64(6), f.count(&models.InvestmentEventRevision{}))
	count, err := f.engine.Where("uid=? AND deleted=? AND investment_event_id<>?", f.uid, false, "").Count(&models.Transaction{})
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = f.engine.Where("uid=? AND invalidated=?", f.uid, true).Count(&models.WealthSnapshot{})
	require.NoError(t, err)
	require.Positive(t, count)
	_, err = f.s.Mutate(nil, f.uid, f.event(investments.Buy, "0.01", "100", "0", 4), "write-deleted-account", "create", false)
	require.Error(t, err)
}

func TestAccountDeletionCashVoidsLinkedInvestments(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "100", "0", 1), "delete-cash-investment")
	input := AccountDeletionInput{ID: fmt.Sprint(f.bankID), Kind: "cash", DeleteRelated: true}
	p := deletionPreview(t, f, input)
	require.Equal(t, 1, p.InvestmentCount)
	require.Contains(t, p.AffectedAccounts, "fixture portfolio")
	input.Token = p.Token
	_, err := f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	require.Empty(t, f.summary(false).Positions)
	var system models.Account
	has, err := f.engine.Where("uid=? AND system_role=?", f.uid, InvestmentSettlementRole).Get(&system)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, system.Deleted)
	require.Zero(t, system.Balance)
	_, err = f.s.DeleteAssetAccount(nil, f.uid, AccountDeletionInput{ID: fmt.Sprint(system.AccountId), Kind: "cash"}, true)
	require.ErrorIs(t, err, ErrInvestmentLinked)
}

func TestAccountDeletionStalePreviewAndDependencyRollback(t *testing.T) {
	f := newInvestmentDBFixture(t)
	initial := f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: "crypto:bitcoin", Quantity: "1", OccurredAt: f.at + 1}}, "delete-dependency-opening")
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio", DeleteRelated: true}
	p := deletionPreview(t, f, input)
	input.Token = p.Token
	other, err := f.s.CreateAccount(nil, f.uid, models.PortfolioAccount{Name: "另一个钱包", Kind: "WALLET"})
	require.NoError(t, err)
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Transfer, AccountID: f.portfolioID, ToAccountID: other.Id, InstrumentID: "crypto:bitcoin", Quantity: "0.5", OccurredAt: f.at + 2}}, "delete-dependency-transfer")
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.ErrorContains(t, err, "已变化")
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Sell, AccountID: other.Id, InstrumentID: "crypto:bitcoin", Quantity: "0.1", Amount: "10", Fee: "0", OccurredAt: f.at + 3}, CashAccountID: fmt.Sprint(f.bankID)}, "delete-dependency-sale")
	p = deletionPreview(t, f, input)
	require.NotEmpty(t, p.BlockedReason)
	input.Token = p.Token
	before := f.balance()
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.Error(t, err)
	require.Equal(t, before, f.balance())
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	for _, event := range events {
		require.False(t, event.Voided)
	}
	require.Equal(t, initial.Event.Version, events[0].Version)
}

func TestAccountDeletionLateFailureRollsBackEverything(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "100", "0", 1), "delete-rollback-buy")
	tableInfo, err := f.engine.TableInfo(new(models.PortfolioAccount))
	require.NoError(t, err)
	table := tableInfo.Name
	_, err = f.engine.Exec("CREATE TRIGGER reject_account_delete BEFORE DELETE ON " + table + " BEGIN SELECT RAISE(ABORT, 'forced failure'); END")
	require.NoError(t, err)
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio", DeleteRelated: true}
	p := deletionPreview(t, f, input)
	input.Token = p.Token
	before := f.balance()
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.ErrorContains(t, err, "forced failure")
	require.Equal(t, before, f.balance())
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.False(t, events[0].Voided)
	require.Equal(t, 1, events[0].Version)
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRevision{}))
	count, err := f.engine.Where("uid=? AND deleted=? AND investment_event_id<>?", f.uid, false, "").Count(&models.Transaction{})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestAccountDeletionFinalChildAndLargeLedger(t *testing.T) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Account{AccountId: 1002, Uid: f.uid, Name: "多币种主账户", Type: models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS, Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, Currency: "CNY"})
	require.NoError(t, err)
	_, err = f.engine.ID(f.bankID).Cols("parent_account_id").Update(&models.Account{ParentAccountId: 1002})
	require.NoError(t, err)
	for i := 0; i < 250; i++ {
		_, err = f.engine.Insert(&models.Transaction{TransactionId: int64(9000 + i), Uid: f.uid, AccountId: f.bankID, Type: models.TRANSACTION_DB_TYPE_INCOME, Amount: 100, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + int64(i))})
		require.NoError(t, err)
	}
	_, err = f.engine.ID(f.bankID).Incr("balance", 25000).Update(&models.Account{})
	require.NoError(t, err)
	input := AccountDeletionInput{ID: fmt.Sprint(f.bankID), Kind: "cash", DeleteRelated: true}
	p := deletionPreview(t, f, input)
	require.Equal(t, "多币种主账户", p.ParentAccountName)
	require.Equal(t, 251, p.TransactionCount)
	input.Token = p.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	count, err := f.engine.Where("uid=? AND deleted=?", f.uid, false).Count(&models.Transaction{})
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = f.engine.Where("uid=? AND deleted=?", f.uid, false).Count(&models.Account{})
	require.NoError(t, err)
	require.Zero(t, count, "last child must not leave an empty parent")
}
