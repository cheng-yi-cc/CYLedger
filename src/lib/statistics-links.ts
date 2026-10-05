import type { LedgerEntry } from '@/lib/mobile-ledger.ts';
import { tagGroupIds, statisticsEligible, type StatisticsTag, type TagStatisticsMode } from '@/lib/statistics-report.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { StatisticsAuxiliary } from '@/lib/statistics-workspace.ts';
export function statisticsLink(start: number, end: number, query: Record<string, string> = {}): string {
    return `/ledger/details?${new URLSearchParams({ start: String(Math.max(0,start)), end: String(end), ...query }).toString()}`;
}
export function filterStatisticsEntries(entries: LedgerEntry[], query: Record<string, string>, tags: Record<string, StatisticsTag>, auxiliary?: StatisticsAuxiliary): LedgerEntry[] {
    return entries.filter(item => {
        if ((query['filterAccountId'] && item.accountId !== query['filterAccountId'] && item.destinationAccountId !== query['filterAccountId']) ||
            (query['filterCategoryId'] && item.categoryId !== query['filterCategoryId'] && item.primaryCategoryId !== query['filterCategoryId']) ||
            (query['filterTagId'] && !item.tagIds.includes(query['filterTagId'])) ||
            (query['filterType'] && item.type !== Number(query['filterType']))) return false;
        if (query['aux'] === 'transfer') { if (item.type !== 4 || item.investment) return false; }
        else if (query['aux'] === 'refund') { if (!statisticsEligible(item) || !item.amount.startsWith('-')) return false; }
        else if (query['aux'] === 'discount') { if (!statisticsEligible(item) || !item.discountAmount || new LedgerDecimal(item.discountAmount).isZero()) return false; }
        else if (query['aux'] === 'debt') { if (auxiliary?.debtActions[item.id] !== query['debtAction']) return false; }
        else if (query['aux'] === 'fees') { if (!auxiliary?.feeIds.includes(item.id)) return false; }
        else if (!statisticsEligible(item)) return false;
        return (!query['type'] || item.type === Number(query['type'])) &&
            (!query['accountId'] || item.accountId === query['accountId'] || item.destinationAccountId === query['accountId']) &&
            (!query['categoryId'] || item.categoryId === query['categoryId'] || (query['primary'] !== 'false' && item.primaryCategoryId === query['categoryId'])) &&
            (!query['tagId'] || tagGroupIds(item.tagIds, tags, query['tagMode'] as TagStatisticsMode || 'all').includes(query['tagId']));
    });
}
