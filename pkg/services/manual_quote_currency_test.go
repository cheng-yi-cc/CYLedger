package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestInvestmentManualUSDQuoteUsesCurrentFXAndHistoricalSnapshots(t *testing.T) {
	f := newInvestmentDBFixture(t)
	asset, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Type: "CRYPTO", Name: "USD24 fixture", Symbol: "USD24"})
	require.NoError(t, err)
	opening := f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: asset.Id, Quantity: "12.34", OccurredAt: f.at + 1}}, "manual-usd-opening")
	before, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	cashBefore := f.balance()
	_, err = f.s.ManualQuote(nil, f.uid, asset.Id, "1.25", time.Now().Unix(), "USD")
	require.NoError(t, err)
	unpriced := f.summary(false)
	require.Nil(t, unpriced.Positions[0].MarketValue)
	require.Nil(t, unpriced.NetAssets)
	require.Equal(t, "USD", unpriced.Positions[0].Quote.Currency)
	require.Equal(t, "1.25", unpriced.Positions[0].Quote.Price)
	require.Empty(t, unpriced.Positions[0].Quote.FXRate, "missing USD FX must not become 1 or zero")

	var stored models.InvestmentQuote
	has, err := f.engine.Where("uid=? AND instrument_id=?", f.uid, asset.Id).Get(&stored)
	require.NoError(t, err)
	require.True(t, has)
	originalPayload := stored.Payload
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	marketquotes.Default.Restore(nil, []marketquotes.FXRate{{Base: "USD", Quote: "CNY", Rate: "7.2", Date: yesterday, Source: marketquotes.SourceECB, ReceivedAt: time.Now().Unix()}})
	first := f.summary(true)
	requireMoney(t, "111.06", first.Positions[0].MarketValue)
	require.Equal(t, yesterday, first.Positions[0].Quote.FXDate)
	require.Equal(t, marketquotes.SourceECB, first.Positions[0].Quote.FXSource)
	require.Equal(t, marketquotes.StateStale, first.Positions[0].Quote.FXState)
	require.Equal(t, 1, first.StalePrices)

	marketquotes.Default.Restore(nil, []marketquotes.FXRate{{Base: "USD", Quote: "CNY", Rate: "7.3", Date: time.Now().UTC().Format("2006-01-02"), Source: marketquotes.SourceECB, ReceivedAt: time.Now().Unix()}})
	current := f.summary(false)
	requireMoney(t, "112.6025", current.Positions[0].MarketValue)
	require.Equal(t, "1.25", current.Positions[0].Quote.Price)
	require.Equal(t, first.Positions[0].Position, current.Positions[0].Position)
	after, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, cashBefore, f.balance())
	stored = models.InvestmentQuote{}
	_, err = f.engine.Where("uid=? AND instrument_id=?", f.uid, asset.Id).Get(&stored)
	require.NoError(t, err)
	require.Equal(t, originalPayload, stored.Payload, "a daily FX change must not rewrite the manual unit price")

	// Historical replay must retain the snapshot's 7.2 rate after current FX changes.
	revision := opening.Event
	revision.Quantity = "24.68"
	_, err = f.s.Mutate(nil, f.uid, revision, "", "revise", false)
	require.NoError(t, err)
	history, err := f.s.History(nil, f.uid)
	require.NoError(t, err)
	require.Len(t, history, 1)
	var observation wealthObservation
	require.NoError(t, json.Unmarshal([]byte(history[0].Payload), &observation))
	requireMoney(t, "222.12", observation.Summary.Positions[0].MarketValue)
	require.Equal(t, "7.2", observation.Summary.Positions[0].Quote.FXRate)
	requireMoney(t, "225.205", f.summary(false).Positions[0].MarketValue)
}

func TestInvestmentManualCurrencyValidationAndLegacyCNY(t *testing.T) {
	f := newInvestmentDBFixture(t)
	asset, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Type: "CRYPTO", Name: "Manual fixture", Symbol: "FIX"})
	require.NoError(t, err)
	f.create(InvestmentEvent{Event: investments.Event{Type: investments.Opening, AccountID: f.portfolioID, InstrumentID: asset.Id, Quantity: "2", OccurredAt: f.at + 1}}, "currency-check-opening")
	_, err = f.s.ManualQuote(nil, f.uid, asset.Id, "7.25", time.Now().Unix(), "")
	require.NoError(t, err)
	legacy := f.summary(false)
	require.Equal(t, "CNY", legacy.Positions[0].Quote.Currency)
	require.Equal(t, "1", legacy.Positions[0].Quote.FXRate)
	requireMoney(t, "14.5", legacy.Positions[0].MarketValue)
	for _, currency := range []string{"EUR", "usd", " USD", "USDT"} {
		_, err = f.s.ManualQuote(nil, f.uid, asset.Id, "1", time.Now().Unix(), currency)
		require.Error(t, err)
	}
	_, err = f.s.ManualQuote(nil, f.uid+1, asset.Id, "1", time.Now().Unix(), "USD")
	require.Error(t, err)
	requireMoney(t, "14.5", f.summary(false).Positions[0].MarketValue)
	stock, err := f.s.CreateInstrument(nil, f.uid, models.InvestmentInstrument{Type: "STOCK", Name: "Stock fixture", Symbol: "STK"})
	require.NoError(t, err)
	_, err = f.s.ManualQuote(nil, f.uid, stock.Id, "1", time.Now().Unix(), "USD")
	require.Error(t, err)
	_, err = f.s.ManualQuote(nil, f.uid, stock.Id, "7", time.Now().Unix(), "CNY")
	require.NoError(t, err)
	_, err = f.s.ManualQuote(nil, f.uid, asset.Id, "8", time.Now().Unix(), "CNY")
	require.NoError(t, err)
	requireMoney(t, "16", f.summary(false).Positions[0].MarketValue)
}
