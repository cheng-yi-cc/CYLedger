// Package investments rebuilds investment holdings from immutable event facts.
// It deliberately has no database, pricing-provider or current-exchange-rate
// dependencies: an event's historical exchange rate must never change on replay.
package investments

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	Opening  = "OPENING"
	Buy      = "BUY"
	Sell     = "SELL"
	Transfer = "TRANSFER"

	// CalculationPrecision applies explicitly to every division. Input facts
	// accept 18 decimal places; derived costs retain 36 decimal places.
	CalculationPrecision int32 = 36
)

// Event contains transaction facts. Cost is an optional total CNY opening cost.
// For BUY/SELL, Amount and Fee are in the settlement asset/currency and
// ExchangeRate is its historical CNY unit value (use "1" for CNY).
// For TRANSFER, Quantity is the quantity received, Fee is an additional quantity
// of the same asset deducted from the source, and ExchangeRate is that asset's
// historical CNY unit value, used only to disclose the network fee's value.
// Missing ExchangeRate is allowed and makes the affected monetary results
// unknown. An empty settlement instrument means fiat; otherwise settlement is
// another independently costed investment position.
type Event struct {
	ID                     string  `json:"id"`
	Type                   string  `json:"type"`
	AccountID              string  `json:"accountId"`
	InstrumentID           string  `json:"instrumentId"`
	ToAccountID            string  `json:"toAccountId,omitempty"`
	Quantity               string  `json:"quantity"`
	Amount                 string  `json:"amount"`
	Fee                    string  `json:"fee"`
	Cost                   *string `json:"cost"`
	SettlementInstrumentID string  `json:"settlementInstrumentId,omitempty"`
	SettlementAccountID    string  `json:"settlementAccountId,omitempty"`
	ExchangeRate           string  `json:"exchangeRate"`
	OccurredAt             int64   `json:"occurredAt"`
	Note                   string  `json:"note"`
	Version                int     `json:"version"`
	Voided                 bool    `json:"voided"`
}

type Position struct {
	AccountID    string  `json:"accountId"`
	InstrumentID string  `json:"instrumentId"`
	Quantity     string  `json:"quantity"`
	Cost         *string `json:"cost"`
	CostKnown    bool    `json:"costKnown"`
	AverageCost  *string `json:"averageCost"`
	RealizedPNL  *string `json:"realizedPnl"`
}

// Leg is an auditable investment quantity/cost movement. Unknown cost is null,
// never "0". Cash movements are separate because upstream cash has its own
// accounting model and the caller must commit both layers atomically.
type Leg struct {
	AccountID     string  `json:"accountId"`
	InstrumentID  string  `json:"instrumentId"`
	QuantityDelta string  `json:"quantityDelta"`
	CostDelta     *string `json:"costDelta"`
}

type EventEffect struct {
	EventID                string  `json:"eventId"`
	Type                   string  `json:"type"`
	Legs                   []Leg   `json:"legs"`
	CashDelta              string  `json:"cashDelta"`
	CashDeltaCNY           *string `json:"cashDeltaCny"`
	AcquiredCost           *string `json:"acquiredCost"`
	ReleasedCost           *string `json:"releasedCost"`
	SettlementReleasedCost *string `json:"settlementReleasedCost"`
	SettlementRealizedPNL  *string `json:"settlementRealizedPnl"`
	RealizedPNL            *string `json:"realizedPnl"`
	InvestmentFee          *string `json:"investmentFee"`
	FeeReleasedCost        *string `json:"feeReleasedCost"`
	FeeDisposalPNL         *string `json:"feeDisposalPnl"`
}

type Result struct {
	Positions   []Position    `json:"positions"`
	Effects     []EventEffect `json:"effects"`
	RealizedPNL *string       `json:"realizedPnl"`
}

// ReplayError identifies the specific historical event that would conflict
// after an edit, allowing the caller to reject its database transaction.
type ReplayError struct {
	EventID string `json:"eventId"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("investment event %q: %s: %s", e.EventID, e.Field, e.Message)
}

var decimalPattern = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,18})?$`)

// ValidateDecimal accepts plain decimal strings, never exponents, whitespace,
// fractions or floating-point JSON values. The length limit bounds input work.
func ValidateDecimal(value string) error {
	if len(value) > 128 || !decimalPattern.MatchString(value) {
		return fmt.Errorf("must be a plain decimal string with at most 18 decimal places and 128 characters")
	}
	return nil
}

func NormalizeDecimal(value string) (string, error) {
	if err := ValidateDecimal(value); err != nil {
		return "", err
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return "", err
	}
	return d.String(), nil
}

type value struct {
	d     decimal.Decimal
	known bool
}

func known(d decimal.Decimal) value { return value{d: d, known: true} }
func zero() value                   { return known(decimal.Zero) }
func (v value) ptr() *string {
	if !v.known {
		return nil
	}
	s := v.d.String()
	return &s
}
func (v value) add(other value) value {
	if !v.known || !other.known {
		return value{}
	}
	return known(v.d.Add(other.d))
}
func (v value) sub(other value) value {
	return v.add(other.neg())
}
func (v value) neg() value {
	if !v.known {
		return value{}
	}
	return known(v.d.Neg())
}
func (v value) mul(q decimal.Decimal) value {
	if q.IsZero() {
		return zero()
	}
	if !v.known {
		return value{}
	}
	return known(v.d.Mul(q))
}

type positionKey struct{ accountID, instrumentID string }
type position struct {
	quantity decimal.Decimal
	cost     value
	realized value
}

type replay struct {
	positions map[positionKey]*position
	result    Result
	realized  value
}

func (r *replay) position(accountID, instrumentID string) *position {
	k := positionKey{accountID, instrumentID}
	if p, ok := r.positions[k]; ok {
		return p
	}
	p := &position{quantity: decimal.Zero, cost: zero(), realized: zero()}
	r.positions[k] = p
	return p
}

func errorAt(e Event, field, message string) error {
	return &ReplayError{EventID: e.ID, Field: field, Message: message}
}

func parse(e Event, field, raw string, allowEmpty bool) (decimal.Decimal, error) {
	if raw == "" && allowEmpty {
		return decimal.Zero, nil
	}
	if err := ValidateDecimal(raw); err != nil {
		return decimal.Zero, errorAt(e, field, err.Error())
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, errorAt(e, field, "invalid decimal")
	}
	if d.IsNegative() {
		return decimal.Zero, errorAt(e, field, "must not be negative")
	}
	return d, nil
}

// Replay sorts a copy of events by OccurredAt and ID, preserving its caller's
// input. Voided events have no effect. Every active fact must be valid, and no
// historical position may become negative. No partial result is returned on an
// error, so callers cannot accidentally persist a partially replayed portfolio.
func Replay(events []Event) (*Result, error) {
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].OccurredAt == ordered[j].OccurredAt {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].OccurredAt < ordered[j].OccurredAt
	})
	r := replay{positions: make(map[positionKey]*position), realized: zero(), result: Result{Positions: []Position{}, Effects: []EventEffect{}}}
	seen := make(map[string]bool)
	for _, e := range ordered {
		if e.Voided {
			continue
		}
		if strings.TrimSpace(e.ID) == "" {
			return nil, errorAt(e, "id", "is required")
		}
		if seen[e.ID] {
			return nil, errorAt(e, "id", "duplicate active event")
		}
		seen[e.ID] = true
		if err := r.apply(e); err != nil {
			return nil, err
		}
	}
	keys := make([]positionKey, 0, len(r.positions))
	for key := range r.positions {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].accountID == keys[j].accountID {
			return keys[i].instrumentID < keys[j].instrumentID
		}
		return keys[i].accountID < keys[j].accountID
	})
	for _, k := range keys {
		p := r.positions[k]
		var avg *string
		if p.cost.known && p.quantity.IsPositive() {
			avg = known(p.cost.d.DivRound(p.quantity, CalculationPrecision)).ptr()
		}
		r.result.Positions = append(r.result.Positions, Position{AccountID: k.accountID, InstrumentID: k.instrumentID,
			Quantity: p.quantity.String(), Cost: p.cost.ptr(), CostKnown: p.cost.known, AverageCost: avg, RealizedPNL: p.realized.ptr()})
	}
	r.result.RealizedPNL = r.realized.ptr()
	return &r.result, nil
}

func (r *replay) apply(e Event) error {
	if strings.TrimSpace(e.AccountID) == "" || strings.TrimSpace(e.InstrumentID) == "" {
		return errorAt(e, "accountId/instrumentId", "both are required")
	}
	q, err := parse(e, "quantity", e.Quantity, false)
	if err != nil {
		return err
	}
	if !q.IsPositive() {
		return errorAt(e, "quantity", "must be greater than zero")
	}
	amount, err := parse(e, "amount", e.Amount, true)
	if err != nil {
		return err
	}
	fee, err := parse(e, "fee", e.Fee, true)
	if err != nil {
		return err
	}
	rate := value{}
	if e.ExchangeRate != "" {
		d, err := parse(e, "exchangeRate", e.ExchangeRate, false)
		if err != nil {
			return err
		}
		if !d.IsPositive() {
			return errorAt(e, "exchangeRate", "must be greater than zero when supplied")
		}
		rate = known(d)
	}
	if e.Cost != nil && e.Type != Opening {
		return errorAt(e, "cost", "explicit cost is only accepted for opening holdings")
	}
	if e.Type != Transfer && e.ToAccountID != "" {
		return errorAt(e, "toAccountId", "only supported for transfers")
	}
	if e.SettlementInstrumentID == "" && e.SettlementAccountID != "" {
		return errorAt(e, "settlementAccountId", "requires a settlement instrument; fiat accounts are linked separately")
	}
	fx := EventEffect{EventID: e.ID, Type: e.Type, Legs: []Leg{}, CashDelta: "0", CashDeltaCNY: zero().ptr(),
		AcquiredCost: zero().ptr(), ReleasedCost: zero().ptr(), SettlementReleasedCost: zero().ptr(),
		SettlementRealizedPNL: zero().ptr(), RealizedPNL: zero().ptr(), InvestmentFee: zero().ptr(),
		FeeReleasedCost: zero().ptr(), FeeDisposalPNL: zero().ptr()}
	p := r.position(e.AccountID, e.InstrumentID)
	realized := zero()
	switch e.Type {
	case Opening:
		if !amount.IsZero() || !fee.IsZero() || e.SettlementInstrumentID != "" {
			return errorAt(e, "amount/fee/settlementInstrumentId", "opening holdings do not have a settlement or fee")
		}
		cost := value{}
		if e.Cost != nil {
			d, err := parse(e, "cost", *e.Cost, false)
			if err != nil {
				return err
			}
			cost = known(d)
		}
		r.deposit(p, q, cost)
		fx.AcquiredCost = cost.ptr()
		fx.Legs = append(fx.Legs, leg(e.AccountID, e.InstrumentID, q, cost))
	case Buy, Sell:
		if !amount.IsPositive() {
			return errorAt(e, "amount", "must be greater than zero for a buy or sell")
		}
		settlementAccount := e.SettlementAccountID
		if settlementAccount == "" {
			settlementAccount = e.AccountID
		}
		if e.SettlementInstrumentID == e.InstrumentID {
			return errorAt(e, "settlementInstrumentId", "must differ from the traded instrument; use a transfer for the same asset")
		}
		fx.InvestmentFee = rate.mul(fee).ptr()
		if e.Type == Buy {
			total := amount.Add(fee)
			cost := rate.mul(total)
			if e.SettlementInstrumentID == "" {
				fx.CashDelta = total.Neg().String()
				fx.CashDeltaCNY = cost.neg().ptr()
			} else {
				settlement := r.position(settlementAccount, e.SettlementInstrumentID)
				released, err := r.withdraw(e, settlement, total, settlementAccount, e.SettlementInstrumentID)
				if err != nil {
					return err
				}
				realized = cost.sub(released)
				settlement.realized = settlement.realized.add(realized)
				fx.SettlementReleasedCost = released.ptr()
				fx.SettlementRealizedPNL = realized.ptr()
				fx.Legs = append(fx.Legs, leg(settlementAccount, e.SettlementInstrumentID, total.Neg(), released.neg()))
			}
			r.deposit(p, q, cost)
			fx.AcquiredCost = cost.ptr()
			fx.Legs = append(fx.Legs, leg(e.AccountID, e.InstrumentID, q, cost))
		} else {
			if fee.GreaterThan(amount) {
				return errorAt(e, "fee", "must not exceed sell proceeds")
			}
			net := amount.Sub(fee)
			proceeds := rate.mul(net)
			released, err := r.withdraw(e, p, q, e.AccountID, e.InstrumentID)
			if err != nil {
				return err
			}
			realized = proceeds.sub(released)
			p.realized = p.realized.add(realized)
			fx.ReleasedCost = released.ptr()
			fx.Legs = append(fx.Legs, leg(e.AccountID, e.InstrumentID, q.Neg(), released.neg()))
			if e.SettlementInstrumentID == "" {
				fx.CashDelta = net.String()
				fx.CashDeltaCNY = proceeds.ptr()
			} else if net.IsPositive() {
				settlement := r.position(settlementAccount, e.SettlementInstrumentID)
				r.deposit(settlement, net, proceeds)
				fx.Legs = append(fx.Legs, leg(settlementAccount, e.SettlementInstrumentID, net, proceeds))
			}
		}
	case Transfer:
		if strings.TrimSpace(e.ToAccountID) == "" || e.ToAccountID == e.AccountID {
			return errorAt(e, "toAccountId", "must identify a different investment account")
		}
		if !amount.IsZero() || e.SettlementInstrumentID != "" {
			return errorAt(e, "amount/settlementInstrumentId", "transfers only support a same-asset quantity and network fee")
		}
		total := q.Add(fee)
		released, err := r.withdraw(e, p, total, e.AccountID, e.InstrumentID)
		if err != nil {
			return err
		}
		transferred, feeCost := released, zero()
		if fee.IsPositive() {
			transferred, feeCost = value{}, value{}
			if released.known {
				transferred = known(released.d.Mul(q).DivRound(total, CalculationPrecision))
				feeCost = released.sub(transferred)
			}
		}
		target := r.position(e.ToAccountID, e.InstrumentID)
		r.deposit(target, q, transferred)
		realized = feeCost.neg()
		p.realized = p.realized.add(realized)
		feeValue := rate.mul(fee)
		fx.InvestmentFee = feeValue.ptr()
		fx.FeeReleasedCost = feeCost.ptr()
		fx.FeeDisposalPNL = feeValue.sub(feeCost).ptr()
		fx.ReleasedCost = released.ptr()
		fx.AcquiredCost = transferred.ptr()
		fx.Legs = append(fx.Legs, leg(e.AccountID, e.InstrumentID, total.Neg(), released.neg()), leg(e.ToAccountID, e.InstrumentID, q, transferred))
	default:
		return errorAt(e, "type", "must be OPENING, BUY, SELL or TRANSFER")
	}
	fx.RealizedPNL = realized.ptr()
	r.realized = r.realized.add(realized)
	r.result.Effects = append(r.result.Effects, fx)
	return nil
}

func (r *replay) deposit(p *position, q decimal.Decimal, cost value) {
	p.quantity = p.quantity.Add(q)
	p.cost = p.cost.add(cost)
}

func (r *replay) withdraw(e Event, p *position, q decimal.Decimal, accountID, instrumentID string) (value, error) {
	if q.GreaterThan(p.quantity) {
		return value{}, errorAt(e, "quantity", fmt.Sprintf("insufficient holding for account %q, instrument %q: available %s, required %s", accountID, instrumentID, p.quantity.String(), q.String()))
	}
	if q.Equal(p.quantity) {
		// Release the entire remaining basis, including rounding residues. Even
		// an unknown holding becomes a known empty position after a full exit.
		released := p.cost
		p.quantity, p.cost = decimal.Zero, zero()
		return released, nil
	}
	released := value{}
	if p.cost.known {
		released = known(p.cost.d.Mul(q).DivRound(p.quantity, CalculationPrecision))
	}
	p.quantity = p.quantity.Sub(q)
	p.cost = p.cost.sub(released)
	return released, nil
}

func leg(accountID, instrumentID string, quantity decimal.Decimal, cost value) Leg {
	return Leg{AccountID: accountID, InstrumentID: instrumentID, QuantityDelta: quantity.String(), CostDelta: cost.ptr()}
}
