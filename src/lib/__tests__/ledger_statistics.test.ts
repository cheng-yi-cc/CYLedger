import { describe, it, expect } from 'vitest';
import { categoryBreakdown, dailyLedgerTotals } from '../ledger-statistics.ts';
const entry=(cny:string|null,categoryId='a',type=3)=>({day:'2026-10-02',type,cny,categoryId,primaryCategoryId:categoryId,primaryCategory:categoryId,title:categoryId});
describe('statistics overview accounting',()=>{
 it('uses net category amounts as the denominator and keeps actual transaction counts',()=>{
  const result=categoryBreakdown([entry('30'),entry('-5'),entry('75','b'),entry(null,'c'),entry('999','a',4)],true,3);
  expect(result.total).toBe('100');expect(result.canShare).toBe(true);
  expect(result.items.map(x=>[x.id,x.amount,x.percent,x.count])).toEqual([['b','75','75.00',1],['a','25','25.00',2]]);
 });
 it('does not turn a net refund into a positive pie slice',()=>{
  const result=categoryBreakdown([entry('-20'),entry('100','b')],true,3);
  expect(result.total).toBe('80');expect(result.canShare).toBe(false);expect(result.items.every(x=>x.percent===null)).toBe(true);
  expect(categoryBreakdown([],true,3).canShare).toBe(false);
 });
 it('does not lose decimal fractions or combine equally named independent categories',()=>{
  const result=categoryBreakdown([entry('0.1'),entry('0.2'),entry('0.3','b')],true,3);
  expect(result.total).toBe('0.6');expect(result.items).toHaveLength(2);expect(result.items.every(x=>x.percent==='50.00')).toBe(true);
 });
 it('excludes opening balances, transfers and investment settlements from daily income/expense',()=>{
  const rows=dailyLedgerTotals([entry('1000','a',1),entry('999','a',4),{...entry('500','a',2),investment:true},entry('0.1','a',2),entry('0.2','a',2),entry('0.1'),{...entry('20'),day:'2026-10-01'}]);
  expect(rows[0]).toEqual({day:'2026-10-02',income:'0.3',expense:'0.1',balance:'0.2',complete:true});expect(rows[1]?.day).toBe('2026-10-01');
 });
 it('marks missing FX as incomplete rather than claiming a complete total',()=>{
  expect(dailyLedgerTotals([entry(null),entry('5')])[0]).toMatchObject({expense:'5',complete:false});
 });
});
