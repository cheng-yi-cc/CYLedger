import { describe, expect, it } from 'vitest';
import moment from 'moment-timezone';
import { budgetReports, budgetDaily, tagGroupIds, groupReport, reportBuckets, reportBreakdown, comparisonValue, elapsedDays } from '../statistics-report.ts';
import { filterStatisticsEntries } from '../statistics-links.ts';
import type { LedgerEntry } from '../mobile-ledger.ts';
import type { StatisticsBudget } from '../statistics-workspace.ts';
const zone='Asia/Shanghai';
const entry=(amount:string|null,day='2026-10-02',patch:Partial<LedgerEntry>={}):LedgerEntry=>({id:'t',type:3,day,time:moment.tz(`${day} 12:00`,zone).unix(),title:'交通',primaryCategory:'旅行',account:'现金',currency:amount===null?'USD':'CNY',amount:amount||'10',cny:amount,hidden:false,categoryId:'child',primaryCategoryId:'root',comment:'',tags:[],bookId:'book',accountId:'cash',destinationAccountId:'0',tagIds:[],investment:false,...patch});
const budget=(patch:Partial<StatisticsBudget>={}):StatisticsBudget=>({id:'b',bookId:'book',categoryId:'0',name:'月预算',amount:'100',startDate:'2026-09-01',endDate:'2026-09-30',kind:'monthly',repeat:true,revision:'1',...patch});
const carry={carrySurplus:true,carryDeficit:true};
describe('budget calculations from ledger facts',()=>{
 it('replays carry, refunds and a monthly override without double counting category budgets',()=>{
  const facts=[entry('40','2026-09-05'),entry('30'),entry('-5'),entry('999','2026-10-02',{type:4}),entry('500','2026-10-02',{excludeFromStatistics:true}),entry('500','2026-10-02',{investment:true})];
  const rows=budgetReports([budget(),budget({id:'override',amount:'80',startDate:'2026-10-01',endDate:'2026-10-31'}),budget({id:'category',categoryId:'root',amount:'50'})],facts,'2026-10',carry);
  expect(rows[0]).toMatchObject({base:'80',carry:'60',available:'140',spent:'25',remaining:'115',complete:true});
  expect(rows[1]).toMatchObject({available:'60',spent:'25',remaining:'35'});
 });
 it('does not convert unknown FX to zero and propagates unknown carry only when enabled',()=>{
  const facts=[entry(null,'2026-09-01'),entry('10')];
  expect(budgetReports([budget()],facts,'2026-10',carry)[0]).toMatchObject({carry:null,available:null,remaining:null,complete:false});
  expect(budgetReports([budget()],facts,'2026-10',{carrySurplus:false,carryDeficit:false})[0]).toMatchObject({available:'100',remaining:'90',complete:true});
 });
 it('keeps books, one-off rules and custom periods separate',()=>{
  const rules=[budget({repeat:false}),budget({id:'custom',kind:'custom',repeat:false,startDate:'2026-10-02',endDate:'2026-10-05'})];
  const rows=budgetReports(rules,[entry('5'),entry('100','2026-10-02',{bookId:'other'}),entry('20','2026-10-06')],'2026-10',carry);
  expect(rows).toHaveLength(1);expect(rows[0]).toMatchObject({spent:'5',remaining:'95',carry:'0'});
 });
 it('supports selective deficit carry and remaining daily allowance including today',()=>{
  const row=budgetReports([budget()],[entry('140','2026-09-10'),entry('20','2026-10-01'),entry('10','2026-10-02')],'2026-10',{carrySurplus:false,carryDeficit:true})[0]!;
  expect(row).toMatchObject({carry:'-40',available:'60',remaining:'30'});
  expect(budgetDaily(row,[entry('10','2026-10-02')],moment.tz('2026-10-02 18:00',zone),'remaining')).toEqual({daily:'1.33',todayRemaining:'-8.67',average:'15.00'});
  expect(budgetDaily({...row,complete:false,available:null},[],moment.tz('2026-10-02',zone),'fixed').daily).toBeNull();
 });
});
describe('statistics grouping, boundaries and drill-down',()=>{
 const tags={p:{id:'p',name:'旅行',parentId:'0'},c:{id:'c',name:'交通',parentId:'p'},d:{id:'d',name:'住宿',parentId:'p'}};
 it('counts one transaction once per root even when parent and siblings are selected',()=>{
  expect(tagGroupIds(['p','c','d'],tags,'parents')).toEqual(['p']);
  expect(tagGroupIds(['p','c','d'],tags,'direct')).toEqual(['p']);
  expect(tagGroupIds(['p','c','d'],tags,'children')).toEqual(['c','d']);
  expect(groupReport([entry('0.1','2026-10-02',{tagIds:['p','c']}),entry('0.2','2026-10-02',{tagIds:['d']})],'tag',{tags,tagMode:'parents'})[0]).toMatchObject({expense:'0.3',count:2});
 });
 it('preserves exact decimals and negative refunds without false pie shares',()=>{
  const groups=groupReport([entry('-10'),entry('20','2026-10-02',{categoryId:'other',primaryCategoryId:'other'})],'category');
  expect(reportBreakdown(groups,'expense')).toMatchObject({total:'10',canShare:false});
  expect(reportBreakdown(groupReport([entry('20'),entry(null)],'category'),'expense')).toMatchObject({complete:false,canShare:false});
  expect(comparisonValue('50','0')).toEqual({difference:'50',percent:null});
  expect(comparisonValue('50','40',false)).toEqual({difference:null,percent:null});
 });
 it('uses the accounting timezone and leap-year month boundaries',()=>{
  const from=moment.tz('2024-02-01',zone),to=from.clone().endOf('month');
  const rows=reportBuckets([entry('3','2024-02-29',{time:moment.utc('2024-02-29T15:59:59Z').unix()}),entry('9','2024-03-01',{time:moment.utc('2024-02-29T16:00:00Z').unix()})],from,to);
  expect(rows).toHaveLength(29);expect(rows[28]?.expense).toBe('3');
  expect(elapsedDays(from,to,moment.tz('2024-02-05 23:00',zone))).toBe(5);
 });
 it('keeps the initial scope when drilling into another dimension and handles auxiliary facts',()=>{
  const facts=[entry('10','2026-10-02',{id:'a',tagIds:['p','c']}),entry('20','2026-10-02',{id:'b',tagIds:['c']}),entry('30','2026-10-02',{id:'d',type:4})];
  expect(filterStatisticsEntries(facts,{filterTagId:'p',tagId:'p',tagMode:'parents'},tags).map(x=>x.id)).toEqual(['a']);
  expect(filterStatisticsEntries(facts,{aux:'debt',debtAction:'repay'},tags,{debtActions:{d:'repay'},feeIds:[]}).map(x=>x.id)).toEqual(['d']);
  expect(filterStatisticsEntries([entry('10','2026-10-02',{discountAmount:'0.00'})],{aux:'discount'},tags)).toEqual([]);
 });
});
