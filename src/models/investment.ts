export interface InvestmentAccount { id: string; name: string; kind: string; platform?: string; instruments?: string[]; currency?: 'CNY' | 'USD'; paymentInstruments?: string[] }
export interface AccountDeletionTarget { id: string; kind: 'cash' | 'portfolio' }
export interface AccountDeletionPreview { name: string; transactionCount: number; investmentCount: number; templateCount: number; dueCount?:number; depositCount?:number; installmentCount?:number; subAccountCount: number; parentAccountName: string; affectedAccounts: string[]; blockedReason: string; token: string }
export interface ConversionInput { fromInstrumentId: string; toInstrumentId: string; fromQuantity: string }
export interface InvestmentConversion extends ConversionInput { toQuantity: string; fromPrice: string; toPrice: string; observedAt: number; expiresAt: number; fromQuote?: InvestmentQuote; toQuote?: InvestmentQuote }
export interface InstrumentBinding { market: string; provider: string; providerId: string; currency: string }
export interface InstrumentCandidate extends InstrumentBinding { name: string; symbol: string; type: 'CRYPTO' | 'STOCK' | 'FUND' | 'OTHER' }
export interface Instrument extends Partial<InstrumentBinding> { id: string; type: 'CRYPTO' | 'STOCK' | 'FUND' | 'OTHER'; symbol: string; name: string; precision: number }
export type InvestmentEventType = 'OPENING' | 'BUY' | 'SELL' | 'TRANSFER' | 'INCOME' | 'EXPENSE';
export interface WalletEntry { currency: 'CNY' | 'USD'; categoryId: string; fxDate: string; fxSource: string }
export interface AssetMovement { instrumentId: string; quantity: string }
export interface InvestmentEvent {
    id: string; type: InvestmentEventType; accountId: string; instrumentId: string; toAccountId: string; bookId?: string;
    quantity: string; amount: string; fee: string; cost: string | null;
    settlementInstrumentId: string; settlementAccountId: string; cashAccountId: string;
    exchangeRate: string; occurredAt: number; note: string; version: number; voided: boolean;
    conversion?: InvestmentConversion;
    wallet?: WalletEntry; additionalMovements?: AssetMovement[];
}
export interface InvestmentQuote {
    instrumentId?: string; price: string; currency: string; source: string; sourceTime: number;
    receivedAt: number; state: string; fxRate?: string; fxDate?: string; fxState?: string; fxSource?: string; fxReceivedAt?: number; connected?: boolean;
    changePercent?: string | null; changePeriod?: '24h' | 'session' | 'nav';
}
export interface InvestmentPosition {
    accountId: string; instrumentId: string; quantity: string; cost: string | null;
    costKnown: boolean; averageCost: string | null; realizedPnl: string | null;
    marketValue?: string | null; unrealizedPnl?: string | null; quote?: InvestmentQuote;
}
export interface InvestmentSettings { baseCurrency: string; timeZone: string }
export interface WealthCashAccount { id: string; name: string; currency: string; balance: string; value: string | null; liability: boolean }
export interface WealthSummary {
    fxRates?: { base: string; quote: string; rate: string; source: string; date: string; state: string; receivedAt: number }[];
    baseCurrency: string; netAssets: string | null; valuedAssets: string; cashAssets: string;
    investmentValue: string; liabilities: string; missingPrices: number; stalePrices: number;
    unrealizedPnl: string | null; realizedPnl: string | null; costComplete: boolean;
    cashAccounts: WealthCashAccount[]; positions: InvestmentPosition[];
}
export interface WealthSnapshot { id: string; recordedAt: number; netAssets: string | null; valuedAssets: string; complete: boolean; invalidated: boolean }
export interface InvestmentPreview { positions: InvestmentPosition[]; effects: unknown; event: InvestmentEvent }
