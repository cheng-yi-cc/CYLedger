import {describe,it,expect} from 'vitest';
import {groupInvestmentHoldings} from '@/lib/investment-groups.ts';
import type {HoldingRow} from '@/lib/investment-mobile.ts';
const row=(account:string,id='crypto:bitcoin',cost:string|null='10'):HoldingRow=>({key:account+':'+id,name:'比特币',group:'加密货币',profit:'2',asset:{id,type:'CRYPTO',symbol:'BTC',name:'比特币',precision:18},account:{id:account,name:account,kind:'wallet'},profile:{id:'',accountId:account,instrumentId:id,name:'',group:'',note:'',profitOffset:'0',hidden:false,excludeFromTotal:false,excludeProfit:false,bookIds:[],version:0},position:{accountId:account,instrumentId:id,quantity:'0.1',cost,costKnown:cost!==null,averageCost:null,realizedPnl:'0',unrealizedPnl:'2',dailyPnl:'0.2',marketValue:'12'}});
describe('cross-account crypto summary',()=>{
 it('sums decimal amounts without duplicating or modifying account positions',()=>{
  const a=row('exchange'),b=row('wallet');const result=groupInvestmentHoldings([a,b]);
  expect(result).toHaveLength(1);expect(result[0]!.position.quantity).toBe('0.2');expect(result[0]!.position.cost).toBe('20');expect(result[0]!.position.dailyPnl).toBe('0.4');expect(result[0]!.profit).toBe('4');expect(result[0]!.members).toEqual([a,b]);expect(a.position.quantity).toBe('0.1');
 });
 it('preserves unknown amounts and never merges by ticker alone',()=>{
  const a=row('exchange'),b=row('wallet',undefined,null);b.position.dailyPnl=null;
  const result=groupInvestmentHoldings([a,b]);expect(result[0]!.position.cost).toBeNull();expect(result[0]!.position.dailyPnl).toBeNull();
  expect(groupInvestmentHoldings([a,row('other','custom-btc')])).toHaveLength(2);
 });
 it('merges only a verified provider identity with a fixed preset',()=>{
  const a=row('exchange'),b=row('wallet','bound');b.asset={...b.asset!,market:'CRYPTO',provider:'coinbase',providerId:'BTC-USD',currency:'USD'};
  expect(groupInvestmentHoldings([a,b])).toHaveLength(1);
 });
});
