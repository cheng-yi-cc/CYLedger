package investments

import (
	"github.com/shopspring/decimal"
	"strings"
)

// Wallet payments release the actual assets once. Their original fiat amount
// and confirmed historical FX determine the disposal/acquisition value; there
// is no cash balance and no assumption that a stablecoin always trades at $1.
func (r *replay) applyWallet(e Event) error {
	if strings.TrimSpace(e.AccountID) == "" || e.InstrumentID == "" || e.ToAccountID != "" || e.SettlementAccountID != "" || e.SettlementInstrumentID != "" || e.Cost != nil || len(e.AdditionalMovements) > 1 || (e.Type == Income && len(e.AdditionalMovements) > 0) {
		return errorAt(e, "wallet", "invalid wallet movement")
	}
	amount, err := parse(e, "amount", e.Amount, false)
	if err != nil {
		return err
	}
	if !amount.IsPositive() || amount.Exponent() < -2 {
		return errorAt(e, "amount", "must be positive with at most two decimal places")
	}
	fee, err := parse(e, "fee", e.Fee, true)
	if err != nil {
		return err
	}
	if !fee.IsZero() {
		return errorAt(e, "fee", "include actual fees in the payment amount and asset quantities")
	}
	rate, err := parse(e, "exchangeRate", e.ExchangeRate, false)
	if err != nil {
		return err
	}
	if !rate.IsPositive() {
		return errorAt(e, "exchangeRate", "a confirmed historical exchange rate is required")
	}
	legs := append([]AssetMovement{{InstrumentID: e.InstrumentID, Quantity: e.Quantity}}, e.AdditionalMovements...)
	quantities := make([]decimal.Decimal, len(legs))
	totalQuantity := decimal.Zero
	seen := map[string]bool{}
	for i, m := range legs {
		if m.InstrumentID == "" || seen[m.InstrumentID] {
			return errorAt(e, "instrumentId", "wallet assets must be distinct")
		}
		seen[m.InstrumentID] = true
		q, err := parse(e, "quantity", m.Quantity, false)
		if err != nil {
			return err
		}
		if !q.IsPositive() {
			return errorAt(e, "quantity", "must be greater than zero")
		}
		quantities[i] = q
		totalQuantity = totalQuantity.Add(q)
	}
	totalValue := amount.Mul(rate)
	remainingValue := totalValue
	effect := EventEffect{EventID: e.ID, Type: e.Type, Legs: []Leg{}, CashDelta: "0", CashDeltaCNY: zero().ptr(), AcquiredCost: zero().ptr(), ReleasedCost: zero().ptr(), RealizedPNL: zero().ptr()}
	releasedTotal, realized := zero(), zero()
	for i, m := range legs {
		allocated := remainingValue
		if i < len(legs)-1 {
			allocated = totalValue.Mul(quantities[i]).DivRound(totalQuantity, CalculationPrecision)
			remainingValue = remainingValue.Sub(allocated)
		}
		p := r.position(e.AccountID, m.InstrumentID)
		if e.Type == Income {
			r.deposit(p, quantities[i], known(allocated))
			effect.Legs = append(effect.Legs, leg(e.AccountID, m.InstrumentID, quantities[i], known(allocated)))
		} else {
			released, err := r.withdraw(e, p, quantities[i], e.AccountID, m.InstrumentID)
			if err != nil {
				return err
			}
			gain := known(allocated).sub(released)
			p.realized = p.realized.add(gain)
			realized = realized.add(gain)
			releasedTotal = releasedTotal.add(released)
			effect.Legs = append(effect.Legs, leg(e.AccountID, m.InstrumentID, quantities[i].Neg(), released.neg()))
		}
	}
	if e.Type == Income {
		effect.AcquiredCost = known(totalValue).ptr()
	} else {
		effect.ReleasedCost = releasedTotal.ptr()
	}
	effect.RealizedPNL = realized.ptr()
	r.realized = r.realized.add(realized)
	r.result.Effects = append(r.result.Effects, effect)
	return nil
}
