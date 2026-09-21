package investments

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func textPtr(v string) *string { return &v }

func event(id, kind, quantity, amount, fee string) Event {
	return Event{ID: id, Type: kind, AccountID: "exchange", InstrumentID: "btc", Quantity: quantity, Amount: amount, Fee: fee, ExchangeRate: "1", Version: 1}
}

func mustReplay(t *testing.T, events ...Event) *Result {
	t.Helper()
	r, err := Replay(events)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	return r
}

func findPosition(t *testing.T, r *Result, account, instrument string) Position {
	t.Helper()
	for _, p := range r.Positions {
		if p.AccountID == account && p.InstrumentID == instrument {
			return p
		}
	}
	t.Fatalf("missing position %s/%s", account, instrument)
	return Position{}
}

func assertDecimal(t *testing.T, name, got, want string) {
	t.Helper()
	actual, err := decimal.NewFromString(got)
	if err != nil || !actual.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func assertKnown(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is unknown, want %s", name, want)
	}
	assertDecimal(t, name, *got, want)
}

func TestBriefFixedAccountingFixture(t *testing.T) {
	buys := []Event{event("01", Buy, "0.01", "6000", "6"), event("02", Buy, "0.01", "7000", "7")}
	r := mustReplay(t, buys...)
	p := findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "quantity", p.Quantity, "0.02")
	assertKnown(t, "cost", p.Cost, "13013")
	assertKnown(t, "average cost", p.AverageCost, "650650")
	bank := decimal.RequireFromString("20000")
	for _, effect := range r.Effects {
		bank = bank.Add(decimal.RequireFromString(effect.CashDelta))
	}
	assertDecimal(t, "bank balance", bank.String(), "6987")
	marketValue := decimal.RequireFromString(p.Quantity).Mul(decimal.RequireFromString("750000"))
	assertDecimal(t, "market value", marketValue.String(), "15000")
	assertDecimal(t, "unrealized pnl", marketValue.Sub(decimal.RequireFromString(*p.Cost)).String(), "1987")
	assertDecimal(t, "net wealth", bank.Add(marketValue).String(), "21987")

	r = mustReplay(t, append(buys, event("03", Sell, "0.005", "4000", "4"))...)
	p = findPosition(t, r, "exchange", "btc")
	sell := r.Effects[2]
	assertDecimal(t, "net sell proceeds", sell.CashDelta, "3996")
	assertKnown(t, "released cost", sell.ReleasedCost, "3253.25")
	assertKnown(t, "realized pnl", sell.RealizedPNL, "742.75")
	assertKnown(t, "position realized pnl", p.RealizedPNL, "742.75")
	assertDecimal(t, "remaining quantity", p.Quantity, "0.015")
	assertKnown(t, "remaining cost", p.Cost, "9759.75")
	bank = bank.Add(decimal.RequireFromString(sell.CashDelta))
	assertDecimal(t, "bank after sell", bank.String(), "10983")
	marketValue = decimal.RequireFromString(p.Quantity).Mul(decimal.RequireFromString("800000"))
	unrealized := marketValue.Sub(decimal.RequireFromString(*p.Cost))
	assertDecimal(t, "remaining market value", marketValue.String(), "12000")
	assertDecimal(t, "remaining unrealized pnl", unrealized.String(), "2240.25")
	assertDecimal(t, "total investment pnl", unrealized.Add(decimal.RequireFromString(*r.RealizedPNL)).String(), "2983")
	assertDecimal(t, "wealth after sell", bank.Add(marketValue).String(), "22983")
}

func TestUnknownCostIsNeverZeroAndFullExitResetsOnlyHolding(t *testing.T) {
	opening := event("01", Opening, "2", "0", "0")
	opening.Cost = nil
	r := mustReplay(t, opening, event("02", Sell, "1", "100", "1"))
	p := findPosition(t, r, "exchange", "btc")
	if p.CostKnown || p.Cost != nil || p.AverageCost != nil || p.RealizedPNL != nil || r.RealizedPNL != nil {
		t.Fatal("unknown basis incorrectly produced known cost or profit")
	}
	if r.Effects[0].CashDelta != "0" || r.Effects[1].ReleasedCost != nil || r.Effects[1].RealizedPNL != nil {
		t.Fatal("opening must not move cash; unknown disposal must not invent cost/profit")
	}
	r = mustReplay(t, opening, event("02", Sell, "2", "200", "0"), event("03", Buy, "1", "150", "1"))
	p = findPosition(t, r, "exchange", "btc")
	assertKnown(t, "new holding cost", p.Cost, "151")
	if !p.CostKnown || p.RealizedPNL != nil || r.RealizedPNL != nil {
		t.Fatal("new basis should be known while lifetime realized profit remains unknown")
	}
}

func TestUnknownOpeningContaminatesMixedAverageBasis(t *testing.T) {
	r := mustReplay(t, event("01", Opening, "1", "0", "0"), event("02", Buy, "2", "300", "0"))
	p := findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "quantity", p.Quantity, "3")
	if p.CostKnown || p.Cost != nil || p.AverageCost != nil {
		t.Fatal("mixed known and unknown acquisition basis must remain unknown")
	}
}

func TestFullSellReleasesAllRoundingResidue(t *testing.T) {
	opening := event("01", Opening, "3", "0", "0")
	opening.Cost = textPtr("1")
	r := mustReplay(t, opening, event("02", Sell, "1", "1", "0"), event("03", Sell, "1", "1", "0"), event("04", Sell, "1", "1", "0"))
	p := findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "empty quantity", p.Quantity, "0")
	assertKnown(t, "empty cost", p.Cost, "0")
	assertKnown(t, "aggregate profit", r.RealizedPNL, "2")
	released := decimal.Zero
	for _, effect := range r.Effects {
		released = released.Add(decimal.RequireFromString(*effect.ReleasedCost))
	}
	assertDecimal(t, "all cost released", released.String(), "1")
	if p.AverageCost != nil {
		t.Fatal("empty position has no average unit cost")
	}
}

func TestReplayOrderIsStableAndDoesNotMutateInput(t *testing.T) {
	buy := event("a", Buy, "2", "10", "0")
	buy.OccurredAt = 10
	sell := event("b", Sell, "1", "8", "0")
	sell.OccurredAt = 10
	input := []Event{sell, buy}
	before := append([]Event(nil), input...)
	r := mustReplay(t, input...)
	if !reflect.DeepEqual(input, before) {
		t.Fatal("Replay mutated input order")
	}
	if r.Effects[0].EventID != "a" || r.Effects[1].EventID != "b" {
		t.Fatal("tie must use event ID")
	}
	buy.OccurredAt = 11
	if _, err := Replay([]Event{buy, sell}); err == nil {
		t.Fatal("timestamp must take precedence over ID")
	}
}

func TestHistoricalEditRejectsOffendingLaterEvent(t *testing.T) {
	buy := event("initial-buy", Buy, "2", "20", "0")
	buy.OccurredAt = 1
	sell := event("later-sale", Sell, "1.5", "30", "0")
	sell.OccurredAt = 2
	mustReplay(t, buy, sell)
	buy.Quantity = "1"
	r, err := Replay([]Event{buy, sell})
	var conflict *ReplayError
	if r != nil || !errors.As(err, &conflict) || conflict.EventID != "later-sale" || !strings.Contains(conflict.Message, "insufficient holding") {
		t.Fatalf("want later-sale conflict and no partial result, got result=%v, error=%v", r, err)
	}
	buy.Voided = true
	if _, err := Replay([]Event{buy, sell}); !errors.As(err, &conflict) || conflict.EventID != "later-sale" {
		t.Fatal("voiding a historical buy must also reject the later oversell")
	}
}

func TestHistoricalEditRecalculatesSubsequentDisposal(t *testing.T) {
	buy := event("01", Buy, "2", "200", "0")
	sell := event("02", Sell, "1", "200", "0")
	r := mustReplay(t, buy, sell)
	assertKnown(t, "original pnl", r.RealizedPNL, "100")
	buy.Amount = "300"
	r = mustReplay(t, buy, sell)
	assertKnown(t, "revised pnl", r.RealizedPNL, "50")
	assertKnown(t, "revised remaining cost", r.Positions[0].Cost, "150")
}

func TestTransferMovesHistoricalCostAndAccountsForNetworkFee(t *testing.T) {
	opening := event("01", Opening, "10", "0", "0")
	opening.Cost = textPtr("1000")
	transfer := event("02", Transfer, "3", "0", "0.1")
	transfer.ToAccountID = "wallet"
	transfer.ExchangeRate = "150"
	r := mustReplay(t, opening, transfer)
	source := findPosition(t, r, "exchange", "btc")
	target := findPosition(t, r, "wallet", "btc")
	assertDecimal(t, "source quantity", source.Quantity, "6.9")
	assertKnown(t, "source cost", source.Cost, "690")
	assertDecimal(t, "target quantity", target.Quantity, "3")
	assertKnown(t, "target cost", target.Cost, "300")
	assertKnown(t, "network fee market value", r.Effects[1].InvestmentFee, "15")
	assertKnown(t, "network fee basis", r.Effects[1].FeeReleasedCost, "10")
	assertKnown(t, "network fee disposal gain", r.Effects[1].FeeDisposalPNL, "5")
	assertKnown(t, "net fee pnl", r.RealizedPNL, "-10")
	assertDecimal(t, "fiat unaffected", r.Effects[1].CashDelta, "0")
	// The fee's disposal gain and expense are presented individually, but the
	// net result must remove its cost once, never charge the fee twice.
	assertDecimal(t, "fee identity", decimal.RequireFromString(*r.Effects[1].FeeDisposalPNL).Sub(decimal.RequireFromString(*r.Effects[1].InvestmentFee)).String(), *r.RealizedPNL)
}

func TestTransferWithoutFeePreservesAggregateQuantityCostAndPNL(t *testing.T) {
	opening := event("01", Opening, "3", "0", "0")
	opening.Cost = textPtr("1")
	transfer := event("02", Transfer, "1", "0", "0")
	transfer.ToAccountID = "wallet"
	transfer.ExchangeRate = ""
	r := mustReplay(t, opening, transfer)
	quantity, cost := decimal.Zero, decimal.Zero
	for _, p := range r.Positions {
		quantity = quantity.Add(decimal.RequireFromString(p.Quantity))
		cost = cost.Add(decimal.RequireFromString(*p.Cost))
	}
	assertDecimal(t, "total quantity", quantity.String(), "3")
	assertDecimal(t, "total cost", cost.String(), "1")
	assertKnown(t, "no realization", r.RealizedPNL, "0")
	assertKnown(t, "no fee", r.Effects[1].InvestmentFee, "0")
}

func TestTransferUnknownCostAndMissingFeeMarketRate(t *testing.T) {
	opening := event("01", Opening, "3", "0", "0")
	transfer := event("02", Transfer, "1", "0", "0")
	transfer.ToAccountID = "wallet"
	transfer.ExchangeRate = ""
	r := mustReplay(t, opening, transfer)
	if findPosition(t, r, "wallet", "btc").CostKnown {
		t.Fatal("unknown transferred basis became known")
	}
	assertKnown(t, "transfer with no fee has no gain", r.RealizedPNL, "0")
	opening.Cost = textPtr("300")
	transfer.Fee = "0.1"
	r = mustReplay(t, opening, transfer)
	assertKnown(t, "known net fee loss without market price", r.RealizedPNL, "-10")
	if r.Effects[1].InvestmentFee != nil || r.Effects[1].FeeDisposalPNL != nil {
		t.Fatal("missing historical market rate must not invent a fee valuation")
	}
}

func TestUSDTSettlementReleasesStablecoinBasisAndRealizesItsGain(t *testing.T) {
	usdt := event("01", Opening, "1000", "0", "0")
	usdt.InstrumentID = "usdt"
	usdt.Cost = textPtr("7000")
	buy := event("02", Buy, "0.01", "800", "2")
	buy.SettlementInstrumentID = "usdt"
	buy.ExchangeRate = "7.2"
	r := mustReplay(t, usdt, buy)
	stable := findPosition(t, r, "exchange", "usdt")
	btc := findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "remaining usdt", stable.Quantity, "198")
	assertKnown(t, "remaining usdt cost", stable.Cost, "1386")
	assertKnown(t, "btc acquisition cost", btc.Cost, "5774.4")
	assertKnown(t, "disposed usdt cost", r.Effects[1].SettlementReleasedCost, "5614")
	assertKnown(t, "stablecoin realized pnl", stable.RealizedPNL, "160.4")
	assertKnown(t, "total realized pnl", r.RealizedPNL, "160.4")
	assertKnown(t, "fee included once", r.Effects[1].InvestmentFee, "14.4")
	assertDecimal(t, "no fiat movement", r.Effects[1].CashDelta, "0")

	sell := event("03", Sell, "0.005", "500", "1")
	sell.SettlementInstrumentID = "usdt"
	sell.ExchangeRate = "7.1"
	r = mustReplay(t, usdt, buy, sell)
	stable = findPosition(t, r, "exchange", "usdt")
	btc = findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "usdt after sale", stable.Quantity, "697")
	assertKnown(t, "usdt cost after sale", stable.Cost, "4928.9")
	assertKnown(t, "btc remaining basis", btc.Cost, "2887.2")
	assertKnown(t, "btc sale pnl", btc.RealizedPNL, "655.7")
	assertKnown(t, "combined asset pnl", r.RealizedPNL, "816.1")
}

func TestCryptoSettlementCanUseAnotherPortfolioAndCannotOverspend(t *testing.T) {
	usdt := event("01", Opening, "100", "0", "0")
	usdt.AccountID, usdt.InstrumentID, usdt.Cost = "wallet", "usdt", textPtr("700")
	buy := event("02", Buy, "0.01", "99", "1")
	buy.SettlementInstrumentID, buy.SettlementAccountID, buy.ExchangeRate = "usdt", "wallet", "7"
	r := mustReplay(t, usdt, buy)
	assertKnown(t, "exhausted stablecoin basis", findPosition(t, r, "wallet", "usdt").Cost, "0")
	assertKnown(t, "acquired btc", findPosition(t, r, "exchange", "btc").Cost, "700")
	buy.Fee = "2"
	if _, err := Replay([]Event{usdt, buy}); err == nil || !strings.Contains(err.Error(), "usdt") {
		t.Fatal("must reject settlement overspending including fee")
	}
}

func TestMissingHistoricalRateLeavesKnownQuantitiesAndUnknownBasis(t *testing.T) {
	buy := event("01", Buy, "1", "100", "1")
	buy.ExchangeRate = ""
	r := mustReplay(t, buy)
	p := findPosition(t, r, "exchange", "btc")
	assertDecimal(t, "quantity", p.Quantity, "1")
	assertDecimal(t, "cash currency debit", r.Effects[0].CashDelta, "-101")
	if p.CostKnown || p.Cost != nil || r.Effects[0].CashDeltaCNY != nil || r.Effects[0].InvestmentFee != nil {
		t.Fatal("missing exchange rate must retain unknown monetary values")
	}
	opening := event("01", Opening, "1", "0", "0")
	opening.Cost = textPtr("100")
	sell := event("02", Sell, "1", "150", "0")
	sell.ExchangeRate = ""
	r = mustReplay(t, opening, sell)
	if r.RealizedPNL != nil {
		t.Fatal("cannot compute base-currency sale profit without historical rate")
	}
}

func TestEighteenDecimalInputsAndExplicitDivisionPrecision(t *testing.T) {
	opening := event("01", Opening, "3", "0", "0")
	opening.Cost = textPtr("1")
	r := mustReplay(t, opening)
	assertKnown(t, "36-place average", r.Positions[0].AverageCost, "0.333333333333333333333333333333333333")
	buy := event("01", Buy, "0.000000000000000001", "0.000000000000000001", "0")
	buy.ExchangeRate = "0.000000000000000001"
	r = mustReplay(t, buy)
	assertDecimal(t, "smallest input quantity", r.Positions[0].Quantity, "0.000000000000000001")
	assertKnown(t, "multiplication retains 36 places", r.Positions[0].Cost, "0.000000000000000000000000000000000001")
	assertKnown(t, "small unit cost", r.Positions[0].AverageCost, "0.000000000000000001")
}

func TestInputValidationRejectsExponentTruncationAndInvalidEvents(t *testing.T) {
	for _, input := range []string{"", "1e-8", " 1", "1 ", "+1", ".5", "1.", "NaN", "Infinity", "1,000", "0.0000000000000000001"} {
		t.Run(input, func(t *testing.T) {
			if err := ValidateDecimal(input); err == nil {
				t.Errorf("invalid plain decimal %q accepted", input)
			}
		})
	}
	for _, input := range []string{"0", "001.2300", "-1", "123.123456789012345678"} {
		if err := ValidateDecimal(input); err != nil {
			t.Errorf("valid decimal %q rejected: %v", input, err)
		}
	}
	if normalized, err := NormalizeDecimal("001.2300"); err != nil || normalized != "1.23" {
		t.Fatalf("unexpected normalization: %q, %v", normalized, err)
	}
	cases := []struct {
		name string
		edit func(*Event)
	}{
		{"negative quantity", func(e *Event) { e.Quantity = "-1" }},
		{"zero quantity", func(e *Event) { e.Quantity = "0" }},
		{"negative fee", func(e *Event) { e.Fee = "-1" }},
		{"zero rate", func(e *Event) { e.ExchangeRate = "0" }},
		{"missing amount", func(e *Event) { e.Amount = "" }},
		{"opening cost on buy", func(e *Event) { e.Cost = textPtr("1") }},
		{"same-asset swap", func(e *Event) { e.SettlementInstrumentID = "btc" }},
		{"unknown type", func(e *Event) { e.Type = "FUTURES" }},
		{"no account", func(e *Event) { e.AccountID = "" }},
		{"no instrument", func(e *Event) { e.InstrumentID = "" }},
		{"unsupported settlement account", func(e *Event) { e.SettlementAccountID = "cash" }},
		{"unexpected transfer target", func(e *Event) { e.ToAccountID = "wallet" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := event("invalid-event", Buy, "1", "10", "0")
			tc.edit(&e)
			_, err := Replay([]Event{e})
			var detail *ReplayError
			if !errors.As(err, &detail) || detail.EventID != e.ID {
				t.Fatalf("expected event-specific error, got %v", err)
			}
		})
	}
}

func TestVoidedEventsAreIgnoredAndDuplicateActiveIDsRejected(t *testing.T) {
	voided := event("01", Buy, "1", "10", "0")
	voided.Voided = true
	r := mustReplay(t, voided)
	if len(r.Positions) != 0 || len(r.Effects) != 0 {
		t.Fatal("voided event affected holdings")
	}
	e := event("same", Buy, "1", "10", "0")
	if _, err := Replay([]Event{e, e}); err == nil {
		t.Fatal("duplicate active event IDs accepted")
	}
}

func TestInvalidTransferAndFeeCannotLeavePartialReplay(t *testing.T) {
	buy := event("01", Buy, "1", "100", "0")
	for _, mutate := range []func(*Event){
		func(e *Event) { e.ToAccountID = "exchange" },
		func(e *Event) { e.Fee = "1" },
		func(e *Event) { e.SettlementInstrumentID = "eth" },
	} {
		transfer := event("02", Transfer, "0.5", "0", "0")
		transfer.ToAccountID = "wallet"
		mutate(&transfer)
		if r, err := Replay([]Event{buy, transfer}); err == nil || r != nil {
			t.Fatal("invalid transfer must produce no partial result")
		}
	}
	sell := event("02", Sell, "1", "10", "11")
	if _, err := Replay([]Event{buy, sell}); err == nil {
		t.Fatal("fee exceeding sale proceeds accepted")
	}
}
