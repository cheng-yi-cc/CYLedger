import axios from 'axios';
import '@/lib/services.ts'; // Reuse the application's authentication, API root and token refresh.
import type { ApiResponse } from '@/core/api.ts';
import type { InvestmentAccount, Instrument, InvestmentEvent, InvestmentSettings, InvestmentPosition, InvestmentPreview, WealthSummary, WealthSnapshot } from '@/models/investment.ts';

async function request<T>(method: 'get' | 'post', path: string, data?: unknown, key?: string): Promise<T> {
    const response = await axios.request<ApiResponse<T> & { errorMessage?: string; error?: string }>({
        method, url: `v1/${path}`, data, headers: key ? { 'Idempotency-Key': key } : undefined
    });
    if (!response.data.success) throw new Error(response.data.errorMessage || response.data.error || '请求失败，请稍后重试');
    return response.data.result;
}

export const investments = {
    accounts: () => request<InvestmentAccount[]>('get', 'investments/accounts'),
    createAccount: (name: string, kind: string) => request<InvestmentAccount>('post', 'investments/accounts', { name, kind }),
    instruments: () => request<Instrument[]>('get', 'investments/instruments'),
    createInstrument: (data: { name: string; symbol: string; type: string }) => request<Instrument>('post', 'investments/instruments', data),
    settings: () => request<InvestmentSettings>('get', 'investments/settings'),
    saveSettings: (data: InvestmentSettings) => request<InvestmentSettings>('post', 'investments/settings', data),
    positions: () => request<InvestmentPosition[]>('get', 'investments/positions'),
    events: () => request<InvestmentEvent[]>('get', 'investments/events'),
    preview: (data: InvestmentEvent) => request<InvestmentPreview>('post', 'investments/events/preview', data),
    saveEvent: (data: InvestmentEvent, key: string) => data.id
        ? request<InvestmentEvent>('post', `investments/events/${encodeURIComponent(data.id)}/revise`, data)
        : request<InvestmentEvent>('post', 'investments/events', data, key),
    voidEvent: (data: InvestmentEvent) => request<InvestmentEvent>('post', `investments/events/${encodeURIComponent(data.id)}/void`, { version: data.version }),
    summary: () => request<WealthSummary>('get', 'wealth/summary'),
    history: () => request<WealthSnapshot[]>('get', 'wealth/history'),
    manualQuote: (instrumentId: string, price: string, asOf: number) => request<unknown>('post', 'investments/quotes/manual', { instrumentId, price, asOf }),
    automaticQuote: (instrumentId: string) => request<unknown>('post', 'investments/quotes/manual', { instrumentId, automatic: true }),
    export: () => request<string>('get', 'investments/export')
};

export function investmentError(error: unknown): string {
    if (axios.isAxiosError(error)) return error.response?.data?.errorMessage || error.response?.data?.error || (error.response ? error.message : '无法连接服务器。请检查网络后重试，尚未确认入账。');
    return error instanceof Error ? error.message : '操作失败，请重试';
}
