import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';

export type StatisticsPeriod = 'daily' | 'month' | 'year' | 'custom';
export interface StatisticsModule { id: string; visible: boolean }
export interface StatisticsPreferences {
    revision: string; modules: Record<StatisticsPeriod, StatisticsModule[]>;
    carrySurplus: boolean; carryDeficit: boolean;
    dailyBudgetMode: 'remaining' | 'fixed'; budgetProgress: 'remaining' | 'spent';
}
export interface StatisticsBudget {
    id: string; bookId: string; categoryId: string; name: string; amount: string;
    startDate: string; endDate: string; kind: 'monthly' | 'custom'; repeat: boolean; revision: string;
}
export interface StatisticsNote { id: string; bookId: string; period: string; content: string; revision: string }
export interface StatisticsAuxiliary { debtActions: Record<string, string>; feeIds: string[] }
export const statisticsModuleNames: Record<string, string> = {
    cash: '收支统计', assets: '资产汇总', budget: '预算占比', wealth: '资产走势', heatmap: '收支热力',
    flow: '收支对比', categories: '分类占比', ranking: '分类数据', report: '报表统计', accounts: '账户收支',
    tagShare: '标签占比', tags: '标签数据', note: '总结'
};
export function defaultStatisticsPreferences(): StatisticsPreferences {
    const ids = {
        daily: ['cash', 'assets', 'budget', 'tags'],
        month: ['cash', 'wealth', 'flow', 'categories', 'ranking', 'report', 'accounts', 'tagShare', 'tags', 'note'],
        year: ['cash', 'heatmap', 'wealth', 'flow', 'categories', 'ranking', 'report', 'accounts', 'tagShare', 'tags', 'note'],
        custom: ['cash', 'wealth', 'flow', 'categories', 'ranking', 'report', 'accounts', 'tagShare', 'tags']
    };
    return { revision: '0', modules: Object.fromEntries(Object.entries(ids).map(([key, value]) => [key, value.map(id => ({ id, visible: true }))])) as StatisticsPreferences['modules'], carrySurplus: false, carryDeficit: false, dailyBudgetMode: 'remaining', budgetProgress: 'remaining' };
}
async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> {
    const response = await axios.request<ApiResponse<T> & { errorMessage?: string }>({ method, url: `v1/statistics/${path}`, data });
    if (!response.data.success) throw Error(response.data.errorMessage || '统计数据加载失败，请重试');
    return response.data.result;
}
export const statisticsWorkspace = {
    preferences: () => request<StatisticsPreferences>('get', 'preferences'),
    savePreferences: (input: StatisticsPreferences) => request<StatisticsPreferences>('post', 'preferences', input),
    budgets: () => request<StatisticsBudget[]>('get', 'budgets'),
    saveBudget: (input: StatisticsBudget) => request<StatisticsBudget>('post', 'budgets/save', input),
    deleteBudget: (input: Pick<StatisticsBudget, 'id' | 'revision'>) => request<boolean>('post', 'budgets/delete', input),
    notes: () => request<StatisticsNote[]>('get', 'notes'),
    saveNote: (input: StatisticsNote) => request<StatisticsNote>('post', 'notes/save', input),
    auxiliary: () => request<StatisticsAuxiliary>('get', 'auxiliary')
};
