export interface InvestmentAccount { id: string; name: string; kind: string; platform?: string; instruments?: string[]; currency?: 'CNY' | 'USD'; paymentInstruments?: string[] }
export interface AccountDeletionTarget { id: string; kind: 'cash' | 'portfolio' }
export interface AccountDeletionPreview { name: string; transactionCount: number; investmentCount: number; templateCount: number; dueCount?:number; depositCount?:number; installmentCount?:number; subAccountCount: number; parentAccountName: string; affectedAccounts: string[]; blockedReason: string; token: string }
export interface ConversionInput { fromInstrumentId: string; toInstrumentId: string; fromQuantity: string }
export interface InvestmentConversion extends ConversionInput { toQuantity: string; fromPrice: string; toPrice: string; observedAt: number; expiresAt: number; fromQuote?: InvestmentQuote; toQuote?: InvestmentQuote }
export interface InstrumentBinding { market: string; provider: string; providerId: string; currency: string }
export interface InstrumentCandidate extends InstrumentBinding { name: string; symbol: string; type: 'CRYPTO' | 'STOCK' | 'FUND' | 'OTHER' }
export interface Instrument extends Partial<InstrumentBinding> { id: string; type: 'CRYPTO' | 'STOCK' | 'FUND' | 'OTHER'; symbol: string; name: string; precision: number }
export type InvestmentEventType = 'OPENING' | 'BUY' | 'SELL' | 'TRANSFER' | 'INCOME' | 'EXPENSE' | 'ADJUST';
export interface WalletEntry { currency: 'CNY' | 'USD'; categoryId: string; fxDate: string; fxSource: string }
export interface AssetMovement { instrumentId: string; quantity: string }
export interface InvestmentEvent {
    id: string; type: InvestmentEventType; accountId: string; instrumentId: string; toAccountId: string; bookId?: string;
    quantity: string; amount: string; fee: string; cost: string | null;
    settlementInstrumentId: string; settlementAccountId: string; cashAccountId: string;
    exchangeRate: string; occurredAt: number; note: string; version: number; voided: boolean;
    conversion?: InvestmentConversion;
    wallet?: WalletEntry; additionalMovements?: AssetMovement[];
    fund?: {tradeDate:string;confirmDate:string;price:string;priceDate:string;source:string;orderId?:string};
}
export interface InvestmentQuote {
    instrumentId?: string; price: string; currency: string; source: string; sourceTime: number;
    receivedAt: number; state: string; fxRate?: string; fxDate?: string; fxState?: string; fxSource?: string; fxReceivedAt?: number; connected?: boolean;
    changePercent?: string | null; changePeriod?: '24h' | 'session' | 'nav';
}
export interface InvestmentPosition {
    dailyPnl?: string | null; dayStart?: number; dailyReason?: string;
    dailyReference?: {source:string;price:string;currency:string;at:number;sourceTime:number;period:number;fxRate:string;fxDate:string};
    accountId: string; instrumentId: string; quantity: string; cost: string | null;
    costKnown: boolean; averageCost: string | null; realizedPnl: string | null;
    marketValue?: string | null; unrealizedPnl?: string | null; quote?: InvestmentQuote;
    totalValue?: string | null; profile?: HoldingProfile;
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

export interface HoldingProfile {id:string;accountId:string;instrumentId:string;name:string;group:string;note:string;profitOffset:string;hidden:boolean;excludeFromTotal:boolean;excludeProfit:boolean;bookIds:string[];version:number}
export interface HoldingSetup {profile:HoldingProfile;instrument:Partial<Instrument>;quantity:string;cost:string|null;price:string;bookId:string;occurredAt:number;expectedQuantity?:string;expectedCost?:string|null}
export interface InvestmentPlan {id:string;accountId:string;instrumentId:string;cashAccountId:string;bookId:string;amount:string;feePercent:string;cycle:string;startDate:string;endDate:string;nextDate:string;time:string;timeZone:string;note:string;paused:boolean;deleted:boolean;version:number}
export interface InvestmentOrder {id:string;planId:string;accountId:string;instrumentId:string;cashAccountId:string;bookId:string;type:'BUY'|'SELL';amount:string;quantity:string;fee:string;feePercent:string;tradeDate:string;confirmDate:string;time:string;timeZone:string;note:string;status:'pending'|'completed'|'cancelled';eventId:string;price:string;priceDate:string;error:string;version:number}
export interface InvestmentReport {items:{event:InvestmentEvent;effect:{cashDeltaCny:string|null;realizedPnl:string|null;investmentFee:string|null;acquiredCost:string|null;releasedCost:string|null;settlementRealizedPnl:string|null}}[];history:{at:number;value:string|null;profit:string|null}[];annualized:string|null;timeZone:string}
