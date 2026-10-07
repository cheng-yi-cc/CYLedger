package investments

import "strings"

// 校准保存目标数量/成本及差额，不伪装成买卖，不修改资金或已实现收益。
func (r *replay) applyAdjustment(e Event) error {
	if strings.TrimSpace(e.AccountID) == "" || strings.TrimSpace(e.InstrumentID) == "" {
		return errorAt(e, "accountId/instrumentId", "both are required")
	}
	q, err := parse(e, "quantity", e.Quantity, false)
	if err != nil {
		return err
	}
	if e.ToAccountID != "" || e.SettlementAccountID != "" || e.SettlementInstrumentID != "" || len(e.AdditionalMovements) != 0 {
		return errorAt(e, "settlement", "holding adjustments cannot transfer or settle assets")
	}
	for _, raw := range []string{e.Amount, e.Fee} {
		d, err := parse(e, "amount/fee", raw, true)
		if err != nil {
			return err
		}
		if !d.IsZero() {
			return errorAt(e, "amount/fee", "holding adjustments cannot move cash")
		}
	}
	cost := value{}
	if e.Cost != nil {
		d, err := parse(e, "cost", *e.Cost, false)
		if err != nil {
			return err
		}
		cost = known(d)
	}
	if q.IsZero() {
		if cost.known && !cost.d.IsZero() {
			return errorAt(e, "cost", "empty holdings must have zero cost")
		}
		cost = zero()
	}
	p := r.position(e.AccountID, e.InstrumentID)
	fx := EventEffect{EventID: e.ID, Type: e.Type, Legs: []Leg{leg(e.AccountID, e.InstrumentID, q.Sub(p.quantity), cost.sub(p.cost))}, CashDelta: "0", CashDeltaCNY: zero().ptr(), RealizedPNL: zero().ptr(), InvestmentFee: zero().ptr()}
	p.quantity, p.cost = q, cost
	r.result.Effects = append(r.result.Effects, fx)
	return nil
}
