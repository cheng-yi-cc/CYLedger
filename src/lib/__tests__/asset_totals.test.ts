import {describe,it,expect,vi} from 'vitest';
vi.mock('@/lib/services.ts',()=>({default:{}}));
import {assetTotals,type AssetItem} from '@/lib/asset-tools.ts';
const item=(value:string|null,hidden=false,excluded=false):AssetItem=>({key:'cash:a',id:'a',name:'账户',category:1,currency:'CNY',balance:value,value,href:'',portfolio:false,hidden,excluded,unrealizedPnl:'0'});
describe('hidden account totals',()=>{
 it('includes hidden assets and debts but obeys the independent exclusion setting',()=>{
  expect(assetTotals([item('100',true),item('-30',true),item('500',false,true)])).toEqual({assets:'100',liabilities:'30',net:'70',missing:0});
 });
 it('does not turn a hidden unknown valuation into a known total',()=>{
  expect(assetTotals([item('100'),item(null,true)])).toMatchObject({net:null,missing:1});
  expect(assetTotals([item('100'),item(null,true,true)])).toMatchObject({net:'100',missing:0});
 });
});
