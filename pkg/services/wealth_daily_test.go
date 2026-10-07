package services

import (
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/investments"
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestDailyWealthUsesAccountingMidnightAndPreservesCost(t *testing.T) {
	f := newInvestmentDBFixture(t)
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	at := time.Date(2026, 10, 6, 16, 0, 0, 0, time.UTC).Unix()
	marketquotes.Default = marketquotes.New(marketquotes.Config{Now: func() time.Time { return now }})
	marketquotes.Default.RestoreDayQuotes([]marketquotes.DayQuote{{InstrumentID: "crypto:bitcoin", At: at, Price: "100", Currency: "USD", Source: marketquotes.SourceCoinbaseREST + " 零点前分钟收盘价", SourceTime: at - 60, FXRate: "7", FXDate: "2026-10-06", FXSource: marketquotes.SourceECB}})
	_, err := f.engine.Insert(&models.InvestmentSettings{Uid: f.uid, TimeZone: "Asia/Shanghai", BaseCurrency: "CNY"})
	require.NoError(t, err)
	cost := "80"
	events := []InvestmentEvent{
		{Event: investments.Event{ID: "old", Type: investments.Opening, AccountID: "a", InstrumentID: "crypto:bitcoin", Quantity: "1", Cost: &cost, OccurredAt: at - 1}},
		{Event: investments.Event{ID: "transfer", Type: investments.Transfer, AccountID: "a", ToAccountID: "b", InstrumentID: "crypto:bitcoin", Quantity: "0.5", OccurredAt: at}},
		{Event: investments.Event{ID: "sale", Type: investments.Sell, AccountID: "b", InstrumentID: "crypto:bitcoin", Quantity: "0.25", Amount: "200", Fee: "1", ExchangeRate: "1", OccurredAt: at + 1}},
	}
	result, err := replayInvestments(events)
	require.NoError(t, err)
	quotes := []InvestmentValuationQuote{{Quote: marketquotes.Quote{InstrumentID: "crypto:bitcoin", Price: "120", Currency: "USD", State: marketquotes.StateDelayed}, FXRate: "7", FXState: marketquotes.StateDelayed}}
	out := buildWealth(result, nil, quotes, nil)
	sess := f.s.UserDataDB(f.uid).NewSession(nil)
	defer sess.Close()
	require.NoError(t, enrichDailyWealth(sess, f.uid, events, quotes, nil, out, now))
	requireMoney(t, "70", out.Positions[0].DailyPNL)
	requireMoney(t, "59", out.Positions[1].DailyPNL)
	requireMoney(t, "40", out.Positions[0].Cost)
	requireMoney(t, "20", out.Positions[1].Cost)
	require.Equal(t, at, out.Positions[0].DayStart)
	require.Equal(t, "80", *events[0].Cost)
}
