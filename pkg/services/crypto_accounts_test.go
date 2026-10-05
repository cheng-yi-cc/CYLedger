package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func conversionFixture(now int64) []InvestmentValuationQuote {
	return []InvestmentValuationQuote{
		{Quote: marketquotes.Quote{InstrumentID: "crypto:tether", Price: "0.999", Currency: "USD", SourceTime: now, ReceivedAt: now, State: marketquotes.StateDelayed, Source: marketquotes.SourceCoinbaseREST}, FXRate: "7", FXDate: "2026-10-02", FXSource: "fixture FX", FXState: "fresh"},
		{Quote: marketquotes.Quote{InstrumentID: "crypto:bitcoin", Price: "60000", Currency: "USD", SourceTime: now, ReceivedAt: now, State: marketquotes.StateLive, Source: marketquotes.SourceCoinbaseWS}, FXRate: "7", FXDate: "2026-10-02", FXSource: "fixture FX", FXState: "fresh"},
	}
}
func TestCryptoConversionUsesObservedStablecoinPriceAndDecimalAmounts(t *testing.T) {
	now := time.Now().Unix()
	quotes := conversionFixture(now)
	q, err := conversionFromQuotes(ConversionInput{ToInstrumentID: "crypto:tether", FromQuantity: "6993"}, quotes, now)
	require.NoError(t, err)
	require.Equal(t, "1000", q.ToQuantity)
	require.Equal(t, "6.993", q.ToPrice)
	btc, err := conversionFromQuotes(ConversionInput{FromInstrumentID: "crypto:tether", ToInstrumentID: "crypto:bitcoin", FromQuantity: "600"}, quotes, now)
	require.NoError(t, err)
	require.Equal(t, "0.00999", btc.ToQuantity)
	require.Equal(t, "6.993", btc.FromPrice)
	sale, err := conversionFromQuotes(ConversionInput{FromInstrumentID: "crypto:bitcoin", FromQuantity: "0.00999"}, quotes, now)
	require.NoError(t, err)
	require.Equal(t, "4195.8", sale.ToQuantity)
	for _, input := range []ConversionInput{{FromQuantity: "1e5", ToInstrumentID: "crypto:tether"}, {FromQuantity: "0", ToInstrumentID: "crypto:tether"}, {FromQuantity: "-1", ToInstrumentID: "crypto:tether"}, {FromQuantity: "1.001", ToInstrumentID: "crypto:tether"}, {FromQuantity: "1", FromInstrumentID: "crypto:tether", ToInstrumentID: "crypto:tether"}} {
		_, err = conversionFromQuotes(input, quotes, now)
		require.Error(t, err)
	}
	quotes[0].State = marketquotes.StateStale
	_, err = conversionFromQuotes(ConversionInput{ToInstrumentID: "crypto:tether", FromQuantity: "1"}, quotes, now)
	require.Error(t, err)
	quotes = conversionFixture(now)
	quotes[0].FXRate = ""
	_, err = conversionFromQuotes(ConversionInput{ToInstrumentID: "crypto:tether", FromQuantity: "1"}, quotes, now)
	require.Error(t, err)
}
func TestCryptoAccountSetupIsAtomicAndIdempotent(t *testing.T) {
	f := newInvestmentDBFixture(t)
	input := CryptoAccountInput{Name: "币安演示", Kind: "EXCHANGE", Platform: "binance", Holdings: []CryptoHoldingInput{{InstrumentID: "crypto:tether", Quantity: "100"}, {InstrumentID: "crypto:bitcoin", Quantity: "0.001"}, {InstrumentID: "crypto:ethereum", Quantity: "0"}}}
	before := f.balance()
	a, err := f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-setup-request")
	require.NoError(t, err)
	require.Len(t, a.Instruments, 3)
	again, err := f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-setup-request")
	require.NoError(t, err)
	require.Equal(t, a.Id, again.Id)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	require.Equal(t, before, f.balance())
	accounts, err := f.s.Accounts(nil, f.uid)
	require.NoError(t, err)
	for _, v := range accounts {
		if v.Id == a.Id {
			require.Equal(t, "binance", v.Platform)
			require.Equal(t, a.Instruments, v.Instruments)
		}
	}
	events, err := f.s.Events(nil, f.uid)
	require.NoError(t, err)
	for _, e := range events {
		require.Nil(t, e.Cost)
	}
	input.Name = "different"
	_, err = f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-setup-request")
	require.ErrorIs(t, err, ErrInvestmentConflict)
	input.Holdings[1].Quantity = "-1"
	_, err = f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-invalid-quantity")
	require.Error(t, err)
	require.Equal(t, int64(2), f.count(&models.PortfolioAccount{}))
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	input.Holdings[1].Quantity = "1"
	input.Holdings[1].InstrumentID = "custom:other-user"
	_, err = f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-invalid-instrument")
	require.Error(t, err)
	input.Platform = "metamask"
	_, err = f.s.CreateCryptoAccount(nil, f.uid, input, "crypto-invalid-platform")
	require.Error(t, err)
}
func TestCryptoConversionSettlementAndExpiredQuote(t *testing.T) {
	f := newInvestmentDBFixture(t)
	now := time.Now().Unix()
	quotes := conversionFixture(now)
	quote, err := conversionFromQuotes(ConversionInput{ToInstrumentID: "crypto:tether", FromQuantity: "6993"}, quotes, now)
	require.NoError(t, err)
	event := f.event("BUY", quote.ToQuantity, "6993", "0", 0)
	event.InstrumentID = "crypto:tether"
	event.ExchangeRate = "1"
	event.OccurredAt = now
	event.Conversion = quote
	saved := f.create(event, "crypto-rmb-usdt")
	require.Equal(t, int64(1300700), f.balance())
	retry, err := f.s.Mutate(nil, f.uid, event, "crypto-rmb-usdt", "create", false)
	require.NoError(t, err)
	require.Equal(t, saved.Event.ID, retry.Event.ID)
	require.Equal(t, int64(1300700), f.balance())
	swap, err := conversionFromQuotes(ConversionInput{FromInstrumentID: "crypto:tether", ToInstrumentID: "crypto:bitcoin", FromQuantity: "600"}, quotes, now)
	require.NoError(t, err)
	buy := f.event("BUY", swap.ToQuantity, "600", "0", 0)
	buy.OccurredAt = now
	buy.CashAccountID = ""
	buy.SettlementAccountID = f.portfolioID
	buy.SettlementInstrumentID = "crypto:tether"
	buy.ExchangeRate = swap.FromPrice
	buy.Conversion = swap
	result := f.create(buy, "crypto-usdt-btc")
	for _, p := range result.Positions {
		if p.InstrumentID == "crypto:tether" {
			require.Equal(t, "400", p.Quantity)
		}
		if p.InstrumentID == "crypto:bitcoin" {
			require.Equal(t, "0.00999", p.Quantity)
		}
	}
	require.Equal(t, int64(1300700), f.balance())
	buy.Amount = "900"
	_, err = f.s.Mutate(nil, f.uid, buy, "crypto-insufficient", "create", false)
	require.Error(t, err)
	require.Equal(t, int64(2), f.count(&models.InvestmentEventRecord{}))
	event.Conversion.ExpiresAt = now - 1
	_, err = f.s.Mutate(nil, f.uid, event, "crypto-expired", "create", false)
	require.Error(t, err)
	require.Equal(t, int64(1300700), f.balance())
}
