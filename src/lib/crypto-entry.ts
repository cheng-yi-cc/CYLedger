import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { Instrument, InvestmentPosition, InvestmentConversion, InvestmentEvent } from '@/models/investment.ts';
export type CryptoMode = 'cash' | 'coin' | 'redeem' | 'transfer' | 'opening';
export type CashEntryMode = 'amount' | 'quantity';
export interface CryptoDraft {
    mode: CryptoMode; fromAccount: string; toAccount: string; fromCoin: string; toCoin: string;
    amount: string; received: string; bookId: string; note: string;
    cashEntryMode?: CashEntryMode; buyPrice?: string;
}
const PurchaseDecimal = LedgerDecimal.clone({ precision: 80 });
/** Normalize keypad input without converting accounting values to floating point. */
export function cryptoInput(value: string, precision = 18): string {
    let text = value.trim();
    if (text.startsWith('.')) text = '0' + text;
    if (text.endsWith('.')) text = text.slice(0, -1);
    if (text.length > 60 || !/^\d+(\.\d+)?$/.test(text)) throw Error('请输入有效的金额或数量');
    if ((text.split('.')[1]?.length || 0) > precision) throw Error(`最多支持 ${precision} 位小数`);
    const amount = new LedgerDecimal(text);
    if (!amount.gt(0)) throw Error('金额或数量必须大于零');
    return amount.toFixed();
}
export function heldCryptoCoins(coins: Instrument[], positions: InvestmentPosition[], accountId: string): Instrument[] {
    const held = new Set(positions.filter(p => p.accountId === accountId && new LedgerDecimal(p.quantity).gt(0)).map(p => p.instrumentId));
    return coins.filter(c => held.has(c.id));
}
export function defaultCryptoCoin(coins: Instrument[], requested = ''): string {
    if (coins.some(c => c.id === requested)) return requested;
    return coins.find(c => c.id === 'crypto:tether')?.id || coins[0]?.id || '';
}
export function receivedAfterQuote(current: string, edited: boolean, estimate: string): string {
    return edited ? current : estimate;
}
/** Derive one settlement fact from the user's other fact and CNY unit price. */
export function calculateCashPurchase(mode: CashEntryMode, amount: string, quantity: string, unitPrice: string): { amount: string; quantity: string } {
    if (!unitPrice.trim()) throw Error('请填写买入单价');
    const price = new PurchaseDecimal(cryptoInput(unitPrice));
    let payment: string;
    let received: string;
    if (mode === 'quantity') {
        received = cryptoInput(quantity);
        payment = new PurchaseDecimal(received).mul(price).toDecimalPlaces(2, PurchaseDecimal.ROUND_HALF_UP).toFixed();
        if (new PurchaseDecimal(payment).isZero()) throw Error('计算出的付款金额不足 0.01 元，请调整数量或单价');
    } else {
        payment = cryptoInput(amount, 2);
        received = new PurchaseDecimal(payment).div(price).toDecimalPlaces(18, PurchaseDecimal.ROUND_DOWN).toFixed();
        if (new PurchaseDecimal(received).isZero()) throw Error('计算出的数量低于支持的最小精度，请调整金额或单价');
    }
    return { amount: payment, quantity: received };
}
export function buildCryptoEvent(d: CryptoDraft, quote: InvestmentConversion | undefined, now: number): InvestmentEvent {
    const purchase = d.mode === 'cash' ? calculateCashPurchase(d.cashEntryMode || 'amount', d.amount, d.received, d.buyPrice || '') : undefined;
    const amount = purchase?.amount || cryptoInput(d.amount, 18);
    const needsQuote = ['coin', 'redeem'].includes(d.mode);
    const received = purchase?.quantity || (needsQuote ? cryptoInput(d.received, d.mode === 'redeem' ? 2 : 18) : amount);
    if (!d.toAccount || (d.mode !== 'opening' && !d.fromAccount)) throw Error('请选择转出和转入账户');
    if (d.mode !== 'redeem' && !d.toCoin || !['opening', 'cash'].includes(d.mode) && !d.fromCoin) throw Error('请选择币种');
    if (d.mode === 'transfer' && d.fromAccount === d.toAccount) throw Error('请选择另一个转入账户');
    if (d.mode === 'coin' && d.fromCoin === d.toCoin) throw Error('请选择不同币种；同币种请使用转移');
    // Only a matching, unexpired observation can establish a historical reference FX.
    const observed = needsQuote && quote && quote.expiresAt > now && quote.observedAt <= now + 5 &&
        quote.fromInstrumentId === (d.mode === 'cash' ? '' : d.fromCoin) &&
        quote.toInstrumentId === (d.mode === 'redeem' ? '' : d.toCoin) &&
        new LedgerDecimal(quote.fromQuantity).eq(amount) ? quote : undefined;
    const outgoing = d.mode === 'redeem' || d.mode === 'transfer';
    return {
        id: '', type: d.mode === 'opening' ? 'OPENING' : d.mode === 'transfer' ? 'TRANSFER' : d.mode === 'redeem' ? 'SELL' : 'BUY',
        accountId: outgoing ? d.fromAccount : d.toAccount, instrumentId: outgoing ? d.fromCoin : d.toCoin,
        toAccountId: d.mode === 'transfer' ? d.toAccount : '', bookId: d.bookId,
        quantity: ['opening', 'redeem', 'transfer'].includes(d.mode) ? amount : received,
        amount: d.mode === 'redeem' ? received : d.mode === 'cash' || needsQuote ? amount : '0', fee: '0', cost: null,
        settlementInstrumentId: d.mode === 'coin' ? d.fromCoin : '', settlementAccountId: d.mode === 'coin' ? d.fromAccount : '',
        cashAccountId: d.mode === 'cash' ? d.fromAccount : d.mode === 'redeem' ? d.toAccount : '',
        exchangeRate: d.mode === 'coin' ? observed?.fromPrice || '' : d.mode === 'transfer' ? '' : '1',
        occurredAt: now, note: d.note.trim(), version: 0, voided: false,
        ...(observed ? { conversion: observed } : {})
    };
}
