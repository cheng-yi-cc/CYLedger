import type { Instrument, InvestmentQuote } from '@/models/investment.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';

const presetInstrumentIds = new Set(['crypto:bitcoin', 'crypto:ethereum', 'crypto:solana', 'crypto:tether', 'crypto:usd-coin']);

export function isPresetInstrument(instrument: Instrument): boolean {
    return presetInstrumentIds.has(instrument.id);
}

export function instrumentMarketLabel(instrument: Instrument): string {
    if (isPresetInstrument(instrument)) return '预置加密资产';
    const markets: Record<string, string> = { CRYPTO: '加密资产', CN_SH: '上海证券市场', CN_SZ: '深圳证券市场', CN_BJ: '北京证券市场', CN_FUND: '场外基金', HK: '香港证券市场', US: '美国证券市场' };
    return instrument.market ? markets[instrument.market] || instrument.market : '自定义资产';
}

export function quoteStatus(quote?: InvestmentQuote): string {
    if (!quote || quote.state === 'unavailable' || quote.state === 'missing') return '缺少报价';
    if (quote.fxState === 'stale') return '折算汇率已过期';
    if (quote.currency !== 'CNY' && !quote.fxRate) return '缺少折算汇率';
    return ({ live: '实时参考报价', realtime: '实时参考报价', fresh: '最新参考报价', delayed: '延迟参考报价', manual: '手动估值', stale: '报价已过期', unavailable: '缺少报价', missing: '缺少报价' } as Record<string,string>)[quote.state.toLowerCase()] || '参考报价';
}

export function investmentDecimalText(value: string | null | undefined, places = 4): string {
    return value == null || value === '' ? '—' : new LedgerDecimal(value).toDecimalPlaces(places).toString();
}

export function investmentProfitClass(value: string | null | undefined): string {
    if (value == null || value === '') return '';
    const amount = new LedgerDecimal(value);
    return amount.gt(0) ? 'inv-up' : amount.lt(0) ? 'inv-down' : '';
}
export function quoteChangeLabel(quote?: InvestmentQuote): string {
    return quote?.changePeriod === '24h' ? '24 小时涨跌' : quote?.changePeriod === 'nav' ? '净值涨跌' : '当日报价涨跌';
}
export function quoteChange(quote?: InvestmentQuote): string {
    if (quote?.changePercent === undefined || quote.changePercent === null || quote.changePercent === '') return '—';
    const value = new LedgerDecimal(quote.changePercent);
    return `${value.gt(0) ? '+' : ''}${value.toFixed(2)}%`;
}
