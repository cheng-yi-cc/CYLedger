import {describe,it,expect} from 'vitest';
import {accountInBooks,holdingInBooks,holdingVisible,investmentAmounts} from '@/lib/investment-scope.ts';
import type {AssetPreferences} from '@/lib/asset-tools.ts';
import type {HoldingRow} from '@/lib/investment-mobile.ts';
const preferences:AssetPreferences={revision:'0',rules:{'portfolio:a':{hidden:true,disabledBooks:['b1']}},reminders:{enabled:false,credit:true,debt:true,deposit:true,advanceDays:0,minuteOfDay:0}};
function row():HoldingRow{return {key:'a:i',name:'基金',group:'基金',profit:'20',profile:{id:'p',accountId:'a',instrumentId:'i',name:'基金',group:'基金',note:'',profitOffset:'0',hidden:false,excludeFromTotal:false,excludeProfit:false,bookIds:[],version:1},position:{accountId:'a',instrumentId:'i',quantity:'10',cost:'100',costKnown:true,averageCost:'10',marketValue:'120',unrealizedPnl:'20',realizedPnl:'0'}};}
describe('asset accounting scope',()=>{
 it('requires the same selected book to allow both account and holding',()=>{
  const p={...row().profile,bookIds:['b1']};
  expect(accountInBooks(preferences,'portfolio','a',['b1','b2'])).toBe(true);
  expect(holdingInBooks(p,preferences,['b1','b2'])).toBe(false);
  expect(holdingInBooks(p,preferences,[])).toBe(true);
  expect(holdingVisible(p,preferences)).toBe(false);
 });
 it('keeps hidden wealth and separates net asset contribution from market value',()=>{
  const hidden=row();hidden.profile.hidden=true;hidden.profile.excludeProfit=true;
  const excluded=row();excluded.profile.excludeFromTotal=true;excluded.position.marketValue=null;
  const money=[{value:'2000',excluded:false,binding:{totalIncome:'5'}},{value:null,excluded:true,binding:{}}];
  expect(investmentAmounts([hidden,excluded],money)).toEqual({netValue:'2100',marketValue:'2120',holdingProfit:'20',totalProfit:'25'});
  expect(holdingVisible(hidden.profile,{...preferences,rules:{}})).toBe(false);
 });
 it('preserves unknown market value independently of a known contribution at cost',()=>{
  const r=row();r.profile.excludeProfit=true;r.position.marketValue=null;r.profit=null;
  expect(investmentAmounts([r],[])).toMatchObject({netValue:'100',marketValue:null,totalProfit:null});
  r.position.cost=null;
  expect(investmentAmounts([r],[]).netValue).toBeNull();
 });
});
