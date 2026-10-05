import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import { isUserLogined, isUserUnlocked } from '@/lib/userstate.ts';

export interface MonetaryBinding {
    id: string; accountId: string; code: string; name: string; startDate: string; nextDate: string;
    bookId: string; categoryId: string; timeZone: string; enabled: boolean; status: string; lastAttempt: number;
}
export interface MonetaryFund { name: string; symbol: string; providerId: string }
export type MonetaryInput = Pick<MonetaryBinding, 'accountId' | 'code' | 'startDate' | 'bookId' | 'categoryId' | 'timeZone' | 'enabled'>;
export type MonetaryDraft = Omit<MonetaryInput,'accountId'> & { fund:MonetaryFund };
export interface MonetarySync { created: number; bindings: MonetaryBinding[] }

async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> {
    const response = await axios.request<ApiResponse<T> & { errorMessage?: string }>({ method, url: `v1/monetary-income/${path}`, data });
    if (!response.data.success) throw new Error(response.data.errorMessage || '自动收益操作失败，请重试');
    return response.data.result;
}

export const monetaryIncome = {
    list: () => request<MonetaryBinding[]>('get', 'list'),
    search: (query: string) => request<MonetaryFund[]>('get', `search?q=${encodeURIComponent(query)}`),
    save: (input: MonetaryInput) => request<MonetaryBinding>('post', 'save', input),
    pause: (accountId: string) => request<boolean>('post', 'pause', { accountId }),
    async sync(accountId = '', force = false): Promise<MonetarySync> {
        const result = await request<MonetarySync>('post', 'sync', { accountId, force });
        if (result.created > 0) useTransactionsStore().updateStoreInvalidState({ transactionList: true, accountList: true, overview: true, statistics: true, explorer: true, reconciliationStatement: true });
        return result;
    }
};

let pending = false;
let lastAttempt = 0;
export async function syncMonetaryIncomeOnOpen(): Promise<void> {
    if (pending || document.hidden || !isUserLogined() || !isUserUnlocked() || Date.now() - lastAttempt < 60_000) return;
    pending = true; lastAttempt = Date.now();
    try { await monetaryIncome.sync(); }
    catch { /* The binding screen reports status and provides an explicit retry. */ }
    finally { pending = false; }
}
