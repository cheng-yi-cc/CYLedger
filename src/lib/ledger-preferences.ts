import moment from 'moment-timezone';

export type LedgerTab = 'home' | 'calendar' | 'assets' | 'statistics' | 'settings';
export type LedgerCardKind = 'income' | 'expense' | 'balance' | 'budget' | 'budgetCompact' | 'asset' | 'wish' | 'count' | 'credit' | 'debt' | 'assetTrend' | 'category' | 'tag' | 'gold' | 'heatmap' | 'flow' | 'line' | 'rank' | 'repayment' | 'reimbursement' | 'sankey' | 'statistics' | 'week';
export interface LedgerCard {
    id: string; kind: LedgerCardKind; title: string; period: 'today' | 'week' | 'month' | 'year' | 'all';
    bookIds: string[]; accountIds: string[]; categoryIds: string[]; tagIds: string[]; wishId: string;
    width: 2 | 3 | 4 | 6; color: string; background: string; image?: string; opacity?: number;
}
export interface LedgerPreferences {
    version: number; view: 'simple' | 'detail'; range: 'all' | 'month';
    remarkFirst: boolean; simpleCategory: boolean; showTime: boolean; showOriginal: boolean;
    deleteImportedScreenshots: boolean; showAccountIcon: boolean; lineIcons: boolean; multiCurrency: boolean;
    navigationText: boolean; navigationStyle: 'floating' | 'glass' | 'fixed'; tabs: LedgerTab[];
    hideHero: boolean; hideAdd: boolean; monthStart: number; defaultAccountId: string;
    categoryAccounts: Record<string, string>; historicalRemarks: boolean; lowBalance: boolean;
    vibration: boolean; keyboard: 'ascending' | 'descending'; pullAction: 'add' | 'refresh' | 'none';
    floatPosition: 'left' | 'right' | 'hidden'; floatAction: 'add' | 'import';
    exitConfirm: boolean; hideRecents: boolean; colorScheme: 'red-expense' | 'green-expense' | 'monochrome';
    accent: string; nightSchedule: boolean; nightStart: string; nightEnd: string;
    specialIcons: Record<string, string>; cards: LedgerCard[];
}
export const ledgerTabNames: Record<LedgerTab, string> = { home: '首页', calendar: '日历', assets: '资产', statistics: '统计', settings: '我的' };
export const ledgerCardNames: Record<LedgerCardKind, string> = { income: '收入', expense: '支出', balance: '结余', budget: '预算明细', budgetCompact: '剩余预算', asset: '资产总览', wish: '愿望进度', count: '账单笔数', credit:'信用卡', debt:'债务', assetTrend:'资产趋势', category:'分类统计', tag:'标签统计', gold:'黄金行情', heatmap:'消费热力图', flow:'现金流', line:'收支趋势', rank:'消费排行', repayment:'待还款', reimbursement:'待报销', sankey:'收支流向', statistics:'收支统计', week:'一周收支' };
export function defaultLedgerPreferences(): LedgerPreferences {
    return {
        version: 1, view: 'detail', range: 'all', remarkFirst: false, simpleCategory: false,
        deleteImportedScreenshots: false, showTime: false, showOriginal: false, showAccountIcon: false, lineIcons: true, multiCurrency: true,
        navigationText: true, navigationStyle: 'floating', tabs: ['home', 'calendar', 'assets', 'statistics', 'settings'],
        hideHero: false, hideAdd: false, monthStart: 1, defaultAccountId: 'category', categoryAccounts: {},
        historicalRemarks: true, lowBalance: true, vibration: false, keyboard: 'ascending', pullAction: 'add',
        floatPosition: 'hidden', floatAction: 'add', exitConfirm: true, hideRecents: false,
        colorScheme: 'red-expense', accent: '#087f72', nightSchedule: false, nightStart: '20:00', nightEnd: '07:00',
        specialIcons: { transfer: 'arrow_right_arrow_left', repayment: 'creditcard', borrow: 'arrow_down_left', lend: 'arrow_up_right', adjustment: 'equal_circle' }, cards: []
    };
}
export function parseLedgerPreferences(value: unknown): LedgerPreferences {
    const defaults = defaultLedgerPreferences();
    if (!value || typeof value !== 'object' || Array.isArray(value)) return defaults;
    const source = value as Partial<LedgerPreferences> & Record<string, unknown>;
    const result = { ...defaults };
    for (const key of Object.keys(defaults) as (keyof LedgerPreferences)[]) {
        if (typeof defaults[key] === 'boolean' && typeof source[key] === 'boolean') (result as unknown as Record<string, unknown>)[key] = source[key];
    }
    const choices = {
        view: ['simple', 'detail'], range: ['all', 'month'], navigationStyle: ['floating', 'glass', 'fixed'],
        keyboard: ['ascending', 'descending'], pullAction: ['add', 'refresh', 'none'], floatPosition: ['left', 'right', 'hidden'],
        floatAction: ['add', 'import'], colorScheme: ['red-expense', 'green-expense', 'monochrome']
    };
    for (const [key, allowed] of Object.entries(choices)) if (allowed.includes(String(source[key]))) (result as unknown as Record<string, unknown>)[key] = source[key];
    if (Number.isInteger(source.monthStart) && Number(source.monthStart) >= 1 && Number(source.monthStart) <= 28) result.monthStart = Number(source.monthStart);
    if (typeof source.defaultAccountId === 'string' && /^(category|last|[0-9]{1,20})$/.test(source.defaultAccountId)) result.defaultAccountId = source.defaultAccountId;
    if (typeof source.accent === 'string' && /^#[0-9a-f]{6}$/i.test(source.accent)) result.accent = source.accent;
    for (const key of ['nightStart', 'nightEnd'] as const) if (typeof source[key] === 'string' && /^([01]\d|2[0-3]):[0-5]\d$/.test(source[key])) result[key] = source[key];
    if (Array.isArray(source.tabs)) {
        result.tabs = [...new Set(source.tabs.filter((tab): tab is LedgerTab => typeof tab === 'string' && Object.hasOwn(ledgerTabNames, tab)))];
        if (!result.tabs.includes('home')) result.tabs.unshift('home');
        if (!result.tabs.includes('settings')) result.tabs.push('settings');
    }
    if (source.categoryAccounts && typeof source.categoryAccounts === 'object' && !Array.isArray(source.categoryAccounts)) {
        result.categoryAccounts = Object.fromEntries(Object.entries(source.categoryAccounts).filter(([key, val]) => /^[0-9]{1,20}$/.test(key) && typeof val === 'string' && /^[0-9]{1,20}$/.test(val)).slice(0, 2000));
    }
    if (source.specialIcons && typeof source.specialIcons === 'object') for (const key of Object.keys(defaults.specialIcons)) {
        const icon = (source.specialIcons as Record<string, unknown>)[key];
        if (typeof icon === 'string' && /^[a-z_0-9]{1,64}$/.test(icon)) result.specialIcons[key] = icon;
    }
    if (Array.isArray(source.cards)) result.cards = source.cards.filter((card): card is LedgerCard => !!card && typeof card === 'object' && typeof card.id === 'string' && card.id.length <= 64 && Object.hasOwn(ledgerCardNames, card.kind)).slice(0, 20).map(card => ({
        id: card.id, kind: card.kind, title: String(card.title || '').slice(0, 32), period: ['today', 'week', 'month', 'year', 'all'].includes(card.period) ? card.period : 'month',
        ...Object.fromEntries(['bookIds', 'accountIds', 'categoryIds', 'tagIds'].map(key => [key, Array.isArray(card[key as keyof LedgerCard]) ? (card[key as keyof LedgerCard] as unknown[]).filter((id): id is string => typeof id === 'string' && id.length <= 64).slice(0, 100) : []])),
        wishId: String(card.wishId || '').slice(0, 64), width: [2,3,4,6].includes(card.width) ? card.width : 3,
        image: typeof card.image==='string'&&card.image.length<=70000&&/^data:image\/(jpeg|webp);base64,[A-Za-z0-9+/=]+$/.test(card.image)?card.image:'', opacity: typeof card.opacity==='number'?Math.min(1,Math.max(.15,card.opacity)):1,
        color: /^#[0-9a-f]{6}$/i.test(card.color) ? card.color : '', background: /^#[0-9a-f]{6}$/i.test(card.background) ? card.background : ''
    } as LedgerCard));
    return result;
}

// Calendar month labels remain stable while an accounting month can start later.
export function ledgerMonthRange(month: string, timeZone: string, startDay = 1) {
    const start = moment.tz(month, 'YYYY-MM', true, timeZone).date(Math.max(1, Math.min(28, startDay))).startOf('day');
    return { start, end: start.clone().add(1, 'month').subtract(1, 'second') };
}
export function ledgerAccountingMonth(now: moment.Moment, startDay = 1): string {
    return now.clone().subtract(now.date() < startDay ? 1 : 0, 'month').format('YYYY-MM');
}
