package services

import (
	"context"
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHoldingSetupAtomicRetryAndValuationPreferences(t *testing.T) {
	f := newInvestmentDBFixture(t)
	cost := "20"
	input := HoldingSetupInput{Profile: models.InvestmentHoldingProfile{Group: "基金", ProfitOffset: "5"}, Instrument: models.InvestmentInstrument{Name: "测试基金", Symbol: "FIXTURE", Type: "FUND"}, Quantity: "10", Cost: &cost, Price: "3", OccurredAt: f.at}
	p, err := f.s.SetupHolding(nil, f.uid, input, "holding-setup-1")
	require.NoError(t, err)
	again, err := f.s.SetupHolding(nil, f.uid, input, "holding-setup-1")
	require.NoError(t, err)
	require.Equal(t, p.Id, again.Id)
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(2000000), f.balance())
	requireMoney(t, "20030", f.summary(false).NetAssets)
	changed := input
	changed.Quantity = "11"
	_, err = f.s.SetupHolding(nil, f.uid, changed, "holding-setup-1")
	require.ErrorIs(t, err, ErrInvestmentConflict)
	p.ExcludeProfit = true
	updated, err := f.s.SaveHoldingProfile(nil, f.uid, *p)
	require.NoError(t, err)
	requireMoney(t, "20020", f.summary(false).NetAssets)
	_, err = f.s.SaveHoldingProfile(nil, f.uid, *p)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	updated.ExcludeFromTotal = true
	_, err = f.s.SaveHoldingProfile(nil, f.uid, *updated)
	require.NoError(t, err)
	requireMoney(t, "20000", f.summary(false).NetAssets)
	bad := input
	bad.Profile.AccountId = "missing-account"
	_, err = f.s.SetupHolding(nil, f.uid, bad, "holding-setup-bad")
	require.Error(t, err)
	require.Equal(t, int64(1), f.count(&models.InvestmentInstrument{}))
	require.Equal(t, int64(1), f.count(&models.InvestmentHoldingProfile{}))
}

func TestHoldingSetupPurchaseAtomicSettlementAndRetry(t *testing.T) {
	f := newInvestmentDBFixture(t)
	input := HoldingSetupInput{AccountName: "测试证券账户", Instrument: models.InvestmentInstrument{Name: "手动基金", Symbol: "FUND1", Type: "FUND"}, Quantity: "10", Price: "12", OccurredAt: f.at,
		Purchase: &HoldingPurchaseInput{Amount: "100", Fee: "2", CashAccountId: fmt.Sprint(f.bankID), ExchangeRate: "1"}}
	p, err := f.s.SetupHolding(nil, f.uid, input, "purchase-setup-1")
	require.NoError(t, err)
	require.Equal(t, int64(1989800), f.balance())
	positions := f.summary(true).Positions
	require.Len(t, positions, 1)
	requireMoney(t, "102", positions[0].Cost)
	require.Equal(t, "10", positions[0].Quantity)
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, investments.Buy, events[0].Type)
	account := new(models.PortfolioAccount)
	has, err := f.engine.ID(p.AccountId).Get(account)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, input.AccountName, account.Name)
	again, err := f.s.SetupHolding(nil, f.uid, input, "purchase-setup-1")
	require.NoError(t, err)
	require.Equal(t, p.Id, again.Id)
	require.Equal(t, int64(1989800), f.balance())
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	bad := input
	bad.Purchase = &HoldingPurchaseInput{Amount: "30000", Fee: "0", CashAccountId: fmt.Sprint(f.bankID), ExchangeRate: "1"}
	_, err = f.s.SetupHolding(nil, f.uid, bad, "purchase-setup-bad")
	require.Error(t, err)
	require.Equal(t, int64(1989800), f.balance())
	require.Equal(t, int64(1), f.count(&models.InvestmentInstrument{}))
	require.Equal(t, int64(1), f.count(&models.InvestmentHoldingProfile{}))
	require.Equal(t, int64(2), f.count(&models.PortfolioAccount{})) // 原测试钱包与新增证券账户。
	_, err = f.s.Mutate(nil, f.uid, events[0], "", "void", false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	require.Empty(t, f.summary(false).Positions)
}

func TestInvestmentReportAssetScopeIncludesHiddenAndExcludesOtherBooks(t *testing.T) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Book{Id: "scope-book-a", Uid: f.uid, Name: "账本甲"}, &models.Book{Id: "scope-book-b", Uid: f.uid, Name: "账本乙"})
	require.NoError(t, err)
	cost := "20"
	makeHolding := func(name string, profile models.InvestmentHoldingProfile) *models.InvestmentHoldingProfile {
		p, err := f.s.SetupHolding(nil, f.uid, HoldingSetupInput{Profile: profile, Instrument: models.InvestmentInstrument{Name: name, Symbol: name, Type: "OTHER"}, Quantity: "10", Cost: &cost, Price: "3", OccurredAt: f.at}, "scope-setup-"+name)
		require.NoError(t, err)
		return p
	}
	hidden := makeHolding("hidden", models.InvestmentHoldingProfile{Hidden: true, BookIds: []string{"scope-book-a"}})
	makeHolding("excluded", models.InvestmentHoldingProfile{ExcludeFromTotal: true, BookIds: []string{"scope-book-a"}})
	makeHolding("otherbook", models.InvestmentHoldingProfile{BookIds: []string{"scope-book-b"}})
	f.summary(true)
	report, err := f.s.InvestmentReport(nil, f.uid, "", "", InvestmentReportScope{BookIds: []string{"scope-book-a"}})
	require.NoError(t, err)
	require.Len(t, report.Items, 1)
	require.Equal(t, hidden.AccountId, report.Items[0].Event.AccountID)
	require.NotEmpty(t, report.History)
	requireMoney(t, "30", report.History[len(report.History)-1].Value)
	requireMoney(t, "10", report.History[len(report.History)-1].Profit)
	prefs := defaultAssetPreferences()
	prefs.Rules["portfolio:"+hidden.AccountId] = models.AssetAccountRule{Hidden: true, DisabledBooks: []string{"scope-book-a"}}
	_, err = f.s.SaveAssetPreferences(nil, f.uid, *prefs)
	require.NoError(t, err)
	report, err = f.s.InvestmentReport(nil, f.uid, hidden.AccountId, "", InvestmentReportScope{BookIds: []string{"scope-book-a", "scope-book-b"}})
	require.NoError(t, err)
	require.Empty(t, report.Items) // 不能用账户允许的乙账本绕过持仓仅在甲生效的限制。
	report, err = f.s.InvestmentReport(nil, f.uid, hidden.AccountId, "", InvestmentReportScope{})
	require.NoError(t, err)
	require.Len(t, report.Items, 1) // 全部账本汇总仍包括隐藏账户。
}

func TestHoldingEditCalibrationConflictAndSnapshotReplay(t *testing.T) {
	f := newInvestmentDBFixture(t)
	cost := "20"
	p, err := f.s.SetupHolding(nil, f.uid, HoldingSetupInput{Instrument: models.InvestmentInstrument{Name: "测试理财", Symbol: "TEST", Type: "OTHER"}, Quantity: "10", Cost: &cost, Price: "3", OccurredAt: f.at}, "calibrate-setup")
	require.NoError(t, err)
	before := f.summary(true)
	requireMoney(t, "20030", before.NetAssets)
	newCost := "36"
	input := HoldingSetupInput{Profile: *p, Quantity: "12", Cost: &newCost, ExpectedQuantity: "10", ExpectedCost: &cost, OccurredAt: f.at + 1}
	updated, err := f.s.UpdateHolding(nil, f.uid, input, "calibrate-once")
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, int64(2000000), f.balance())
	requireMoney(t, "20036", f.summary(false).NetAssets)
	_, err = f.s.UpdateHolding(nil, f.uid, input, "calibrate-again")
	require.ErrorIs(t, err, ErrInvestmentConflict)
	f.quote("9000") // unrelated current price must not alter the saved valuation.
	history, err := f.s.History(nil, f.uid)
	require.NoError(t, err)
	requireMoney(t, "20036", history[0].NetAssets)
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	var adjustment InvestmentEvent
	for _, e := range events {
		if e.Type == investments.Adjust {
			adjustment = e
		}
	}
	_, err = f.s.Mutate(nil, f.uid, adjustment, "", "void", false)
	require.NoError(t, err)
	requireMoney(t, "20030", f.summary(false).NetAssets)
}

func TestHoldingProfileEditPreservesFractionalReplayCost(t *testing.T) {
	f := newInvestmentDBFixture(t)
	cost := "1"
	p, err := f.s.SetupHolding(nil, f.uid, HoldingSetupInput{Instrument: models.InvestmentInstrument{Name: "分数成本", Symbol: "FRACTION", Type: "OTHER"}, Quantity: "3", Cost: &cost, Price: "1", OccurredAt: f.at}, "fraction-opening")
	require.NoError(t, err)
	_, err = f.s.Mutate(nil, f.uid, InvestmentEvent{Event: investments.Event{Type: investments.Sell, AccountID: p.AccountId, InstrumentID: p.InstrumentId, Quantity: "1", Amount: "1", Fee: "0", ExchangeRate: "1", OccurredAt: f.at + 1}, CashAccountID: fmt.Sprint(f.bankID)}, "fraction-sell", "create", false)
	require.NoError(t, err)
	positions := f.summary(false).Positions
	require.Len(t, positions, 1)
	old := positions[0]
	p.Name = "仅改显示名称"
	_, err = f.s.UpdateHolding(nil, f.uid, HoldingSetupInput{Profile: *p, Quantity: old.Quantity, Cost: old.Cost, ExpectedQuantity: old.Quantity, ExpectedCost: old.Cost, OccurredAt: f.at + 2}, "fraction-edit")
	require.NoError(t, err)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, old.Cost, f.summary(false).Positions[0].Cost)
}

func fixtureFund(t *testing.T, f *investmentDBFixture) *models.InvestmentInstrument {
	t.Helper()
	a, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Name: "虚构基金", Symbol: "000001", Type: "FUND"})
	require.NoError(t, err)
	a.Provider = "eastmoney"
	a.ProviderID = "000001"
	a.Market = "CN_FUND"
	a.Currency = "CNY"
	_, err = f.engine.ID(a.Id).AllCols().Update(a)
	require.NoError(t, err)
	return a
}

func TestFundPlanBackfillIdempotencyAndVoidedTombstone(t *testing.T) {
	f := newInvestmentDBFixture(t)
	a := fixtureFund(t, f)
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -2).Format("2006-01-02")
	_, errBalance := f.engine.Where("uid=? AND type=?", f.uid, models.TRANSACTION_DB_TYPE_MODIFY_BALANCE).Cols("transaction_time").Update(&models.Transaction{TransactionTime: now.AddDate(0, 0, -4).Unix() * 1000})
	require.NoError(t, errBalance)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ErrCode":0,"Data":{"FundType":"001","LSJZList":[{"FSRQ":%q,"DWJZ":"2"}]}}`, r.URL.Query().Get("startDate"))
	}))
	defer server.Close()
	marketquotes.Default = marketquotes.New(marketquotes.Config{FundNAVURL: server.URL})
	_, fetchError := marketquotes.Default.FundHistory(context.Background(), "000001", start, now.Format("2006-01-02"))
	require.NoError(t, fetchError)
	p, err := f.s.SaveInvestmentPlan(nil, f.uid, models.InvestmentPlan{AccountId: f.portfolioID, InstrumentId: a.Id, CashAccountId: fmt.Sprint(f.bankID), Amount: "100", FeePercent: "1", Cycle: "daily", StartDate: start, EndDate: start, Time: "09:00", TimeZone: "UTC"})
	require.NoError(t, err)
	result, err := f.s.syncInvestmentPlansAt(nil, f.uid, true, now)
	require.NoError(t, err)
	ordersOnAttempt, _ := f.s.InvestmentOrders(nil, f.uid)
	require.Equal(t, 1, result.Created, "%+v", ordersOnAttempt)
	require.Equal(t, int64(1990000), f.balance())
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "49.5", events[0].Quantity)
	require.Equal(t, "99.01", events[0].Amount)
	require.Equal(t, "0.99", events[0].Fee)
	require.Equal(t, start, events[0].Fund.PriceDate)
	result, err = f.s.syncInvestmentPlansAt(nil, f.uid, true, now)
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Equal(t, int64(1990000), f.balance())
	_, err = f.s.Mutate(nil, f.uid, events[0], "", "void", false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	// 即使修订计划从原日期重新生成，撤销过的本期也不能再次扣款。
	p.Cycle = "weekly"
	_, err = f.s.SaveInvestmentPlan(nil, f.uid, *p)
	require.NoError(t, err)
	result, err = f.s.syncInvestmentPlansAt(nil, f.uid, true, now)
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Equal(t, int64(2000000), f.balance())
}

func TestFundConfirmationInsufficientCashAndCancel(t *testing.T) {
	f := newInvestmentDBFixture(t)
	a := fixtureFund(t, f)
	date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	order := models.InvestmentOrder{AccountId: f.portfolioID, InstrumentId: a.Id, CashAccountId: fmt.Sprint(f.bankID), Type: "BUY", Amount: "30000", Fee: "0", FeePercent: "0", TradeDate: date, Time: "09:00", TimeZone: "UTC"}
	o, err := f.s.SaveInvestmentOrder(nil, f.uid, order, "fund-order-test")
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	_, err = f.s.ConfirmInvestmentOrder(nil, f.uid, InvestmentOrderConfirmation{Id: o.Id, Version: o.Version, Price: "2", Date: date})
	require.Error(t, err)
	require.Equal(t, int64(0), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(2000000), f.balance())
	require.NoError(t, f.s.CancelInvestmentOrder(nil, f.uid, o.Id, o.Version))
	_, err = f.s.ConfirmInvestmentOrder(nil, f.uid, InvestmentOrderConfirmation{Id: o.Id, Version: o.Version, Price: "2", Date: date})
	require.ErrorIs(t, err, ErrInvestmentConflict)
}

func TestInvestmentPlanCalendarMonthEnd(t *testing.T) {
	require.Equal(t, "2024-02-29", nextInvestmentDate("2024-01-31", "monthly", "2024-01-31"))
	require.Equal(t, "2024-03-31", nextInvestmentDate("2024-02-29", "monthly", "2024-01-31"))
}

func TestAccountDeletionStopsPlansAndPendingOrders(t *testing.T) {
	f := newInvestmentDBFixture(t)
	a := fixtureFund(t, f)
	date := time.Now().UTC().Format("2006-01-02")
	p, err := f.s.SaveInvestmentPlan(nil, f.uid, models.InvestmentPlan{AccountId: f.portfolioID, InstrumentId: a.Id, CashAccountId: fmt.Sprint(f.bankID), Amount: "100", FeePercent: "0", Cycle: "monthly", StartDate: date, Time: "09:00", TimeZone: "UTC"})
	require.NoError(t, err)
	o, err := f.s.SaveInvestmentOrder(nil, f.uid, models.InvestmentOrder{AccountId: f.portfolioID, InstrumentId: a.Id, CashAccountId: fmt.Sprint(f.bankID), Type: "BUY", Amount: "100", TradeDate: date, Time: "09:00", TimeZone: "UTC"}, "pending-before-delete")
	require.NoError(t, err)
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio", DeleteRelated: true}
	preview, err := f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	require.Equal(t, 2, preview.DueCount)
	p.Paused = true
	_, err = f.s.SaveInvestmentPlan(nil, f.uid, *p)
	require.NoError(t, err)
	input.Token = preview.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.Error(t, err)
	preview, err = f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	input.Token = preview.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	plans, err := f.s.InvestmentPlans(nil, f.uid)
	require.NoError(t, err)
	require.Empty(t, plans)
	orders, err := f.s.InvestmentOrders(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, o.Id, orders[0].Id)
	require.Equal(t, "cancelled", orders[0].Status)
}
