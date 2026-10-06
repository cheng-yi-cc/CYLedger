import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { LedgerEntry } from '@/lib/mobile-ledger.ts';
export interface LedgerSearchFilters {
    start: string; end: string; minimum: string; maximum: string; remark: string; location: string;
    bookIds: string[]; accountIds: string[]; reimbursementIds: string[]; categoryIds: string[]; tagIds: string[];
    flags: string[]; matchAll: boolean;
}
export const ledgerSearchFlags: Record<string, string> = {
    expense: '支出', income: '收入', transfer: '转账', refund: '退款', reimbursement: '所有报销',
    pending: '待报销', completed: '已报销', discount: '有优惠', picture: '有附件', location: '有地点', excluded: '不计收支', untagged: '无标签'
};
export function emptyLedgerSearchFilters(): LedgerSearchFilters {
    return { start: '', end: '', minimum: '', maximum: '', remark: '', location: '', bookIds: [], accountIds: [], reimbursementIds: [], categoryIds: [], tagIds: [], flags: [], matchAll: false };
}
export function validateLedgerSearch(filters: LedgerSearchFilters): string {
    if (filters.start && filters.end && filters.start > filters.end) return '起始日期不能晚于截止日期。';
    for (const amount of [filters.minimum, filters.maximum]) if (amount && !/^\d{1,13}(\.\d{1,2})?$/.test(amount)) return '请输入有效金额，最多保留两位小数。';
    if (filters.minimum && filters.maximum && new LedgerDecimal(filters.minimum).gt(filters.maximum)) return '最低金额不能大于最高金额。';
    return '';
}
function hasFlag(item: LedgerEntry, flag: string): boolean {
    const reimbursement = !!item.reimbursementAccountId && item.reimbursementAccountId !== '0';
    switch (flag) {
        case 'expense': return item.type === 3;
        case 'income': return item.type === 2;
        case 'transfer': return item.type === 4;
        case 'refund': return item.type === 3 && new LedgerDecimal(item.amount).lt(0);
        case 'reimbursement': return reimbursement;
        case 'pending': return reimbursement && !item.reimbursementClosedAt;
        case 'completed': return reimbursement && !!item.reimbursementClosedAt;
        case 'discount': return new LedgerDecimal(item.discountAmount || '0').gt(0);
        case 'picture': return !!item.pictures?.length;
        case 'location': return !!item.location;
        case 'excluded': return !!item.excludeFromStatistics;
        case 'untagged': return !item.tagIds.length;
        default: return false;
    }
}
export function filterLedgerEntries(items: LedgerEntry[], query: string, filters: LedgerSearchFilters): LedgerEntry[] {
    if (validateLedgerSearch(filters)) return [];
    const tagNames = [...query.matchAll(/#([^\s#]+)/g)].map(match => match[1]!.toLocaleLowerCase());
    const terms = query.replace(/#([^\s#]+)/g, '').trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
    const remarkTerms = filters.remark.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
    return items.filter(item => {
        if (filters.start && item.day < filters.start || filters.end && item.day > filters.end) return false;
        const amount = new LedgerDecimal(item.amount).abs();
        if (filters.minimum && amount.lt(filters.minimum) || filters.maximum && amount.gt(filters.maximum)) return false;
        if (filters.bookIds.length && !filters.bookIds.includes(item.bookId)) return false;
        if (filters.accountIds.length && !filters.accountIds.some(id => id === item.accountId || id === item.destinationAccountId)) return false;
        if (filters.categoryIds.length && !filters.categoryIds.some(id => id === item.categoryId || id === item.primaryCategoryId)) return false;
        if (filters.reimbursementIds.length && !filters.reimbursementIds.includes(item.reimbursementAccountId || '')) return false;
        if (filters.tagIds.length && !filters.tagIds.some(id => item.tagIds.includes(id))) return false;
        if (!tagNames.every(name => item.tags.some(tag => tag.toLocaleLowerCase() === name))) return false;
        if (!remarkTerms.every(term => item.comment.toLocaleLowerCase().includes(term))) return false;
        if (filters.location && !item.location?.toLocaleLowerCase().includes(filters.location.toLocaleLowerCase())) return false;
        const text = [item.title, item.primaryCategory, item.comment, item.account, item.bookName, ...item.tags].join(' ').toLocaleLowerCase();
        if (!terms.every(term => text.includes(term))) return false;
        return !filters.flags.length || (filters.matchAll ? filters.flags.every(flag => hasFlag(item, flag)) : filters.flags.some(flag => hasFlag(item, flag)));
    });
}
export function ledgerFilterCount(filters: LedgerSearchFilters): number {
    return [filters.start || filters.end, filters.minimum || filters.maximum, filters.remark, filters.location, ...filters.bookIds, ...filters.accountIds, ...filters.categoryIds, ...filters.reimbursementIds, ...filters.tagIds, ...filters.flags].filter(Boolean).length;
}
