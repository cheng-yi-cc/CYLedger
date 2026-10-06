import {describe,it,expect} from 'vitest';
import {allocateWalletPayment,positionValue,walletValue} from '@/lib/wallet-entry.ts';
import type {InvestmentPosition,WealthSummary} from '@/models/investment.ts';
const position=(id:string,quantity:string):InvestmentPosition=>({accountId:'wallet',instrumentId:id,quantity,cost:null,costKnown:false,averageCost:null,realizedPnl:null});
describe('wallet payment quantities and display currency',()=>{
 it('allocates the exact decimal shortage only to the fallback asset',()=>{
  expect(allocateWalletPayment('15','USD','7.2',['usd24','usdc'],[position('usd24','10'),position('usdc','20')],'wallet')).toEqual([{instrumentId:'usd24',quantity:'10'},{instrumentId:'usdc',quantity:'5'}]);
  expect(allocateWalletPayment('0.30','USD','7.2',['usd24','usdc'],[position('usd24','0.1'),position('usdc','0.2')],'wallet')).toEqual([{instrumentId:'usd24',quantity:'0.1'},{instrumentId:'usdc',quantity:'0.2'}]);
 });
 it('rejects combined insufficiency and never spends an unrelated wallet',()=>{
  expect(()=>allocateWalletPayment('31','USD','7.2',['usd24','usdc'],[position('usd24','10'),position('usdc','20')],'wallet')).toThrow('余额不足');
  expect(()=>allocateWalletPayment('1','USD','7.2',['usd24'],[position('usd24','10')],'other')).toThrow('余额不足');
 });
 it('converts CNY only with a confirmed positive FX and preserves tiny quantities',()=>{
  expect(allocateWalletPayment('108','CNY','7.2',['usd24','usdc'],[position('usd24','10'),position('usdc','20')],'wallet')[1]!.quantity).toBe('5');
  expect(()=>allocateWalletPayment('108','CNY','',['usd24'],[position('usd24','100')],'wallet')).toThrow();
 });
 it('shows original USD valuation without fabricating missing RMB conversion',()=>{
  const p={...position('usd24','12.34'),marketValue:null,quote:{price:'1.25',currency:'USD',source:'手动估值',sourceTime:1,receivedAt:1,state:'manual'}};
  expect(positionValue(p,'USD')).toBe('15.425');expect(positionValue(p,'CNY')).toBeNull();
  expect(walletValue([p,position('unquoted','1')],'USD')).toBeNull();
  expect(walletValue([],'USD')).toBe('0');
 });
 it('does not treat a CNY quote as a dollar price when FX is missing',()=>{
  const p={...position('usd24','10'),marketValue:'72'};
  expect(positionValue(p,'USD')).toBeNull();
  const summary={fxRates:[{base:'USD',quote:'CNY',rate:'7.2',date:'2026-10-06',source:'fixture',state:'live',receivedAt:1}]} as WealthSummary;
  expect(positionValue(p,'USD',summary)).toBe('10');
 });
});
