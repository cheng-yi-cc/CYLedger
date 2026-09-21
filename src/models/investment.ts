export interface InvestmentAccount { id: string; name: string; kind: string }
export interface Instrument { id: string; type: 'CRYPTO' | 'STOCK' | 'FUND' | 'OTHER'; symbol: string; name: string; precision: number }
export type InvestmentEventType = 'OPENING' | 'BUY' | 'SELL' | 'TRANSFER';
export interface InvestmentEvent {
    id: string; type: InvestmentEventType; accountId: string; instrumentId: string; toAccountId: string;
    quantity: string; amount: string; fee: string; cost: string | null;
    settlementInstrumentId: string; settlementAccountId: string; cashAccountId: string;
    exchangeRate: string; occurredAt: number; note: string; version: number; voided: boolean;
}
export interface InvestmentQuote {
    instrumentId?: string; price: string; currency: string; source: string; sourceTime: number;
    receivedAt: number; state: string; fxRate?: string; fxDate?: string; fxState?: string; fxSource?: string; fxReceivedAt?: number; connected?: boolean;
}
export interface InvestmentPosition {
    accountId: string; instrumentId: string; quantity: string; cost: string | null;
    costKnown: boolean; averageCost: string | null; realizedPnl: string | null;
    marketValue?: string | null; unrealizedPnl?: string | null; quote?: InvestmentQuote;
}
export interface InvestmentSettings { baseCurrency: string; timeZone: string }
export interface WealthCashAccount { id: string; name: string; currency: string; balance: string; value: string | null; liability: boolean }
export interface WealthSummary {
    baseCurrency: string; netAssets: string | null; valuedAssets: string; cashAssets: string;
    investmentValue: string; liabilities: string; missingPrices: number; stalePrices: number;
    unrealizedPnl: string | null; realizedPnl: string | null; costComplete: boolean;
    cashAccounts: WealthCashAccount[]; positions: InvestmentPosition[];
}
export interface WealthSnapshot { id: string; recordedAt: number; netAssets: string | null; valuedAssets: string; complete: boolean; invalidated: boolean }
export interface InvestmentPreview { positions: InvestmentPosition[]; effects: unknown; event: InvestmentEvent }
