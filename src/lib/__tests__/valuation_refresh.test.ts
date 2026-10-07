import {afterEach,describe,expect,it,vi} from 'vitest';
import type {WealthSummary} from '@/models/investment.ts';
const {summary}=vi.hoisted(()=>({summary:vi.fn()}));
vi.mock('@/lib/investments.ts',()=>({investments:{summary}}));
import {createValuationRefresh} from '@/lib/valuation-refresh.ts';
afterEach(()=>{vi.useRealTimers();vi.unstubAllGlobals();summary.mockReset();});
describe('foreground valuation refresh',()=>{
 it('reads at entry and five seconds, pauses while hidden, and resumes immediately',async()=>{
  vi.useFakeTimers();const doc=Object.assign(new EventTarget(),{hidden:false});vi.stubGlobal('document',doc);
  summary.mockResolvedValue({positions:[]} as unknown as WealthSummary);
  const apply=vi.fn(),live=createValuationRefresh(apply);live.start();await vi.advanceTimersByTimeAsync(0);
  expect(apply).toHaveBeenCalledTimes(1);await vi.advanceTimersByTimeAsync(5000);expect(apply).toHaveBeenCalledTimes(2);
  doc.hidden=true;await vi.advanceTimersByTimeAsync(10000);expect(summary).toHaveBeenCalledTimes(2);
  doc.hidden=false;doc.dispatchEvent(new Event('visibilitychange'));await vi.advanceTimersByTimeAsync(0);expect(apply).toHaveBeenCalledTimes(3);
  live.stop();await vi.advanceTimersByTimeAsync(10000);expect(summary).toHaveBeenCalledTimes(3);
 });
 it('discards a reply from the page that has already closed',async()=>{
  vi.useFakeTimers();vi.stubGlobal('document',Object.assign(new EventTarget(),{hidden:false}));
  let resolve!:(v:WealthSummary)=>void;summary.mockImplementationOnce(()=>new Promise<WealthSummary>(r=>resolve=r));
  const apply=vi.fn(),live=createValuationRefresh(apply);live.start();live.stop();resolve({positions:[]} as unknown as WealthSummary);
  await vi.advanceTimersByTimeAsync(0);expect(apply).not.toHaveBeenCalled();
 });
});
