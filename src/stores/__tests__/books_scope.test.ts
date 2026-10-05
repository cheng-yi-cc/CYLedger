import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import type { AxiosResponse } from 'axios';
import type { ApiResponse } from '@/core/api.ts';
import { DateRange } from '@/core/datetime.ts';
import { useBooksStore } from '../books.ts';
import { useTransactionsStore } from '../transaction.ts';
import { useStatisticsStore } from '../statistics.ts';
import { useOverviewStore } from '../overview.ts';
import { useExplorersStore } from '../explorer.ts';
import services from '@/lib/services.ts';

vi.hoisted(() => {
    const storage = { getItem: () => null, setItem: () => undefined, removeItem: () => undefined };
    vi.stubGlobal('localStorage', storage);
    vi.stubGlobal('sessionStorage', storage);
});
vi.mock('@/lib/services.ts', () => ({ default: {
    getTransactions: vi.fn(), getAllTransactionsByMonth: vi.fn(), getAllTransactions: vi.fn(),
    getTransactionStatistics: vi.fn(), getTransactionStatisticsTrends: vi.fn(),
    getTransactionAmounts: vi.fn(), getTransactionDailyAmounts: vi.fn()
} }));

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (cause: unknown) => void;
    const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
    return { promise, resolve, reject };
}
function response<T>(result: T): AxiosResponse<ApiResponse<T>> {
    return { data: { success: true, result } } as AxiosResponse<ApiResponse<T>>;
}

beforeEach(() => { setActivePinia(createPinia()); });

describe('book scope request races', () => {
    it('discards both late list results and late errors without clearing the latest scope', async () => {
        const books = useBooksStore(), transactions = useTransactionsStore();
        const lateResult = deferred<Awaited<ReturnType<typeof services.getTransactions>>>();
        const lateError = deferred<Awaited<ReturnType<typeof services.getTransactions>>>();
        vi.mocked(services.getTransactions).mockReturnValueOnce(lateResult.promise).mockReturnValueOnce(lateError.promise)
            .mockResolvedValueOnce(response({ items: [], nextTimeSequenceId: 88, totalCount: 0 }));
        books.selectedBookIds = ['book-a'];
        const options = { reload: true, autoExpand: true, defaultCurrency: 'CNY' };
        const oldResult = transactions.loadTransactions(options).catch(cause => cause);
        const oldError = transactions.loadTransactions(options).catch(cause => cause);
        books.selectedBookIds = ['book-b'];
        await transactions.loadTransactions(options);
        expect(transactions.transactionListStateInvalid).toBe(false);
        lateResult.resolve(response({ items: [], nextTimeSequenceId: 11, totalCount: 0 }));
        lateError.reject(new Error('old request failed'));
        expect(await oldResult).toMatchObject({ processed: true, isStaleScope: true });
        expect(await oldError).toMatchObject({ processed: true, isStaleScope: true });
        expect(transactions.transactionsNextTimeId).toBe(88);
        expect(transactions.transactionListStateInvalid).toBe(false);
    });

    it('keeps newer statistics after switching away and back to the original scope', async () => {
        const books = useBooksStore(), statistics = useStatisticsStore();
        const old = deferred<Awaited<ReturnType<typeof services.getTransactionStatistics>>>();
        const current = { startTime: 20, endTime: 30, items: [] };
        vi.mocked(services.getTransactionStatistics).mockReturnValueOnce(old.promise).mockResolvedValueOnce(response(current));
        books.selectedBookIds = ['book-a'];
        const pending = statistics.loadCategoricalAnalysis({ force: false }).catch(cause => cause);
        books.selectedBookIds = ['book-b'];
        books.selectedBookIds = ['book-a'];
        await statistics.loadCategoricalAnalysis({ force: false });
        old.resolve(response({ startTime: 1, endTime: 2, items: [] }));
        expect(await pending).toMatchObject({ isStaleScope: true });
        expect(statistics.transactionCategoryStatisticsData).toEqual(current);
        expect(statistics.transactionStatisticsStateInvalid).toBe(false);
    });

    it('invalidates nested overview caches even while its main summary remains invalid', async () => {
        const books = useBooksStore(), overview = useOverviewStore();
        const first = { startTime: 1, endTime: 2, items: [] };
        const second = { startTime: 3, endTime: 4, items: [] };
        vi.mocked(services.getTransactionStatistics).mockResolvedValueOnce(response(first)).mockResolvedValueOnce(response(second));
        books.selectedBookIds = ['book-a'];
        await overview.loadTransactionCategoryStatistics({ force: false, dateType: DateRange.ThisMonth.type });
        expect(overview.transactionOverviewStateInvalid).toBe(true);
        books.selectedBookIds = ['book-b'];
        expect(await overview.loadTransactionCategoryStatistics({ force: false, dateType: DateRange.ThisMonth.type })).toEqual(second);
        expect(services.getTransactionStatistics).toHaveBeenCalledTimes(2);
    });

    it('leaves explorer invalid after an obsolete request so a new scope can reload', async () => {
        const books = useBooksStore(), explorers = useExplorersStore();
        const old = deferred<Awaited<ReturnType<typeof services.getAllTransactions>>>();
        vi.mocked(services.getAllTransactions).mockReturnValueOnce(old.promise).mockResolvedValueOnce(response([]));
        books.selectedBookIds = ['book-a'];
        const pending = explorers.loadAllTransactions({ force: false }).catch(cause => cause);
        books.selectedBookIds = ['book-b'];
        old.resolve(response([]));
        expect(await pending).toMatchObject({ isStaleScope: true });
        expect(explorers.transactionExplorerStateInvalid).toBe(true);
        await explorers.loadAllTransactions({ force: false });
        expect(explorers.transactionExplorerStateInvalid).toBe(false);
    });
});
