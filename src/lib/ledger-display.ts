import Decimal from 'decimal.js';

// Keep financial aggregation in decimal arithmetic; convert only chart coordinates.
export const LedgerDecimal = Decimal.clone({ precision: 40 });

export function ledgerMoney(value: string | null | undefined, symbol = true): string {
    if (value === null || value === undefined) return '—';
    const [whole, fraction] = new LedgerDecimal(value).toFixed(2).split('.');
    return `${symbol ? '¥' : ''}${whole!.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}.${fraction}`;
}

export function ledgerSignedAmount(amount: string, type: number): string {
    if (type !== 2 && type !== 3) return ledgerMoney(amount,false);
    const value = new LedgerDecimal(amount).mul(type === 3 ? -1 : 1);
    return `${value.gt(0) ? '+' : ''}${ledgerMoney(value.toString(),false)}`;
}

export function ledgerTotals(items: {type: number; cny: string | null}[]) {
    let income = new LedgerDecimal(0), expense = new LedgerDecimal(0);
    let complete = true;
    for (const item of items) {
        if (item.type !== 2 && item.type !== 3) continue;
        if (item.cny === null) { complete = false; continue; }
        if (item.type === 2) income = income.plus(item.cny);
        else expense = expense.plus(item.cny);
    }
    return { income: income.toString(), expense: expense.toString(), balance: income.minus(expense).toString(), complete };
}
