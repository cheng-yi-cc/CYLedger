package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

func dcaFixture(t *testing.T, balance string) (*investmentDBFixture, models.CryptoDCASaveRequest, time.Time) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.ID(f.portfolioID).Cols("kind").Update(&models.PortfolioAccount{Kind: "EXCHANGE"})
	require.NoError(t, err)
	now := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour).Add(12 * time.Hour)
	cost := "700"
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: "crypto:tether", Quantity: balance, Amount: "0", Fee: "0", Cost: &cost, OccurredAt: now.AddDate(0, 0, -5).Unix()}}, "dca-opening-stable")
	req := models.CryptoDCASaveRequest{RequestKey: "dca-plan-fixture", AccountId: f.portfolioID, InstrumentId: "crypto:bitcoin", PaymentInstrumentId: "crypto:tether", Amount: "10", DailyTime: "08:00", StartDate: now.AddDate(0, 0, -1).Format("2006-01-02"), TimeZone: "UTC"}
	return f, req, now
}

func dcaFixtureQuote(_ context.Context, _, _ string, at int64) (*marketquotes.CryptoHistoricalQuote, error) {
	return &marketquotes.CryptoHistoricalQuote{PaymentPrice: "0.999", TargetPrice: "50000", PriceTime: at, Source: "fixture minute", FXRate: "7", FXDate: time.Unix(at, 0).UTC().AddDate(0, 0, -1).Format("2006-01-02"), FXSource: "fixture FX", ReceivedAt: at + 60}, nil
}

func TestCryptoDCAAtomicCatchupCostAndPermanentDeduplication(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	p, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	p2, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	require.Equal(t, p.Id, p2.Id)
	req.Amount = "11"
	_, err = f.s.SaveCryptoDCA(nil, f.uid, req)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	state, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Equal(t, 2, state.Created)
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, events, 3)
	replay, err := replayInvestments(events)
	require.NoError(t, err)
	for _, pos := range replay.Positions {
		if pos.InstrumentID == "crypto:bitcoin" {
			require.Equal(t, "0.0003996", pos.Quantity)
			requireMoney(t, "139.86", pos.Cost)
		} else if pos.InstrumentID == "crypto:tether" {
			require.Equal(t, "80", pos.Quantity)
		}
	}
	var purchase InvestmentEvent
	for _, e := range events {
		if e.DCA != nil {
			purchase = e
			require.Equal(t, e.OccurredAt, e.DCA.PriceTime)
			require.Equal(t, "6.993", e.ExchangeRate)
		}
	}
	_, err = f.s.Mutate(nil, f.uid, purchase, "", "void", false)
	require.NoError(t, err)
	state, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Zero(t, state.Created)
	require.Equal(t, int64(2), f.count(&models.CryptoDCADay{}))
	// Restored persisted facts still suffice; no in-memory seen flag is used.
	f.s.dcaSyncLocks = [64]sync.Mutex{}
	state, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Zero(t, state.Created)
}

func TestCryptoDCAMissingHistoryRemainsPendingAndDoesNotBlockLaterDates(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	_, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	state, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, func(ctx context.Context, pay, target string, at int64) (*marketquotes.CryptoHistoricalQuote, error) {
		if time.Unix(at, 0).Format("2006-01-02") == req.StartDate {
			return nil, errors.New("unavailable")
		}
		return dcaFixtureQuote(ctx, pay, target, at)
	})
	require.NoError(t, err)
	require.Equal(t, 1, state.Created)
	require.True(t, state.Plans[0].Enabled)
	pending := 0
	for _, day := range state.Days {
		if day.Status == "pending" {
			pending++
		}
	}
	require.Equal(t, 1, pending)
	_, err = f.engine.Where("status=?", "pending").Cols("last_attempt").Update(&models.CryptoDCADay{LastAttempt: 0})
	require.NoError(t, err)
	state, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Equal(t, 1, state.Created)
}

func TestCryptoDCAInsufficientFundsPausesWithoutPurchaseAndResumeSkipsGap(t *testing.T) {
	f, req, now := dcaFixture(t, "9")
	p, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	state, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, func(context.Context, string, string, int64) (*marketquotes.CryptoHistoricalQuote, error) {
		t.Fatal("must check funds before requesting public prices")
		return nil, nil
	})
	require.NoError(t, err)
	require.Zero(t, state.Created)
	require.False(t, state.Plans[0].Enabled)
	require.Equal(t, cryptoDCAPausedBalance, state.Plans[0].Status)
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	require.True(t, state.Alerts[0].Paused)
	_, err = f.s.SetCryptoDCAEnabled(nil, f.uid, p.Id, state.Plans[0].Revision, true)
	require.NoError(t, err)
	state, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Zero(t, state.Created)
	require.Greater(t, state.Plans[0].NextDate, now.Format("2006-01-02"))
}

func TestCryptoDCAThreeDayWarningAggregatesSharedStablecoin(t *testing.T) {
	f, req, now := dcaFixture(t, "80")
	req.StartDate = cryptoDCANextDate(now.Format("2006-01-02"))
	_, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	req.RequestKey = "dca-second-plan"
	req.InstrumentId = "crypto:ethereum"
	req.Amount = "20"
	_, err = f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	state, err := f.s.CryptoDCAState(nil, f.uid, now)
	require.NoError(t, err)
	require.Len(t, state.Alerts, 1)
	require.Equal(t, "90", state.Alerts[0].Required)
	require.Equal(t, "80", state.Alerts[0].Balance)
}

func TestCryptoDCAExecutionRollsBackDayAndHoldingTogether(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	req.StartDate = now.Format("2006-01-02")
	_, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	table, err := f.engine.TableInfo(&models.CryptoDCADay{})
	require.NoError(t, err)
	_, err = f.engine.Exec(fmt.Sprintf("CREATE TRIGGER fail_dca_day BEFORE UPDATE ON %s WHEN NEW.status='done' BEGIN SELECT RAISE(ABORT, 'fixture failure'); END", table.Name))
	require.NoError(t, err)
	state, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Zero(t, state.Created)
	require.Equal(t, int64(1), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, "pending", state.Days[0].Status)
	_, err = f.engine.Exec("DROP TRIGGER fail_dca_day")
	require.NoError(t, err)
	_, err = f.engine.Where("uid=?", f.uid).Cols("last_attempt").Update(&models.CryptoDCADay{})
	require.NoError(t, err)
	state, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Equal(t, 1, state.Created)
}

func TestCryptoDCAConcurrentExecutionAndUserIsolation(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	req.StartDate = now.Format("2006-01-02")
	p, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	var wg sync.WaitGroup
	failures := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, int64(1), f.count(&models.CryptoDCADay{}))
	_, err = f.s.SaveCryptoDCA(nil, f.uid+1, req)
	require.Error(t, err)
	_, err = f.s.SetCryptoDCAEnabled(nil, f.uid+1, p.Id, p.Revision, false)
	require.Error(t, err)
	state, err := f.s.CryptoDCAState(nil, f.uid+1, now)
	require.NoError(t, err)
	require.Empty(t, state.Plans)
	require.Empty(t, state.Days)
}

func TestCryptoDCAScheduleDSTAndInvalidInput(t *testing.T) {
	p := models.CryptoDCAPlan{TimeZone: "America/New_York", DailyTime: "02:30"}
	at, err := cryptoDCAScheduled(p, "2026-03-08")
	require.NoError(t, err)
	require.Equal(t, "2026-03-08 03:00", at.Format("2006-01-02 15:04"))
	p.DailyTime = "01:30"
	at, err = cryptoDCAScheduled(p, "2026-11-01")
	require.NoError(t, err)
	require.Equal(t, "2026-11-01 01:30", at.Format("2006-01-02 15:04"))
	f, req, _ := dcaFixture(t, "100")
	for _, value := range []string{"0", "-1", "1e5", "0.0000000000000000001"} {
		req.Amount = value
		_, err = f.s.SaveCryptoDCA(nil, f.uid, req)
		require.Error(t, err)
	}
}

func TestCryptoDCABackfillDoesNotUseLaterDeposits(t *testing.T) {
	f, req, now := dcaFixture(t, "9")
	_, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: "crypto:tether", Quantity: "100", Amount: "0", Fee: "0", OccurredAt: now.Unix()}}, "dca-later-funding")
	state, err := f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	require.Zero(t, state.Created)
	require.False(t, state.Plans[0].Enabled)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
}

func TestCryptoDCARevisionAndUntrustedSourceProtection(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	p, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	_, err = f.s.syncCryptoDCAAt(nil, f.uid, true, now, dcaFixtureQuote)
	require.NoError(t, err)
	before, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	req.Id, req.Revision, req.Amount = p.Id, p.Revision, "12"
	updated, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	require.Greater(t, updated.NextDate, now.Format("2006-01-02"))
	_, err = f.s.SaveCryptoDCA(nil, f.uid, req)
	require.ErrorIs(t, err, ErrInvestmentConflict)
	after, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, before, after)
	for _, event := range before {
		if event.DCA != nil {
			_, err = f.s.Mutate(nil, f.uid, event, "dca-forged-public-source", "create", false)
			require.Error(t, err)
			event.Note = "用户核对后修订"
			event.DCA = nil
			result, err := f.s.Mutate(nil, f.uid, event, "", "revise", false)
			require.NoError(t, err)
			require.NotNil(t, result.Event.DCA)
			break
		}
	}
}

func TestCryptoDCAAccountDeletionCancelsPendingWithPreviewRevision(t *testing.T) {
	f, req, now := dcaFixture(t, "100")
	p, err := f.s.SaveCryptoDCA(nil, f.uid, req)
	require.NoError(t, err)
	require.NoError(t, f.s.prepareCryptoDCADays(nil, f.uid, now))
	input := AccountDeletionInput{ID: f.portfolioID, Kind: "portfolio", DeleteRelated: true}
	preview, err := f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	require.Equal(t, 1, preview.CryptoDCAPlanCount)
	input.Token = preview.Token
	_, err = f.s.SetCryptoDCAEnabled(nil, f.uid, p.Id, p.Revision, false)
	require.NoError(t, err)
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.Error(t, err)
	preview, err = f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	input.Token = preview.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	deletedPlan := new(models.CryptoDCAPlan)
	has, err := f.engine.ID(p.Id).Get(deletedPlan)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, deletedPlan.Enabled)
	require.Equal(t, "账户已删除，定投已停止", deletedPlan.Status)
	pending, err := f.engine.Where("uid=? AND status=?", f.uid, "pending").Count(&models.CryptoDCADay{})
	require.NoError(t, err)
	require.Zero(t, pending)
}
