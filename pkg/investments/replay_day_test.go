package investments

import "testing"

func TestReplayDayRebasesUnknownHistoricalCostAndCarriesTransfers(t *testing.T) {
	opening := event("01", Opening, "2", "0", "0")
	opening.OccurredAt = 99
	transfer := event("02", Transfer, "1", "0", "0")
	transfer.OccurredAt, transfer.ToAccountID = 101, "wallet"
	sell := event("03", Sell, "0.5", "65", "1")
	sell.OccurredAt, sell.AccountID = 102, "wallet"
	r, err := ReplayDay([]Event{sell, opening, transfer}, 100, map[string]string{"btc": "100"})
	if err != nil {
		t.Fatal(err)
	}
	a, b := findPosition(t, r, "exchange", "btc"), findPosition(t, r, "wallet", "btc")
	assertKnown(t, "remaining original daily basis", a.Cost, "100")
	assertKnown(t, "remaining transferred daily basis", b.Cost, "50")
	assertKnown(t, "today realized net fees", b.RealizedPNL, "14")
	assertKnown(t, "today realized total", r.RealizedPNL, "14")
	if opening.Cost != nil {
		t.Fatal("mutated original cost")
	}
	lifetime := mustReplay(t, opening, transfer, sell)
	if lifetime.RealizedPNL != nil {
		t.Fatal("invented historical cost")
	}
}

func TestReplayDayBoundaryBuyAndMissingReference(t *testing.T) {
	buy := event("today", Buy, "1", "120", "2")
	buy.OccurredAt = 100
	r, err := ReplayDay([]Event{buy}, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertKnown(t, "today purchase including fees", r.Positions[0].Cost, "122")
	old := event("old", Opening, "1", "0", "0")
	old.Cost = textPtr("10")
	old.OccurredAt = 99
	r, err = ReplayDay([]Event{old, buy}, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Positions[0].Cost != nil {
		t.Fatal("missing opening price became zero")
	}
	adjust := event("adjust", Adjust, "3", "0", "0")
	adjust.Cost = textPtr("1")
	adjust.OccurredAt = 101
	r, err = ReplayDay([]Event{old, adjust}, 100, map[string]string{"btc": "100"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Positions[0].Cost != nil {
		t.Fatal("today calibration created artificial return")
	}
}
