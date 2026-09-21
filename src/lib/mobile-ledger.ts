import { ref } from 'vue';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
export { LedgerDecimal, ledgerMoney, ledgerTotals, ledgerSignedAmount } from '@/lib/ledger-display.ts';
import moment from 'moment-timezone';
import services from '@/lib/services.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { investmentError } from '@/lib/investments.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';
import type { TransactionInfoResponse } from '@/models/transaction.ts';

export interface LedgerEntry {
    id: string; type: number; day: string; time: number; title: string; account: string;
    currency: string; amount: string; cny: string | null; hidden: boolean;
    categoryId: string; primaryCategory: string; comment: string; tags: string[];
}
export function useMobileLedger() {
    const accounts = useAccountsStore();
    const categories = useTransactionCategoriesStore();
    const tags = useTransactionTagsStore();
    const exchange = useExchangeRatesStore();
    const entries = ref<LedgerEntry[]>([]);
    const loading = ref(true);
    const error = ref('');
    let requestNumber = 0;

    function normalize(item: TransactionInfoResponse): LedgerEntry {
        const account = accounts.allAccountsMap[item.sourceAccountId] || item.sourceAccount;
        const destination = accounts.allAccountsMap[item.destinationAccountId] || item.destinationAccount;
        const category = categories.allTransactionCategoriesMap[item.categoryId] || item.category;
        const parent = category?.parentId ? categories.allTransactionCategoriesMap[category.parentId] : undefined;
        const currency = account?.currency || '';
        const amount = new LedgerDecimal(item.sourceAmount.toString()).div(100).toString();
        const converted = currency === 'CNY' ? amount : currency
            ? exchange.getExchangedAmount(parseBigDecimal(amount), currency, 'CNY')?.toString() ?? null : null;
        return {
            id: item.id, type: item.type, day: moment.unix(item.time).format('YYYY-MM-DD'), time: item.time,
            title: category?.name || ({ 1: '余额调整', 2: '收入', 3: '支出', 4: '账户转账' }[item.type] || '账户转账'),
            primaryCategory: parent?.name || category?.name || '未分类', categoryId: item.categoryId,
            account: `${account?.name || '账户'}${item.type === 4 && destination ? ` → ${destination.name}` : ''}`,
            currency, amount, cny: converted, hidden: item.hideAmount, comment: item.comment,
            tags: (item.tagIds || []).map(id => tags.allTransactionTagsMap[id]?.name).filter((name): name is string => !!name)
        };
    }
    async function load(startTime: number, endTime: number): Promise<void> {
        const version = ++requestNumber;
        loading.value = true;
        error.value = '';
        try {
            const [, , , response] = await Promise.all([
                accounts.loadAllAccounts({ force: true }).catch(keepUpToDate), categories.loadAllCategories({ force: false }).catch(keepUpToDate),
                tags.loadAllTags({ force: false }).catch(keepUpToDate), services.getAllTransactions({ startTime, endTime })
            ]);
            if (version !== requestNumber) return;
            if (!response.data.success) throw new Error('账单加载失败，请重试。');
            const items = response.data.result;
            if (items.some(item => (accounts.allAccountsMap[item.sourceAccountId]?.currency || item.sourceAccount?.currency) !== 'CNY')) {
                // A failed FX request must not turn missing amounts into zero.
                await exchange.getLatestExchangeRates({ silent: true, force: false }).catch(() => undefined);
            }
            if (version !== requestNumber) return;
            entries.value = items.map(normalize).sort((a, b) => b.time - a.time || b.id.localeCompare(a.id));
        } catch (cause) {
            if (version === requestNumber) { entries.value = []; error.value = investmentError(cause); }
        } finally { if (version === requestNumber) loading.value = false; }
    }
    return { entries, loading, error, load };
}
// Legacy stores report an unchanged force refresh by rejecting with this marker.
export function keepUpToDate(error: unknown): void {
    if (typeof error === 'object' && error !== null && 'isUpToDate' in error && error.isUpToDate === true) return;
    throw error;
}
