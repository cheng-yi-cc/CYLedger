import axios from 'axios';
import type { AccountDeletionTarget, AccountDeletionPreview } from '@/models/investment.ts';
import type { HoldingProfile,HoldingSetup,InvestmentPlan,InvestmentOrder,InvestmentReport } from '@/models/investment.ts';
import '@/lib/services.ts'; // Reuse the application's authentication, API root and token refresh.
import type { ApiResponse } from '@/core/api.ts';
import type { InvestmentAccount, Instrument, InstrumentBinding, InstrumentCandidate, InvestmentEvent, InvestmentSettings, InvestmentPosition, InvestmentPreview, InvestmentQuote, WealthSummary, WealthSnapshot, ConversionInput, InvestmentConversion } from '@/models/investment.ts';

async function request<T>(method: 'get' | 'post', path: string, data?: unknown, key?: string, timeout?: number): Promise<T> {
    const response = await axios.request<ApiResponse<T> & { errorMessage?: string; error?: string }>({
        method, url: `v1/${path}`, data, headers: key ? { 'Idempotency-Key': key } : undefined, ...(timeout ? { timeout } : {})
    });
    if (!response.data.success) throw new Error(response.data.errorMessage || response.data.error || '请求失败，请稍后重试');
    return response.data.result;
}

export const investments = {
    profiles:()=>request<HoldingProfile[]>('get','investments/holdings'),
    saveProfile:(data:HoldingProfile)=>request<HoldingProfile>('post','investments/holdings',data),
    setupHolding:(data:HoldingSetup,key:string)=>request<HoldingProfile>('post','investments/holdings/setup',data,key,45000),
    updateHolding:(data:HoldingSetup,key:string)=>request<HoldingProfile>('post','investments/holdings/update',data,key),
    plans:()=>request<InvestmentPlan[]>('get','investments/plans'),
    savePlan:(data:InvestmentPlan)=>request<InvestmentPlan>('post','investments/plans',data),
    orders:()=>request<InvestmentOrder[]>('get','investments/orders'),
    saveOrder:(data:InvestmentOrder,key:string)=>request<InvestmentOrder>('post','investments/orders',data,key),
    confirmOrder:(data:{id:string;version:number;price:string;date:string})=>request<InvestmentPreview>('post','investments/orders/confirm',data),
    cancelOrder:(data:{id:string;version:number})=>request<boolean>('post','investments/orders/cancel',data),
    syncPlans:(force=false)=>request<{created:number;pending:number}>('post','investments/plans/sync',{force},undefined,120000),
    report:(accountId='',instrumentId='')=>request<InvestmentReport>('get',`investments/report?${new URLSearchParams({accountId,instrumentId})}`),
    previewAccountDeletion: (target: AccountDeletionTarget) => request<AccountDeletionPreview>('post', 'wealth/accounts/delete/preview', target),
    deleteAccount: (target: AccountDeletionTarget, token: string, deleteRelated: boolean) => request<AccountDeletionPreview>('post', 'wealth/accounts/delete', { ...target, token, deleteRelated }),
    accounts: () => request<InvestmentAccount[]>('get', 'investments/accounts'),
    createAccount: (name: string, kind: string) => request<InvestmentAccount>('post', 'investments/accounts', { name, kind }),
    updateAccount: (data:InvestmentAccount) => request<InvestmentAccount>('post','investments/accounts/update',data),
    createCryptoAccount: (data: { name:string; kind:string; platform:string; bookId:string; currency?:string; paymentInstruments?:string[]; holdings:{instrumentId:string;quantity:string}[] }, key:string) => request<InvestmentAccount>('post', 'investments/accounts/crypto',data,key),
    conversion: (data:ConversionInput) => request<InvestmentConversion>('post','investments/conversion',data,undefined,45000),
    instruments: () => request<Instrument[]>('get', 'investments/instruments'),
    createInstrument: (data: { name: string; symbol: string; type: string } & Partial<InstrumentBinding>) => request<Instrument>('post', 'investments/instruments', data),
    // 基金提供方允许 20 秒连接预算，首次查询不能被默认 10 秒前端超时提前取消。
    searchInstruments: (query: string, market = '') => request<InstrumentCandidate[]>('get', `investments/instruments/search?q=${encodeURIComponent(query)}&market=${encodeURIComponent(market)}`, undefined, undefined, market === 'CN_FUND' ? 30000 : undefined),
    bindInstrument: (instrumentId: string, binding: InstrumentBinding) => request<Instrument>('post', 'investments/instruments/bind', { instrumentId, ...binding }),
    settings: () => request<InvestmentSettings>('get', 'investments/settings'),
    saveSettings: (data: InvestmentSettings) => request<InvestmentSettings>('post', 'investments/settings', data),
    positions: () => request<InvestmentPosition[]>('get', 'investments/positions'),
    quotes: () => request<InvestmentQuote[]>('get', 'investments/quotes'),
    events: () => request<InvestmentEvent[]>('get', 'investments/events'),
    preview: (data: InvestmentEvent) => request<InvestmentPreview>('post', 'investments/events/preview', data),
    saveEvent: (data: InvestmentEvent, key: string) => data.id
        ? request<InvestmentEvent>('post', `investments/events/${encodeURIComponent(data.id)}/revise`, data)
        : request<InvestmentEvent>('post', 'investments/events', data, key),
    voidEvent: (data: InvestmentEvent) => request<InvestmentEvent>('post', `investments/events/${encodeURIComponent(data.id)}/void`, { version: data.version }),
    summary: () => request<WealthSummary>('get', 'wealth/summary'),
    history: () => request<WealthSnapshot[]>('get', 'wealth/history'),
    manualQuote: (instrumentId: string, price: string, asOf: number, currency: 'CNY' | 'USD' = 'CNY') => request<unknown>('post', 'investments/quotes/manual', { instrumentId, price, asOf, currency }),
    automaticQuote: (instrumentId: string) => request<unknown>('post', 'investments/quotes/manual', { instrumentId, automatic: true }),
    export: () => request<string>('get', 'investments/export')
};

export function investmentError(error: unknown): string {
    if (axios.isAxiosError(error)) return error.response?.data?.errorMessage || error.response?.data?.error || (error.response ? error.message : '无法连接服务器。请检查网络后重试，尚未确认入账。');
    return error instanceof Error ? error.message : '操作失败，请重试';
}
