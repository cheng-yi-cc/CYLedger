import { LedgerDecimal, ledgerMoney } from '@/lib/ledger-display.ts';
import { cryptoInput } from '@/lib/crypto-entry.ts';
import type { AssetMovement, Instrument, InvestmentAccount, InvestmentPosition, WealthSummary } from '@/models/investment.ts';

const Money = LedgerDecimal.clone({precision:80});
export function walletCurrency(account?: InvestmentAccount): 'CNY' | 'USD' { return account?.currency === 'USD' ? 'USD' : 'CNY'; }
export function currencyMoney(value: string | null | undefined, currency: string): string { return value == null ? '—' : `${currency==='USD'?'$':'¥'}${ledgerMoney(value,false)}`; }
export function usdFX(summary?: WealthSummary) { return summary?.fxRates?.find(rate=>rate.base==='USD'&&rate.quote==='CNY'&&new Money(rate.rate||'0').gt(0)); }
export function positionValue(position: InvestmentPosition | undefined, currency: string, summary?: WealthSummary): string | null {
    if (!position || new Money(position.quantity).isZero()) return '0';
    if(currency==='CNY')return position.marketValue??null;
    if(position.quote?.currency==='USD'&&position.quote.price)return new Money(position.quantity).mul(position.quote.price).toFixed();
    const fx=usdFX(summary);return position.marketValue!=null&&fx?new Money(position.marketValue).div(fx.rate).toFixed():null;
}
export function walletValue(positions: InvestmentPosition[],currency:string,summary?:WealthSummary):string|null {
    let total=new Money(0);
    for(const position of positions){const value=positionValue(position,currency,summary);if(value===null)return null;total=total.plus(value)}
    return total.toFixed();
}
export function suggestedPaymentCoins(account:InvestmentAccount,coins:Instrument[]):string[] {
    const selected=coins.filter(c=>(account.instruments||[]).includes(c.id));
    if(Array.isArray(account.paymentInstruments))return account.paymentInstruments.filter(id=>selected.some(c=>c.id===id));
    // Symbol collisions are ambiguous, so only suggest a uniquely named asset.
    const primary=selected.filter(c=>c.symbol.toUpperCase()==='USD24');
    const ids=primary.length===1?[primary[0]!.id]:[];
    if(selected.some(c=>c.id==='crypto:usd-coin'))ids.push('crypto:usd-coin');
    return ids;
}
export function allocateWalletPayment(amount:string,currency:string,rate:string,ids:string[],positions:InvestmentPosition[],accountId:string):AssetMovement[] {
    let remaining=new Money(cryptoInput(amount,2));
    if(currency==='CNY')remaining=remaining.div(cryptoInput(rate));
    // This is a user-confirmable 1 USD per coin estimate, never a market quote.
    remaining=remaining.toDecimalPlaces(18,Money.ROUND_UP);
    const legs:AssetMovement[]=[];
    for(const id of [...new Set(ids)].filter(Boolean)){
        const available=new Money(positions.find(p=>p.accountId===accountId&&p.instrumentId===id)?.quantity||'0');
        const quantity=Money.min(available,remaining);
        if(quantity.gt(0)){legs.push({instrumentId:id,quantity:quantity.toFixed()});remaining=remaining.minus(quantity)}
    }
    if(remaining.gt(0))throw Error('所选币种余额不足，请核对金额或选择其他币种');
    return legs;
}
