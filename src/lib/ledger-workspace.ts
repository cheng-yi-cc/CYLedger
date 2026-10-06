import axios from 'axios';
import moment from 'moment-timezone';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import type { LedgerEntry } from '@/lib/mobile-ledger.ts';
import type { Account } from '@/models/account.ts';
import { LedgerDecimal, ledgerTotals } from '@/lib/ledger-display.ts';
export interface WishLog { id: string; date: string; amount: string; note: string }
export interface LedgerWish {
    name: string; icon: string; target: string; initial: string; startDate: string; endDate: string;
    mode: 'manual' | 'schedule' | 'income' | 'balance' | 'asset'; cycle: 'day' | 'week' | 'month' | 'year';
    amount: string; ratio: string; bookIds: string[]; accountIds: string[]; incomeCategoryIds: string[]; expenseCategoryIds: string[];
    note: string; archived: boolean; logs: WishLog[];
}
export interface LedgerKeyword { keywords: string[]; categoryId: string; accountId: string; tagIds: string[]; enabled: boolean }
export interface LedgerWorkspaceItem<T> { id: string; kind: 'wish' | 'keyword'; revision: string; data: T }
export function newLedgerWish(timeZone: string): LedgerWorkspaceItem<LedgerWish> {
    return { id: '', kind: 'wish', revision: '0', data: { name: '', icon: 'star', target: '', initial: '0', startDate: moment().tz(timeZone).format('YYYY-MM-DD'), endDate: '', mode: 'manual', cycle: 'month', amount: '0', ratio: '100', bookIds: [], accountIds: [], incomeCategoryIds: [], expenseCategoryIds: [], note: '', archived: false, logs: [] } };
}
async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> { const result = await axios.request<ApiResponse<T> & { errorMessage?: string }>({method,url:`v1/ledger/items${path}`,data}); if (!result.data.success) throw Error(result.data.errorMessage || '保存失败，请重试'); return result.data.result; }
export const ledgerWorkspace = {
    wishes: () => request<LedgerWorkspaceItem<LedgerWish>[]>('get','?kind=wish'),
    keywords: () => request<LedgerWorkspaceItem<LedgerKeyword>[]>('get','?kind=keyword'),
    save: <T>(item: LedgerWorkspaceItem<T>) => request<LedgerWorkspaceItem<T>>('post','/save',item),
    delete: <T>(item: LedgerWorkspaceItem<T>) => request<boolean>('post','/delete',{id:item.id,kind:item.kind,revision:item.revision})
};
export function wishProgress(wish: LedgerWish, entries: LedgerEntry[], accounts: Record<string, Account>, now: moment.Moment) {
    const end = wish.endDate && wish.endDate < now.format('YYYY-MM-DD') ? moment.tz(wish.endDate,now.tz() || 'UTC').endOf('day') : now;
    const start = moment.tz(wish.startDate,now.tz() || 'UTC').startOf('day');
    let saved: InstanceType<typeof LedgerDecimal> | null = new LedgerDecimal(wish.initial);
    for (const item of wish.logs || []) if (item.date <= end.format('YYYY-MM-DD')) saved = saved.plus(item.amount);
    if (wish.mode === 'schedule' && !end.isBefore(start)) {
        const occurrences = Math.max(0,end.diff(start,wish.cycle as moment.unitOfTime.Diff)+1);
        saved = saved.plus(new LedgerDecimal(wish.amount).mul(occurrences));
    } else if (wish.mode === 'income' || wish.mode === 'balance') {
        const eligible = entries.filter(item => item.time >= start.unix() && item.time <= end.unix() && (!wish.bookIds.length || wish.bookIds.includes(item.bookId)) && (item.type === 2 ? !wish.incomeCategoryIds.length || wish.incomeCategoryIds.includes(item.categoryId) || wish.incomeCategoryIds.includes(item.primaryCategoryId) : !wish.expenseCategoryIds.length || wish.expenseCategoryIds.includes(item.categoryId) || wish.expenseCategoryIds.includes(item.primaryCategoryId)));
        const totals = ledgerTotals(eligible);
        saved = totals.complete ? saved.plus(new LedgerDecimal(wish.mode === 'income' ? totals.income : totals.balance).mul(wish.ratio).div(100)) : null;
    } else if (wish.mode === 'asset') {
        const selected = Object.values(accounts).filter(account => !account.subAccounts?.length && (wish.accountIds.length ? wish.accountIds.includes(account.id) : !account.assetProfile.excludeFromTotal));
        saved = selected.some(account => account.currency !== 'CNY') ? null : selected.reduce((sum,account) => sum.plus(new LedgerDecimal(account.balance).div(100)),new LedgerDecimal(0)).mul(wish.ratio).div(100);
    }
    const target = new LedgerDecimal(wish.target || '0');
    return { saved: saved?.toFixed(2) ?? null, remaining: saved ? LedgerDecimal.max(0,target.minus(saved)).toFixed(2) : null, percent: saved && target.gt(0) ? LedgerDecimal.max(0,LedgerDecimal.min(100,saved.div(target).mul(100))).toNumber() : 0, achieved: !!saved && saved.gte(target), daysRemaining: wish.endDate ? moment.tz(wish.endDate,now.tz() || 'UTC').startOf('day').diff(now.clone().startOf('day'),'days') : null };
}
export function keywordMatch(text: string, rules: LedgerWorkspaceItem<LedgerKeyword>[]): LedgerKeyword | undefined {
    const normalized = text.trim().toLocaleLowerCase();
    return rules.find(rule => rule.data.enabled && rule.data.keywords.some(word => normalized.includes(word.toLocaleLowerCase())))?.data;
}
