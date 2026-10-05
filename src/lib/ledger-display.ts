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

export function ledgerTotals(items: {type: number; cny: string | null; excludeFromStatistics?: boolean}[]) {
    let income = new LedgerDecimal(0), expense = new LedgerDecimal(0);
    let complete = true;
    for (const item of items) {
        if (item.excludeFromStatistics) continue;
        if (item.type !== 2 && item.type !== 3) continue;
        if (item.cny === null) { complete = false; continue; }
        if (item.type === 2) income = income.plus(item.cny);
        else expense = expense.plus(item.cny);
    }
    return { income: income.toString(), expense: expense.toString(), balance: income.minus(expense).toString(), complete };
}

export function ledgerCategoryTotals(items: { type: number; cny: string | null; categoryId: string; primaryCategoryId: string; primaryCategory: string; title: string; excludeFromStatistics?: boolean }[], primary: boolean, type: number) {
    const groups = new Map<string, { id: string; name: string; amount: InstanceType<typeof LedgerDecimal> }>();
    for (const item of items) {
        if (item.excludeFromStatistics) continue;
        if (item.type !== type || item.cny === null) continue;
        const id = primary ? item.primaryCategoryId : item.categoryId;
        const name = primary ? item.primaryCategory : item.primaryCategoryId === item.categoryId ? item.title : `${item.primaryCategory} / ${item.title}`;
        const group = groups.get(id) || { id, name, amount: new LedgerDecimal(0) };
        group.amount = group.amount.plus(item.cny);
        groups.set(id, group);
    }
    const maximum = [...groups.values()].reduce((max, item) => LedgerDecimal.max(max, item.amount.abs()), new LedgerDecimal(1));
    return [...groups.values()].sort((a,b) => b.amount.abs().comparedTo(a.amount.abs())).map(item => ({ id: item.id, name: item.name, amount: item.amount.toString(), width: item.amount.abs().div(maximum).mul(100).toNumber() }));
}
