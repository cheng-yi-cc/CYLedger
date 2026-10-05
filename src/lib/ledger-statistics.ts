import { LedgerDecimal, ledgerCategoryTotals, ledgerTotals } from '@/lib/ledger-display.ts';
interface StatEntry {
    type: number; cny: string | null; day: string; categoryId: string;
    primaryCategoryId: string; primaryCategory: string; title: string; investment?: boolean; excludeFromStatistics?: boolean;
}
export function categoryBreakdown(entries: StatEntry[], primary: boolean, type: number) {
    const valid = entries.filter(e => !e.investment && !e.excludeFromStatistics && e.type === type && e.cny !== null);
    const groups = ledgerCategoryTotals(valid, primary, type);
    const total = groups.reduce((s,g) => s.plus(g.amount), new LedgerDecimal(0));
    // A net refund is negative, not a positive slice of a spending pie.
    const canShare = total.gt(0) && groups.every(g => new LedgerDecimal(g.amount).gte(0));
    const counts = new Map<string, number>();
    for (const e of valid) { const id = primary ? e.primaryCategoryId : e.categoryId; counts.set(id, (counts.get(id) || 0) + 1); }
    let offset = 0;
    const items = groups.map(g => {
        const percent = canShare ? new LedgerDecimal(g.amount).div(total).mul(100) : null;
        const share = percent?.toNumber() || 0;
        const item = {...g, count:counts.get(g.id) || 0, percent:percent?.toFixed(2) ?? null, share, offset};
        offset += share; return item;
    });
    return {items, total:total.toString(), canShare};
}
export function dailyLedgerTotals(entries: StatEntry[]) {
    const days = new Map<string, StatEntry[]>();
    for (const e of entries) {
        if (e.investment || e.excludeFromStatistics || ![2,3].includes(e.type)) continue;
        const group = days.get(e.day) || []; group.push(e); days.set(e.day, group);
    }
    return [...days].sort(([a],[b]) => b.localeCompare(a)).map(([day,items]) => ({day, ...ledgerTotals(items)}));
}
