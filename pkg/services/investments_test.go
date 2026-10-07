package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

// These tests exercise the actual SQLite transaction boundary and existing cash
// ledger, with isolated temporary files. They never start a public market feed.
type investmentDBFixture struct {
	t           *testing.T
	s           *InvestmentService
	engine      *xorm.Engine
	uid         int64
	bankID      int64
	portfolioID string
	at          int64
}

func newInvestmentDBFixture(t *testing.T) *investmentDBFixture {
	t.Helper()
	previousDB := *datastore.Container
	previousUUID := *uuid.Container
	previousMarket := marketquotes.Default
	config := &settings.Config{
		DatabaseConfig:    &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType, DatabasePath: filepath.Join(t.TempDir(), "investment-test.db"), MaxIdleConnection: 1, MaxOpenConnection: 1},
		UuidGeneratorType: settings.InternalUuidGeneratorType,
		UuidServerId:      231,
	}
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, uuid.InitializeUuidGenerator(config))
	marketquotes.Default = marketquotes.New(marketquotes.Config{})
	s := &InvestmentService{ServiceUsingDB: ServiceUsingDB{container: datastore.Container}}
	session := s.UserDataDB(1).NewSession(nil)
	engine := session.Engine()
	require.NoError(t, session.Close())
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
		*datastore.Container = previousDB
		*uuid.Container = previousUUID
		marketquotes.Default = previousMarket
	})
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(
		&models.StatisticsBudget{}, &models.StatisticsNote{}, &models.StatisticsPreference{},
		&models.LocalLedgerItem{},
		&models.Book{}, &models.CalendarEvent{}, &models.AssetPresentation{}, &models.FixedDeposit{}, &models.ReimbursementReceipt{}, &models.AssetAdjustment{}, &models.CreditInstallment{}, &models.DebtMovement{}, &models.MonetaryIncomeBinding{}, &models.MonetaryIncomeDay{}, &models.TransactionTemplate{}, &models.Account{}, &models.Transaction{}, &models.TransactionCategory{}, &models.TransactionTag{}, &models.TransactionTagIndex{}, &models.TransactionPictureInfo{},
		&models.PortfolioAccount{}, &models.InvestmentInstrument{}, &models.InvestmentSettings{}, &models.InvestmentEventRecord{}, &models.InvestmentEventRevision{},
		&models.InvestmentTransactionLink{}, &models.InvestmentIdempotency{}, &models.InvestmentQuote{}, &models.WealthSnapshot{},
		&models.CryptoDCAPlan{}, &models.CryptoDCADay{},
	))
	f := &investmentDBFixture{t: t, s: s, engine: engine, uid: 101, bankID: 1001, at: time.Now().Add(-time.Hour).Unix()}
	_, err := engine.Insert(&models.Account{AccountId: f.bankID, Uid: f.uid, Name: "fixture bank", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, Currency: "CNY", CreatedUnixTime: f.at - 120})
	require.NoError(t, err)
	require.NoError(t, Transactions.CreateTransaction(nil, &models.Transaction{Uid: f.uid, AccountId: f.bankID, Type: models.TRANSACTION_DB_TYPE_MODIFY_BALANCE, Amount: 2000000, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at - 60)}, nil, nil))
	account, err := s.CreateAccount(nil, f.uid, models.PortfolioAccount{Name: "fixture portfolio", Kind: "WALLET"})
	require.NoError(t, err)
	f.portfolioID = account.Id
	return f
}

func (f *investmentDBFixture) event(kind, quantity, amount, fee string, offset int64) InvestmentEvent {
	return InvestmentEvent{Event: investments.Event{Type: kind, AccountID: f.portfolioID, InstrumentID: "crypto:bitcoin", Quantity: quantity, Amount: amount, Fee: fee, OccurredAt: f.at + offset}, CashAccountID: fmt.Sprint(f.bankID)}
}

func (f *investmentDBFixture) create(event InvestmentEvent, key string) *InvestmentPreview {
	f.t.Helper()
	result, err := f.s.Mutate(nil, f.uid, event, key, "create", false)
	require.NoError(f.t, err)
	require.NotNil(f.t, result)
	return result
}

func (f *investmentDBFixture) balance() int64 {
	f.t.Helper()
	var account models.Account
	has, err := f.engine.ID(f.bankID).Get(&account)
	require.NoError(f.t, err)
	require.True(f.t, has)
	return account.Balance
}

func (f *investmentDBFixture) count(bean interface{}) int64 {
	f.t.Helper()
	count, err := f.engine.Where("uid=?", f.uid).Count(bean)
	require.NoError(f.t, err)
	return count
}

func (f *investmentDBFixture) quote(price string) {
	f.t.Helper()
	_, err := f.s.ManualQuote(nil, f.uid, "crypto:bitcoin", price, time.Now().Unix(), "")
	require.NoError(f.t, err)
}

func (f *investmentDBFixture) summary(save bool) *WealthSummary {
	f.t.Helper()
	result, err := f.s.Summary(nil, f.uid, save)
	require.NoError(f.t, err)
	return result
}

func requireMoney(t *testing.T, expected string, actual *string) {
	t.Helper()
	require.NotNil(t, actual)
	require.Equal(t, expected, *actual)
}

func TestInvestmentDBFixedFixtureAndLinkedCashTransfers(t *testing.T) {
	f := newInvestmentDBFixture(t)
	first := f.create(f.event(investments.Buy, "0.01", "6000", "6", 1), "fixture-buy-1")
	require.Equal(t, int64(1399400), f.balance())
	second := f.create(f.event(investments.Buy, "0.01", "7000", "7", 2), "fixture-buy-2")
	require.Equal(t, int64(698700), f.balance())
	require.Len(t, second.Positions, 1)
	require.Equal(t, "0.02", second.Positions[0].Quantity)
	requireMoney(t, "13013", second.Positions[0].Cost)
	requireMoney(t, "650650", second.Positions[0].AverageCost)
	f.quote("750000")
	summary := f.summary(false)
	requireMoney(t, "21987", summary.NetAssets)
	require.Equal(t, "15000", summary.InvestmentValue)
	requireMoney(t, "1987", summary.UnrealizedPNL)
	require.Len(t, summary.CashAccounts, 1, "system settlement must not count as user assets")
	require.Equal(t, "6987", summary.CashAccounts[0].Balance)
	third := f.create(f.event(investments.Sell, "0.005", "4000", "4", 3), "fixture-sell-1")
	require.Equal(t, int64(1098300), f.balance())
	require.Equal(t, "0.015", third.Positions[0].Quantity)
	requireMoney(t, "9759.75", third.Positions[0].Cost)
	requireMoney(t, "742.75", third.Positions[0].RealizedPNL)
	for _, effect := range third.Effects {
		if effect.EventID == third.Event.ID {
			require.Equal(t, "3996", effect.CashDelta)
			requireMoney(t, "3253.25", effect.ReleasedCost)
			requireMoney(t, "742.75", effect.RealizedPNL)
		}
	}
	f.quote("800000")
	summary = f.summary(true)
	requireMoney(t, "22983", summary.NetAssets)
	requireMoney(t, "2240.25", summary.UnrealizedPNL)
	requireMoney(t, "742.75", summary.RealizedPNL)
	require.Equal(t, "12000", summary.InvestmentValue)
	require.Equal(t, 0, summary.MissingPrices)
	require.Equal(t, "manual", summary.Positions[0].Quote.State)

	var txs []models.Transaction
	require.NoError(t, f.engine.Where("uid=? AND deleted=? AND investment_event_id<>?", f.uid, false, "").Find(&txs))
	require.Len(t, txs, 6)
	require.Equal(t, int64(6), f.count(&models.InvestmentTransactionLink{}))
	require.Equal(t, int64(3), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(3), f.count(&models.InvestmentEventRevision{}))
	byID := make(map[int64]models.Transaction)
	for _, tx := range txs {
		byID[tx.TransactionId] = tx
	}
	for _, tx := range txs {
		require.Contains(t, []string{first.Event.ID, second.Event.ID, third.Event.ID}, tx.InvestmentEventId)
		require.Contains(t, []models.TransactionDbType{models.TRANSACTION_DB_TYPE_TRANSFER_IN, models.TRANSACTION_DB_TYPE_TRANSFER_OUT}, tx.Type)
		paired := byID[tx.RelatedId]
		require.Equal(t, tx.TransactionId, paired.RelatedId)
		require.Equal(t, tx.InvestmentEventId, paired.InvestmentEventId)
		require.Equal(t, tx.Amount, paired.Amount)
		require.Equal(t, tx.AccountId, paired.RelatedAccountId)
	}
	publicAccounts, err := Accounts.GetAllAccountsByUid(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, publicAccounts, 1)
	require.Equal(t, f.bankID, publicAccounts[0].AccountId)
	publicCount, err := Accounts.GetTotalAccountCountByUid(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, int64(1), publicCount)
	income, expense, err := Transactions.GetAccountsTotalIncomeAndExpense(nil, f.uid, f.at-1, time.Now().Unix(), nil, nil, time.UTC, false)
	require.NoError(t, err)
	require.Empty(t, income, "sale proceeds are not lifestyle income")
	require.Empty(t, expense, "investment purchases are not lifestyle expenses")
	// A newly constructed service reads facts and manual quote from SQLite;
	// quantities and costs do not depend on the previous in-memory response.
	reloaded := &InvestmentService{ServiceUsingDB: ServiceUsingDB{container: datastore.Container}}
	restoredSummary, err := reloaded.Summary(nil, f.uid, false)
	require.NoError(t, err)
	require.Equal(t, summary, restoredSummary)
}

func TestInvestmentDBPreviewIdempotencyAndAtomicRollback(t *testing.T) {
	f := newInvestmentDBFixture(t)
	buy := f.event(investments.Buy, "0.01", "6000", "6", 1)
	preview, err := f.s.Mutate(nil, f.uid, buy, "", "create", true)
	require.NoError(t, err)
	require.Equal(t, "0.01", preview.Positions[0].Quantity)
	require.Equal(t, int64(2000000), f.balance())
	for _, bean := range []interface{}{&models.InvestmentEventRecord{}, &models.InvestmentIdempotency{}, &models.InvestmentSettings{}, &models.InvestmentTransactionLink{}} {
		require.Zero(t, f.count(bean), "preview must roll back %T", bean)
	}
	require.Equal(t, int64(1), f.count(&models.Transaction{}), "only original bank balance record remains")
	require.Equal(t, int64(1), f.count(&models.Account{}), "preview system account must roll back")

	// Fail the event write after real cash movements and transfer records have
	// been inserted. This proves database atomicity, not merely early validation.
	table := f.engine.TableName(&models.InvestmentEventRecord{})
	_, err = f.engine.Exec("CREATE TRIGGER fail_investment_fact BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(ABORT, 'injected investment fact failure'); END")
	require.NoError(t, err)
	_, err = f.s.Mutate(nil, f.uid, buy, "rollback-buy", "create", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "injected investment fact failure")
	require.Equal(t, int64(2000000), f.balance())
	require.Equal(t, int64(1), f.count(&models.Transaction{}))
	require.Zero(t, f.count(&models.InvestmentTransactionLink{}))
	require.Zero(t, f.count(&models.InvestmentSettings{}))
	require.Equal(t, int64(1), f.count(&models.Account{}))
	_, err = f.engine.Exec("DROP TRIGGER fail_investment_fact")
	require.NoError(t, err)

	created := f.create(buy, "idempotent-buy")
	retry, err := f.s.Mutate(nil, f.uid, buy, "idempotent-buy", "create", false)
	require.NoError(t, err)
	require.Equal(t, created, retry)
	require.Equal(t, int64(1399400), f.balance())
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(3), f.count(&models.Transaction{}))
	changed := buy
	changed.Amount = "6001"
	_, err = f.s.Mutate(nil, f.uid, changed, "idempotent-buy", "create", false)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	require.Equal(t, int64(1399400), f.balance())

	_, err = f.s.Mutate(nil, f.uid, f.event(investments.Sell, "0.02", "12000", "0", 2), "invalid-oversell", "create", false)
	require.Error(t, err)
	_, err = f.s.Mutate(nil, f.uid, f.event(investments.Buy, "1", "20000", "0", 3), "invalid-overdraw", "create", false)
	require.Error(t, err)
	require.Equal(t, int64(1399400), f.balance())
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(1), f.count(&models.InvestmentIdempotency{}))
	require.Equal(t, int64(3), f.count(&models.Transaction{}))
	// Retrying the rejected key with valid content is allowed: failures must not
	// persist their idempotency record before the transaction commits.
	f.create(f.event(investments.Sell, "0.005", "3500", "0", 2), "invalid-oversell")
	require.Equal(t, int64(1749400), f.balance())
}

func TestInvestmentDBRevisionVoidAndSnapshotInvalidation(t *testing.T) {
	f := newInvestmentDBFixture(t)
	first := f.create(f.event(investments.Buy, "0.01", "6000", "6", 1), "revision-buy-1")
	f.create(f.event(investments.Buy, "0.01", "7000", "7", 2), "revision-buy-2")
	sale := f.create(f.event(investments.Sell, "0.005", "4000", "4", 3), "revision-sell")
	f.quote("800000")
	f.summary(true)
	history, err := f.s.History(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.False(t, history[0].Invalidated)
	requireMoney(t, "22983", history[0].NetAssets)
	revised := first.Event
	revised.Amount = "5000"
	result, err := f.s.Mutate(nil, f.uid, revised, "", "revise", false)
	require.NoError(t, err)
	require.Equal(t, 2, result.Event.Version)
	require.Equal(t, int64(1198300), f.balance(), "old settlement is reversed before new settlement")
	requireMoney(t, "9009.75", result.Positions[0].Cost)
	requireMoney(t, "992.75", result.Positions[0].RealizedPNL)
	var invalidated models.WealthSnapshot
	has, err := f.engine.ID(history[0].Id).Get(&invalidated)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, invalidated.Invalidated, "the edit atomically marks the stored observation invalid")
	f.quote("900000") // a newer price must never leak into historical rebuilding
	history, err = f.s.History(nil, f.uid)
	require.NoError(t, err)
	require.False(t, history[0].Invalidated)
	requireMoney(t, "23983", history[0].NetAssets)
	require.True(t, history[0].Complete)
	_, err = f.s.Mutate(nil, f.uid, revised, "", "revise", false)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	require.Equal(t, int64(1198300), f.balance())
	voided, err := f.s.Mutate(nil, f.uid, sale.Event, "", "void", false)
	require.NoError(t, err)
	require.True(t, voided.Event.Voided)
	require.Equal(t, int64(798700), f.balance())
	require.Equal(t, "0.02", voided.Positions[0].Quantity)
	requireMoney(t, "12013", voided.Positions[0].Cost)
	requireMoney(t, "0", voided.Positions[0].RealizedPNL)
	_, err = f.s.Mutate(nil, f.uid, sale.Event, "", "void", false)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	var active []models.Transaction
	require.NoError(t, f.engine.Where("uid=? AND deleted=? AND investment_event_id<>?", f.uid, false, "").Find(&active))
	require.Len(t, active, 4)
	require.Equal(t, int64(5), f.count(&models.InvestmentEventRevision{}))
}

func TestInvestmentDBUserIsolationAndMissingCostOrPrice(t *testing.T) {
	f := newInvestmentDBFixture(t)
	custom, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Type: "FUND", Name: "private fund", Symbol: "FUND-X"})
	require.NoError(t, err)
	opening := InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: custom.Id, Quantity: "12.5", OccurredAt: f.at + 1, Note: "private investment note"}}
	created := f.create(opening, "private-opening")
	require.Equal(t, int64(2000000), f.balance(), "opening must not debit cash")
	unpriced := f.summary(true)
	require.Nil(t, unpriced.NetAssets)
	require.Equal(t, "20000", unpriced.ValuedAssets)
	require.Equal(t, 1, unpriced.MissingPrices)
	require.Nil(t, unpriced.Positions[0].MarketValue)
	require.Nil(t, unpriced.Positions[0].Cost)
	require.Nil(t, unpriced.UnrealizedPNL)
	_, err = f.s.ManualQuote(nil, f.uid, custom.Id, "2.4", time.Now().Unix(), "")
	require.NoError(t, err)
	priced := f.summary(false)
	requireMoney(t, "20030", priced.NetAssets)
	requireMoney(t, "30", priced.Positions[0].MarketValue)
	require.Nil(t, priced.Positions[0].UnrealizedPNL, "unknown cost must not become zero cost")
	require.Nil(t, priced.UnrealizedPNL)
	require.False(t, priced.CostComplete)
	revisedOpening := created.Event
	revisedOpening.Quantity = "15"
	_, err = f.s.Mutate(nil, f.uid, revisedOpening, "", "revise", false)
	require.NoError(t, err)
	history, err := f.s.History(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Nil(t, history[0].NetAssets, "missing historical price stays unknown despite a current manual quote")
	require.False(t, history[0].Complete)
	require.NoError(t, f.s.RemoveManualQuote(nil, f.uid, custom.Id))
	require.Nil(t, f.summary(false).NetAssets)

	const otherUID int64 = 202
	otherAccount, err := f.s.CreateAccount(nil, otherUID, models.PortfolioAccount{Name: "other portfolio"})
	require.NoError(t, err)
	_, err = f.engine.Insert(&models.Account{AccountId: 2002, Uid: otherUID, Name: "other bank", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, Currency: "CNY", Balance: 9900})
	require.NoError(t, err)
	_, err = f.s.Mutate(nil, otherUID, opening, "other-use-account", "create", false)
	require.Error(t, err)
	opening.AccountID = otherAccount.Id
	_, err = f.s.Mutate(nil, otherUID, opening, "other-use-asset", "create", false)
	require.Error(t, err)
	otherBuy := f.event(investments.Buy, "0.001", "500", "0", 2)
	otherBuy.AccountID = otherAccount.Id
	_, err = f.s.Mutate(nil, otherUID, otherBuy, "other-use-cash", "create", false)
	require.Error(t, err)
	_, err = f.s.Mutate(nil, otherUID, created.Event, "", "void", false)
	require.Error(t, err)
	_, err = f.s.ManualQuote(nil, otherUID, custom.Id, "100", time.Now().Unix(), "")
	require.Error(t, err)
	otherEvents, err := f.s.Events(nil, otherUID)
	require.NoError(t, err)
	require.Empty(t, otherEvents)
	otherInstruments, err := f.s.Instruments(nil, otherUID)
	require.NoError(t, err)
	for _, instrument := range otherInstruments {
		require.NotEqual(t, custom.Id, instrument.Id)
	}
	otherSummary, err := f.s.Summary(nil, otherUID, false)
	require.NoError(t, err)
	requireMoney(t, "99", otherSummary.NetAssets)
	require.Empty(t, otherSummary.Positions)
	otherExport, err := f.s.Export(nil, otherUID)
	require.NoError(t, err)
	require.NotContains(t, otherExport, "private investment note")
}

func TestInvestmentDBLegacyGuardsAndHiddenSystemAccounts(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "6000", "6", 1), "legacy-guard-buy")
	var out models.Transaction
	has, err := f.engine.Where("uid=? AND type=? AND deleted=?", f.uid, models.TRANSACTION_DB_TYPE_TRANSFER_OUT, false).Get(&out)
	require.NoError(t, err)
	require.True(t, has)
	var system models.Account
	has, err = f.engine.Where("uid=? AND system_role=?", f.uid, InvestmentSettlementRole).Get(&system)
	require.NoError(t, err)
	require.True(t, has)
	_, err = Accounts.GetAccountByAccountId(nil, f.uid, system.AccountId)
	require.ErrorIs(t, err, errs.ErrAccountNotFound)
	checks := map[string]func() error{
		"modify linked transaction": func() error {
			copy := out
			copy.Comment = "bypass"
			return Transactions.ModifyTransaction(nil, &copy, false, 0, nil, nil, nil, nil)
		},
		"delete linked transaction": func() error { return Transactions.DeleteTransaction(nil, f.uid, out.TransactionId) },
		"batch category": func() error {
			return Transactions.BatchUpdateTransactionsCategory(nil, f.uid, []int64{out.TransactionId}, 123)
		},
		"batch add tags": func() error {
			return Transactions.BatchAddTagsToTransactions(nil, f.uid, []*models.Transaction{&out}, map[int64][]int64{out.TransactionId: {123}})
		},
		"batch remove tags": func() error {
			return Transactions.BatchRemoveTagsFromTransactions(nil, f.uid, []int64{out.TransactionId}, []int64{123})
		},
		"batch clear tags": func() error {
			return Transactions.BatchClearAllTagsFromTransactions(nil, f.uid, []int64{out.TransactionId})
		},
		"delete all": func() error { return Transactions.DeleteAllTransactions(nil, f.uid, false) },
		"move all":   func() error { return Transactions.MoveAllTransactionsBetweenAccounts(nil, f.uid, f.bankID, 9999) },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) { require.ErrorIs(t, check(), ErrInvestmentLinked) })
	}
	require.Error(t, Accounts.HideAccount(nil, f.uid, []int64{system.AccountId}, true))
	require.Error(t, Accounts.DeleteAccount(nil, f.uid, system.AccountId))
	require.Equal(t, int64(1399400), f.balance())
	require.Equal(t, int64(3), f.count(&models.Transaction{}))
	// A guessed system account ID must also be rejected by ordinary create and
	// import services; excluding it from the dropdown is not an authorization check.
	_, err = f.engine.Insert(&models.TransactionCategory{CategoryId: 500, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, Name: "transfer parent"}, &models.TransactionCategory{CategoryId: 501, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 500, Name: "transfer child"})
	require.NoError(t, err)
	ordinary := &models.Transaction{Uid: f.uid, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, AccountId: f.bankID, RelatedAccountId: system.AccountId, CategoryId: 501, Amount: 100, RelatedAccountAmount: 100, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 5)}
	require.Error(t, Transactions.CreateTransaction(nil, ordinary, nil, nil), "public cash mutations must reject the system settlement account")
	ordinary.TransactionId, ordinary.RelatedId = 0, 0
	require.Error(t, Transactions.BatchCreateTransactions(nil, f.uid, []*models.Transaction{ordinary}, nil, nil), "CSV import must not bypass system-role protection")
	require.Equal(t, int64(1399400), f.balance())
	_, err = f.engine.Insert(&models.Account{AccountId: 1003, Uid: f.uid, Name: "normal transfer destination", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "CNY"})
	require.NoError(t, err)
	ordinary.TransactionId, ordinary.RelatedId, ordinary.RelatedAccountId = 0, 0, 1003
	require.NoError(t, Transactions.CreateTransaction(nil, ordinary, nil, nil))
	require.Equal(t, int64(1399300), f.balance())
	ordinary.RelatedAccountId = system.AccountId
	require.Error(t, Transactions.ModifyTransaction(nil, ordinary, false, 0, nil, nil, nil, nil), "editing an ordinary transaction must not target a system account")
	require.Equal(t, int64(1399300), f.balance())
}

func TestInvestmentDBConcurrentSellNeverOversells(t *testing.T) {
	f := newInvestmentDBFixture(t)
	initial := f.create(f.event(investments.Buy, "0.01", "6000", "0", 1), "concurrent-buy")
	var group sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			_, err := f.s.Mutate(nil, f.uid, f.event(investments.Sell, "0.008", "5000", "0", 2+int64(index)), fmt.Sprintf("concurrent-sell-%d", index), "create", false)
			errors <- err
		}(i)
	}
	group.Wait()
	close(errors)
	successes, failures := 0, 0
	for err := range errors {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)
	require.Equal(t, int64(1900000), f.balance())
	summary := f.summary(false)
	require.Equal(t, "0.002", summary.Positions[0].Quantity)
	requireMoney(t, "1200", summary.Positions[0].Cost)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	invalidRevision := initial.Event
	invalidRevision.Quantity = "0.007"
	_, err := f.s.Mutate(nil, f.uid, invalidRevision, "", "revise", false)
	require.Error(t, err, "a historical edit must not make a later sale negative")
	require.Equal(t, int64(1900000), f.balance())
	require.Equal(t, "0.002", f.summary(false).Positions[0].Quantity)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRevision{}))
}

func TestInvestmentDBManualQuoteChangesNoFactsOrCosts(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "6000", "0", 1), "quote-only-buy")
	before, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	beforeJSON, err := json.Marshal(before)
	require.NoError(t, err)
	f.quote("700000")
	first := f.summary(false)
	f.quote("800000")
	second := f.summary(false)
	after, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	afterJSON, err := json.Marshal(after)
	require.NoError(t, err)
	require.Equal(t, string(beforeJSON), string(afterJSON))
	require.Equal(t, int64(1400000), f.balance())
	require.Equal(t, first.Positions[0].Position, second.Positions[0].Position)
	requireMoney(t, "1000", first.UnrealizedPNL)
	requireMoney(t, "2000", second.UnrealizedPNL)
	exported, err := f.s.Export(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, 2, len(strings.Split(strings.TrimSpace(exported), "\n")))
}

func TestInvestmentDBMoveAllProtectsSystemAfterVoidAndAllowsUnrelatedAccounts(t *testing.T) {
	f := newInvestmentDBFixture(t)
	buy := f.create(f.event(investments.Buy, "0.01", "6000", "6", 1), "move-guard-buy")
	for index, id := range []int64{1003, 1004} {
		_, err := f.engine.Insert(&models.Account{AccountId: id, Uid: f.uid, Name: fmt.Sprintf("ordinary account %d", index), Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "CNY"})
		require.NoError(t, err)
		require.NoError(t, Transactions.CreateTransaction(nil, &models.Transaction{Uid: f.uid, AccountId: id, Type: models.TRANSACTION_DB_TYPE_MODIFY_BALANCE, Amount: int64(500 + 200*index), TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at - 50 + int64(index))}, nil, nil))
	}
	// Active investment links in the bank do not justify blocking an unrelated
	// merge between two ordinary cash accounts.
	require.NoError(t, Transactions.MoveAllTransactionsBetweenAccounts(nil, f.uid, 1003, 1004))
	var from, to models.Account
	has, err := f.engine.ID(1003).Get(&from)
	require.NoError(t, err)
	require.True(t, has)
	has, err = f.engine.ID(1004).Get(&to)
	require.NoError(t, err)
	require.True(t, has)
	require.Zero(t, from.Balance)
	require.Equal(t, int64(1200), to.Balance)
	require.Equal(t, int64(1399400), f.balance())
	_, err = f.s.Mutate(nil, f.uid, buy.Event, "", "void", false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	activeLinks, err := f.engine.Where("uid=? AND deleted=? AND investment_event_id<>?", f.uid, false, "").Count(&models.Transaction{})
	require.NoError(t, err)
	require.Zero(t, activeLinks)
	var system models.Account
	has, err = f.engine.Where("uid=? AND system_role=?", f.uid, InvestmentSettlementRole).Get(&system)
	require.NoError(t, err)
	require.True(t, has)
	require.Zero(t, system.Balance)
	// Once every linked transfer is voided, checking active links alone is no
	// longer sufficient. The role itself still forbids ordinary merges.
	require.ErrorIs(t, Transactions.MoveAllTransactionsBetweenAccounts(nil, f.uid, f.bankID, system.AccountId), ErrInvestmentLinked)
	require.ErrorIs(t, Transactions.MoveAllTransactionsBetweenAccounts(nil, f.uid, system.AccountId, f.bankID), ErrInvestmentLinked)
	require.Equal(t, int64(2000000), f.balance())
	system = models.Account{AccountId: system.AccountId}
	has, err = f.engine.ID(system.AccountId).Get(&system)
	require.NoError(t, err)
	require.True(t, has)
	require.Zero(t, system.Balance)
}

func TestInvestmentDBFiatScientificNotationRejectedBeforeArithmetic(t *testing.T) {
	f := newInvestmentDBFixture(t)
	cases := []struct{ amount, fee string }{
		{"1e999", "0"}, {"1e-999", "0"}, {"1e2", "0"},
		{"100", "1e999"}, {"100", "1e-999"}, {" 100", "0"},
	}
	for index, sample := range cases {
		_, err := f.s.Mutate(nil, f.uid, f.event(investments.Buy, "0.001", sample.amount, sample.fee, int64(index+1)), fmt.Sprintf("invalid-decimal-%d", index), "create", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "普通十进制字符串", "reject notation before decimal magnitude or scale checks")
	}
	require.Equal(t, int64(2000000), f.balance())
	require.Zero(t, f.count(&models.InvestmentEventRecord{}))
	require.Zero(t, f.count(&models.InvestmentIdempotency{}))
	require.Equal(t, int64(1), f.count(&models.Transaction{}))
}

func TestInvestmentDBExportTransferIncludesBothInvestmentAccounts(t *testing.T) {
	f := newInvestmentDBFixture(t)
	f.create(f.event(investments.Buy, "0.01", "6000", "0", 1), "export-transfer-buy")
	destination, err := f.s.CreateAccount(nil, f.uid, models.PortfolioAccount{Name: "destination wallet", Kind: "WALLET"})
	require.NoError(t, err)
	transfer := f.create(InvestmentEvent{Event: investments.Event{Type: investments.Transfer, AccountID: f.portfolioID, ToAccountID: destination.Id, InstrumentID: "crypto:bitcoin", Quantity: "0.003", Fee: "0", OccurredAt: f.at + 2}}, "export-own-transfer")
	exported, err := f.s.Export(nil, f.uid)
	require.NoError(t, err)
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(exported, "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 3)
	columns := make(map[string]int)
	for index, header := range rows[0] {
		columns[header] = index
	}
	require.Contains(t, columns, "转入投资账户ID")
	var exportedTransfer []string
	for _, row := range rows[1:] {
		if row[columns["记录ID"]] == transfer.Event.ID {
			exportedTransfer = row
		}
	}
	require.NotEmpty(t, exportedTransfer)
	require.Equal(t, f.portfolioID, exportedTransfer[columns["投资账户ID"]])
	require.Equal(t, destination.Id, exportedTransfer[columns["转入投资账户ID"]])
	require.Equal(t, investments.Transfer, exportedTransfer[columns["类型"]])
	require.Equal(t, "crypto:bitcoin", exportedTransfer[columns["资产ID"]])
	require.Equal(t, "0.003", exportedTransfer[columns["数量"]])
	require.Equal(t, int64(1400000), f.balance(), "own-asset transfer must not settle cash again")
}

func TestInvestmentDBForeignCashFXStalenessIsVisibleWithoutPositions(t *testing.T) {
	zero := "0"
	result := &investments.Result{Positions: []investments.Position{}, RealizedPNL: &zero}
	cash := []models.Account{
		{AccountId: 1, Name: "dollar cash", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "USD", Balance: 10000},
		{AccountId: 2, Name: "yuan cash", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "CNY", Balance: 5000},
	}
	fx := []marketquotes.FXRate{{Base: "USD", Quote: "CNY", Rate: "7", Date: "2026-01-01", Source: marketquotes.SourceECB, ReceivedAt: 1767225600, State: marketquotes.StateStale}}
	stale := buildWealth(result, cash, nil, fx)
	require.Empty(t, stale.Positions)
	require.Equal(t, 1, stale.StalePrices, "a cash-only user must see stale conversion rates")
	requireMoney(t, "750", stale.NetAssets)
	requireMoney(t, "700", stale.CashAccounts[0].Value)
	require.NotNil(t, stale.CashAccounts[0].FX)
	require.Equal(t, fx[0], *stale.CashAccounts[0].FX, "cash valuation must retain source date and retrieval time")
	fx[0].State = marketquotes.StateDelayed
	fresh := buildWealth(result, cash, nil, fx)
	require.Zero(t, fresh.StalePrices)
	requireMoney(t, "750", fresh.NetAssets)
	missing := buildWealth(result, cash, nil, nil)
	require.Nil(t, missing.NetAssets)
	require.Equal(t, 1, missing.MissingPrices)
	require.Equal(t, "50", missing.ValuedAssets)
	fx[0].State = marketquotes.StateStale
	cash[0].Balance = 0
	empty := buildWealth(result, cash, nil, fx)
	require.Zero(t, empty.StalePrices, "zero foreign cash has no stale converted exposure")
}
