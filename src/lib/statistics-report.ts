import moment from 'moment-timezone';
import { LedgerDecimal, ledgerTotals } from '@/lib/ledger-display.ts';
import type { LedgerEntry } from '@/lib/mobile-ledger.ts';
import type { StatisticsBudget, StatisticsPreferences } from '@/lib/statistics-workspace.ts';

export type StatisticsMetric = 'expense' | 'income' | 'balance';
export type TagStatisticsMode = 'all' | 'parents' | 'direct' | 'children';
export interface StatisticsTag { id: string; name: string; parentId?: string }
export interface ReportGroup {
    id: string; name: string; income: string; expense: string; balance: string;
    count: number; complete: boolean; icon?: string; iconType?: number;
}
export interface ChartBucket {
    key: string; label: string; from: number; to: number; income: string; expense: string; balance: string; complete: boolean; count: number;
}
export function statisticsEligible(item: Pick<LedgerEntry, 'type' | 'investment' | 'excludeFromStatistics'>): boolean {
    return !item.investment && !item.excludeFromStatistics && [2, 3].includes(item.type);
}
export function withinRange(entries: LedgerEntry[], from: number, to: number): LedgerEntry[] {
    return entries.filter(entry => entry.time >= from && entry.time <= to);
}
export function tagGroupIds(ids: string[], tags: Record<string, StatisticsTag>, mode: TagStatisticsMode): string[] {
    return [...new Set(ids.flatMap(id => {
        const parent = tags[id]?.parentId;
        if (mode === 'parents') return [parent && parent !== '0' ? parent : id];
        if (mode === 'direct') return parent && parent !== '0' ? [] : [id];
        if (mode === 'children') return parent && parent !== '0' ? [id] : [];
        return [id];
    }))];
}
export function groupReport(entries: LedgerEntry[], dimension: 'category' | 'account' | 'tag', options: { primary?: boolean; tags?: Record<string, StatisticsTag>; tagMode?: TagStatisticsMode; type?: number } = {}): ReportGroup[] {
    const groups = new Map<string, { name: string; entries: LedgerEntry[] }>();
    for (const entry of entries) {
        if (!statisticsEligible(entry) || (options.type && entry.type !== options.type)) continue;
        let ids: string[];
        if (dimension === 'tag') ids = tagGroupIds(entry.tagIds, options.tags || {}, options.tagMode || 'all');
        else ids = [dimension === 'account' ? entry.accountId : options.primary !== false ? entry.primaryCategoryId : entry.categoryId];
        for (const id of ids) {
            const name = dimension === 'tag' ? options.tags?.[id]?.name || '已删除标签' : dimension === 'account' ? entry.account : options.primary !== false ? entry.primaryCategory : entry.title;
            const group = groups.get(id) || { name, entries: [] };
            group.entries.push(entry); groups.set(id, group);
        }
    }
    return [...groups].map(([id, group]) => ({ id, name: group.name, ...ledgerTotals(group.entries), count: group.entries.length, icon: group.entries[0]?.icon, iconType: group.entries[0]?.iconType }));
}
export function reportBreakdown(groups: ReportGroup[], metric: StatisticsMetric) {
    const items = [...groups].sort((a, b) => new LedgerDecimal(b[metric]).abs().comparedTo(new LedgerDecimal(a[metric]).abs()) || a.id.localeCompare(b.id));
    const total = items.reduce((sum, item) => sum.plus(item[metric]), new LedgerDecimal(0));
    const canShare = total.gt(0) && items.every(item => item.complete && new LedgerDecimal(item[metric]).gte(0));
    const max = LedgerDecimal.max(1, ...items.map(item => new LedgerDecimal(item[metric]).abs()));
    return { total: total.toString(), canShare, complete: items.every(item => item.complete), items: items.map(item => ({ ...item, amount: item[metric], percent: canShare ? new LedgerDecimal(item[metric]).div(total).mul(100).toFixed(2) : null, width: new LedgerDecimal(item[metric]).abs().div(max).mul(100).toNumber() })) };
}
export function reportBuckets(entries: LedgerEntry[], from: moment.Moment, to: moment.Moment, unit: 'day' | 'month' | 'year' = 'day'): ChartBucket[] {
    const format = unit === 'year' ? 'YYYY' : unit === 'month' ? 'YYYY-MM' : 'YYYY-MM-DD';
    const byKey = new Map<string, LedgerEntry[]>();
    for (const entry of entries) {
        if (!statisticsEligible(entry) || entry.time < from.unix() || entry.time > to.unix()) continue;
        const key = moment.unix(entry.time).tz(from.tz() || 'UTC').format(format);
        const group = byKey.get(key) || []; group.push(entry); byKey.set(key, group);
    }
    const result: ChartBucket[] = [], cursor = from.clone().startOf(unit);
    while (cursor.isSameOrBefore(to)) {
        const key = cursor.format(format);
        result.push({ key, label: cursor.format(unit === 'year' ? 'YYYY年' : unit === 'month' ? 'M月' : 'M-D'), from: Math.max(from.unix(), cursor.unix()), to: Math.min(to.unix(), cursor.clone().endOf(unit).unix()), count: byKey.get(key)?.length || 0, ...ledgerTotals(byKey.get(key) || []) });
        cursor.add(1, unit);
    }
    return result;
}
export function elapsedDays(from: moment.Moment, to: moment.Moment, now: moment.Moment): number {
    const end = now.isBetween(from, to, undefined, '[]') ? now : to;
    return Math.max(1, end.clone().startOf('day').diff(from.clone().startOf('day'), 'days') + 1);
}
export function comparisonValue(current: string, previous: string, complete = true): { difference: string | null; percent: string | null } {
    if (!complete) return { difference: null, percent: null };
    const before = new LedgerDecimal(previous), difference = new LedgerDecimal(current).minus(before);
    return { difference: difference.toString(), percent: before.isZero() ? null : difference.div(before.abs()).mul(100).toFixed(1) };
}
export interface BudgetReport {
    budget: StatisticsBudget; base: string; carry: string | null; available: string | null; spent: string;
    remaining: string | null; complete: boolean; from: string; to: string;
}
/** Replay monthly limits from facts. Carry is a calculation, never a second balance. */
export function budgetReports(budgets: StatisticsBudget[], entries: LedgerEntry[], month: string, preferences: Pick<StatisticsPreferences, 'carrySurplus' | 'carryDeficit'>): BudgetReport[] {
    const target = moment.utc(month, 'YYYY-MM', true);
    if (!target.isValid()) return [];
    const lanes = new Map<string, StatisticsBudget[]>(), result: BudgetReport[] = [];
    const expenseEntries = entries.filter(entry => statisticsEligible(entry) && entry.type === 3);
    const totalsFor = (budget: StatisticsBudget, from: string, to: string) => ledgerTotals(expenseEntries.filter(entry => entry.bookId === budget.bookId && entry.day >= from && entry.day <= to && (budget.categoryId === '0' || entry.categoryId === budget.categoryId || entry.primaryCategoryId === budget.categoryId)));
    for (const budget of budgets) {
        if (budget.kind === 'custom') {
            if (budget.startDate > target.clone().endOf('month').format('YYYY-MM-DD') || budget.endDate < `${month}-01`) continue;
            const spent = totalsFor(budget, budget.startDate, budget.endDate);
            result.push({ budget, base: budget.amount, carry: '0', available: budget.amount, spent: spent.expense, remaining: spent.complete ? new LedgerDecimal(budget.amount).minus(spent.expense).toString() : null, complete: spent.complete, from: budget.startDate, to: budget.endDate });
        } else {
            const key = `${budget.bookId}/${budget.categoryId}`, list = lanes.get(key) || [];
            list.push(budget); lanes.set(key, list);
        }
    }
    for (const lane of lanes.values()) {
        lane.sort((a, b) => a.startDate.localeCompare(b.startDate));
        const cursor = moment.utc(lane[0]!.startDate).startOf('month');
        let remaining: InstanceType<typeof LedgerDecimal> | null = new LedgerDecimal(0);
        while (cursor.isSameOrBefore(target, 'month')) {
            const from = cursor.format('YYYY-MM-DD'), to = cursor.clone().endOf('month').format('YYYY-MM-DD');
            const budget = lane.find(item => item.startDate === from) || [...lane].reverse().find(item => item.repeat && item.startDate < from);
            if (!budget) { remaining = new LedgerDecimal(0); cursor.add(1, 'month'); continue; }
            const carry: InstanceType<typeof LedgerDecimal> | null = !preferences.carrySurplus && !preferences.carryDeficit ? new LedgerDecimal(0) : remaining === null ? null : remaining.gt(0) && preferences.carrySurplus || remaining.lt(0) && preferences.carryDeficit ? remaining : new LedgerDecimal(0);
            const available: InstanceType<typeof LedgerDecimal> | null = carry === null ? null : carry.plus(budget.amount);
            const spent = totalsFor(budget, from, to);
            remaining = !spent.complete || available === null ? null : available.minus(spent.expense);
            if (cursor.isSame(target, 'month')) result.push({ budget, base: budget.amount, carry: carry?.toString() ?? null, available: available?.toString() ?? null, spent: spent.expense, remaining: remaining?.toString() ?? null, complete: remaining !== null, from, to });
            cursor.add(1, 'month');
        }
    }
    return result;
}

export function budgetDaily(report: BudgetReport, entries: LedgerEntry[], now: moment.Moment, mode: 'remaining' | 'fixed') {
    const from = moment.tz(report.from, now.tz() || 'UTC'), to = moment.tz(report.to, now.tz() || 'UTC').endOf('day');
    const current = now.isBetween(from, to, undefined, '[]');
    if (!report.complete || report.available === null) return { daily: null, todayRemaining: null, average: null };
    const days = to.clone().startOf('day').diff(from, 'days') + 1;
    const today = ledgerTotals(entries.filter(entry => statisticsEligible(entry) && entry.type === 3 && entry.bookId === report.budget.bookId && entry.day === now.format('YYYY-MM-DD') && (report.budget.categoryId === '0' || entry.categoryId === report.budget.categoryId || entry.primaryCategoryId === report.budget.categoryId))).expense;
    const daily = mode === 'remaining' && current ? new LedgerDecimal(report.remaining!).plus(today).div(to.clone().startOf('day').diff(now.clone().startOf('day'), 'days') + 1) : new LedgerDecimal(report.available).div(days);
    return { daily: daily.toFixed(2), todayRemaining: current ? daily.minus(today).toFixed(2) : null, average: new LedgerDecimal(report.spent).div(elapsedDays(from, to, now)).toFixed(2) };
}
