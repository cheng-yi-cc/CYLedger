import {describe,expect,it,vi} from 'vitest';
vi.mock('@/lib/services.ts',()=>({default:{}}));
import moment from 'moment-timezone';
import {unzipSync,strFromU8} from 'fflate';
import {emptyLedgerSearchFilters,filterLedgerEntries} from '../ledger-search.ts';
import {newLedgerWish,wishProgress} from '../ledger-workspace.ts';
import {parseBillOCR} from '../ledger-ocr.ts';
import {ledgerEntriesCSV,ledgerWorkbook} from '../ledger-export.ts';
import type {LedgerEntry} from '../mobile-ledger.ts';
const zone='Asia/Shanghai';
const entry=(patch:Partial<LedgerEntry>={}):LedgerEntry=>({id:'1',type:3,day:'2026-10-05',time:moment.tz('2026-10-05 12:00',zone).unix(),title:'早餐',primaryCategory:'餐饮',primaryCategoryId:'p',categoryId:'c',account:'现金',accountId:'a',bookId:'b',destinationAccountId:'0',currency:'CNY',amount:'12.34',cny:'12.34',hidden:false,comment:'公司 早餐',tags:['工作'],tagIds:['t'],investment:false,...patch});
describe('search and local import contracts',()=>{
 it('combines parent category, destination account, exact hashtag and refund amount without floating comparison',()=>{
  const filters={...emptyLedgerSearchFilters(),minimum:'12.34',maximum:'12.34',categoryIds:['p'],accountIds:['destination']};
  const match=entry({amount:'-12.34',destinationAccountId:'destination'});
  expect(filterLedgerEntries([match,entry({tags:['工作日']})],'#工作 公司',filters)).toEqual([match]);
  filters.minimum='12.35';expect(filterLedgerEntries([match],'',filters)).toEqual([]);
 });
 it('distinguishes any/all conditions and preserves named places',()=>{
  const facts=[entry({id:'picture',pictures:[{pictureId:'p'}] as LedgerEntry['pictures']}),entry({id:'both',location:'上海公司',pictures:[{pictureId:'p'}] as LedgerEntry['pictures']})];
  const filters={...emptyLedgerSearchFilters(),flags:['picture','location']};
  expect(filterLedgerEntries(facts,'',filters)).toHaveLength(2);filters.matchAll=true;expect(filterLedgerEntries(facts,'',filters).map(x=>x.id)).toEqual(['both']);
 });
 it('does not turn totals, ambiguous amounts or missing categories into confirmed OCR bills',()=>{
  const rows=parseBillOCR('2026年10月05日\n午餐 -32.50\n退款 +18.25\n总计 -50.75\n余额 ￥900.00\n两项 -1.00 -2.00',zone);
  expect(rows).toHaveLength(2);expect(rows.map(x=>[x.type,x.sourceAmount,x.sourceAccountId,x.categoryId])).toEqual([[3,3250,'',''],[2,1825,'','']]);
  expect(moment.unix(rows[0]!.time).tz(zone).format('YYYY-MM-DD')).toBe('2026-10-05');
 });
 it('keeps spreadsheet IDs and formula-like notes as text, escapes XML, and guards CSV formulas',()=>{
  const note='=SUM(1,2) & <note>';
  expect(ledgerEntriesCSV([entry({comment:note})],zone)).toContain("'=SUM(1,2)");
  const files=unzipSync(ledgerWorkbook([['ID','备注'],['9007199254740993123',note]]));
  const xml=strFromU8(files['xl/worksheets/sheet1.xml']!);
  expect(xml).toContain('9007199254740993123');expect(xml).toContain('&amp; &lt;note&gt;');expect(xml).not.toContain('<f>');
 });
});
describe('wish progress derived from original facts',()=>{
 it('stops scheduled contributions at the deadline and never mutates transactions',()=>{
  const wish=newLedgerWish(zone).data;Object.assign(wish,{target:'1000',initial:'10.10',mode:'schedule',cycle:'month',amount:'0.20',startDate:'2026-01-31',endDate:'2026-03-31'});
  expect(wishProgress(wish,[],{},moment.tz('2026-10-05',zone)).saved).toBe('10.70');
 });
 it('excludes other books, refunds spending and propagates missing historical FX',()=>{
  const wish=newLedgerWish(zone).data;Object.assign(wish,{target:'1000',mode:'balance',ratio:'50',bookIds:['b'],startDate:'2026-10-01'});
  const facts=[entry({type:2,amount:'100',cny:'100'}),entry({amount:'20',cny:'20'}),entry({amount:'-5',cny:'-5'}),entry({bookId:'other',cny:'999'})];
  const before=JSON.stringify(facts);expect(wishProgress(wish,facts,{},moment.tz('2026-10-06',zone)).saved).toBe('42.50');expect(JSON.stringify(facts)).toBe(before);
  expect(wishProgress(wish,[...facts,entry({currency:'USD',cny:null})],{},moment.tz('2026-10-06',zone)).saved).toBeNull();
 });
});
