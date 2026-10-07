import { shallowRef } from 'vue';
import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import { getCurrentUserInfo, isUserLogined, isUserUnlocked } from '@/lib/userstate.ts';

export interface CryptoDCAPlan {
    id: string; accountId: string; instrumentId: string; paymentInstrumentId: string; amount: string;
    dailyTime: string; startDate: string; nextDate: string; timeZone: string; bookId: string;
    enabled: boolean; revision: number; status: string; lastAttempt: number;
}
export interface CryptoDCADay {
    id: string; planId: string; date: string; scheduledAt: number; accountId: string;
    instrumentId: string; paymentInstrumentId: string; amount: string;
    status: 'pending' | 'done' | 'skipped'; message: string; lastAttempt: number; eventId: string;
}
export interface CryptoDCAAlert {
    accountId: string; accountName: string; paymentInstrumentId: string; balance: string; required: string; paused: boolean;
}
export interface CryptoDCAState { plans: CryptoDCAPlan[]; days: CryptoDCADay[]; alerts: CryptoDCAAlert[]; created: number }
export interface CryptoDCAInput {
    id: string; revision: number; requestKey: string; accountId: string; instrumentId: string; paymentInstrumentId: string;
    amount: string; dailyTime: string; startDate: string; timeZone: string; bookId: string;
}
export const dcaSymbols: Record<string, string> = {
    'crypto:bitcoin': 'BTC', 'crypto:ethereum': 'ETH', 'crypto:solana': 'SOL',
    'crypto:tether': 'USDT', 'crypto:usd-coin': 'USDC'
};
export const cryptoDCAState = shallowRef<CryptoDCAState>();
async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> {
    const response = await axios.request<ApiResponse<T> & { errorMessage?: string }>({ method, url: `v1/investments/dca${path}`, data, timeout: 30000 });
    if (!response.data.success) throw Error(response.data.errorMessage || '定投操作失败，请重试');
    return response.data.result;
}
let syncing: Promise<CryptoDCAState> | undefined;
let lastAttempt = 0;
let lastOwner = '';
function owner(): string { return getCurrentUserInfo()?.username || ''; }
function accept(state: CryptoDCAState, requestedOwner: string): CryptoDCAState {
    if (requestedOwner !== owner() || !isUserUnlocked()) return state;
    cryptoDCAState.value = state;
    if (state.created > 0) window.dispatchEvent(new CustomEvent('cy-dca-updated'));
    return state;
}
export const cryptoDCA = {
    async list(): Promise<CryptoDCAState> { const requestedOwner=owner(); return accept(await request<CryptoDCAState>('get', ''),requestedOwner); },
    save: (data: CryptoDCAInput) => request<CryptoDCAPlan>('post', '/save', data),
    enabled: (plan: CryptoDCAPlan, enabled: boolean) => request<CryptoDCAPlan>('post', '/enabled', { id: plan.id, revision: plan.revision, enabled }),
    sync(force = false): Promise<CryptoDCAState> {
        if (syncing) return syncing;
        lastAttempt = Date.now();
        lastOwner=owner(); const requestedOwner=lastOwner;
        syncing = request<CryptoDCAState>('post', '/sync', { force }).then(state=>accept(state,requestedOwner)).finally(() => { syncing = undefined; });
        return syncing;
    }
};
export async function syncCryptoDCAOnOpen(): Promise<void> {
    if (document.hidden || !isUserLogined() || !isUserUnlocked()) { cryptoDCAState.value = undefined; return; }
    if (lastOwner===owner() && Date.now() - lastAttempt < 60000) return;
    try { await cryptoDCA.sync(); }
    catch { /* Saved pending occurrences are shown with retry on the plan page. */ }
}
