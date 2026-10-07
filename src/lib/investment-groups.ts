import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { HoldingRow } from '@/lib/investment-mobile.ts';

export interface HoldingGroupRow extends HoldingRow { members: HoldingRow[] }
const knownCoins: Record<string,string> = { 'BTC-USD':'bitcoin', 'ETH-USD':'ethereum', 'SOL-USD':'solana', 'USDT-USD':'tether', 'USDC-USD':'usd-coin' };
function identity(row: HoldingRow): string {
    const a = row.asset;
    if (a?.type !== 'CRYPTO') return row.key;
    if (a.id.startsWith('crypto:') && Object.values(knownCoins).includes(a.id.slice(7))) return a.id;
    if (a.market === 'CRYPTO' && a.providerId) {
        if (a.provider === 'coingecko') return `crypto:${a.providerId}`;
        if (a.provider === 'coinbase' && knownCoins[a.providerId]) return `crypto:${knownCoins[a.providerId]}`;
        return `${a.provider}:${a.providerId}`;
    }
    return `instrument:${a.id}`;
}
function sum(values: (string|null|undefined)[]): string|null {
    return values.some(v=>v==null)?null:values.reduce((n,v)=>n.plus(v!),new LedgerDecimal(0)).toString();
}
export function groupInvestmentHoldings(rows: HoldingRow[]): HoldingGroupRow[] {
    const grouped = new Map<string,HoldingRow[]>();
    for (const row of rows) { const key=identity(row); grouped.set(key,[...(grouped.get(key)||[]),row]); }
    return [...grouped].map(([key,members])=>{
        const first=members[0]!;
        if (members.length===1) return {...first,members};
        const positions=members.map(r=>r.position);
        const quantity=sum(positions.map(p=>p.quantity))!;
        const cost=sum(positions.map(p=>p.cost));
        // A symbol alone never proves that two assets are the same currency.
        const quotes=positions.map(p=>p.quote);
        const quote=quotes.every(q=>q?.price===quotes[0]?.price&&q?.currency===quotes[0]?.currency&&q?.source===quotes[0]?.source)?quotes[0]:undefined;
        return {...first,key,members,name:first.asset?.name||first.name,group:'加密货币',profit:sum(members.map(r=>r.profit)),position:{...first.position,quantity,cost,costKnown:cost!==null,averageCost:cost!==null&&new LedgerDecimal(quantity).gt(0)?new LedgerDecimal(cost).div(quantity).toString():null,marketValue:sum(positions.map(p=>p.marketValue)),unrealizedPnl:sum(positions.map(p=>p.unrealizedPnl)),realizedPnl:sum(positions.map(p=>p.realizedPnl)),dailyPnl:sum(positions.map(p=>p.dailyPnl)),quote}};
    });
}
