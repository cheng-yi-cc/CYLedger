import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
export interface CalendarEvent {
    id: string; transactionId?: string; bookId: string; accountId: string; accountName: string; currency: string;
    kind: 'repayment' | 'deposit'; date: string; amount: string; note: string; completed: boolean;
}
export type CalendarEventInput = Pick<CalendarEvent, 'id' | 'bookId' | 'accountId' | 'kind' | 'date' | 'amount' | 'note'>;
async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> {
    const response = await axios.request<ApiResponse<T>>({ method, url: `v1/calendar/${path}`, data });
    if (!response.data.success) throw new Error('到期事项操作失败，请重试。');
    if(method==='post'){void import('@/lib/asset-tools.ts').then(async({syncAssetAutomationOnOpen})=>{const{useLedgerScopeStore}=await import('@/stores/ledgerScope.ts');await syncAssetAutomationOnOpen(useLedgerScopeStore().timeZone,true);}).catch(()=>{});}
    return response.data.result;
}
export const calendarEvents = {
    list: (month: string) => request<CalendarEvent[]>('get', `list?month=${encodeURIComponent(month)}`),
    save: (input: CalendarEventInput) => request<CalendarEvent>('post', 'save', input),
    complete: (id: string, completed: boolean) => request<boolean>('post', 'complete', { id, completed }),
    remove: (id: string) => request<boolean>('post', 'delete', { id })
};
