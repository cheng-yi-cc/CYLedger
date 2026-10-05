import axios from 'axios';
import {creditAccounts,type CreditReport} from '@/lib/credit-accounts.ts';
import {debtAccounts,type DebtReport} from '@/lib/debt-accounts.ts';
import '@/lib/services.ts';
import moment from 'moment-timezone';
import type { ApiResponse } from '@/core/api.ts';
import type { CalendarEvent } from '@/lib/calendar-events.ts';
import type { WealthSummary,InvestmentAccount } from '@/models/investment.ts';
import type { Account } from '@/models/account.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
export interface AssetRule { hidden:boolean; disabledBooks:string[] }
export interface AssetPreferences { revision:string; rules:Record<string,AssetRule>; reminders:{enabled:boolean;credit:boolean;debt:boolean;deposit:boolean;advanceDays:number;minuteOfDay:number} }
export interface FixedDeposit {receivedInterest?:string;closedDate?:string;id:string;accountId:string;bookId:string;categoryId:string;currency:string;principal:string;annualRate:string;term:number;unit:'day'|'month'|'year';startDate:string;maturityDate:string;expectedInterest:string;timeZone:string;note:string;settled:boolean;transactionId:string;closed:boolean}
export const defaultAssetPreferences=():AssetPreferences=>({revision:'0',rules:{},reminders:{enabled:false,credit:true,debt:true,deposit:true,advanceDays:0,minuteOfDay:540}});
async function request<T>(method:'get'|'post',path:string,data?:unknown):Promise<T>{const r=await axios.request<ApiResponse<T>>({method,url:`v1/${path}`,data});if(!r.data.success)throw Error('资产操作失败，请重试');return r.data.result;}
export const assetTools={adjustBalance:(data:{accountId:string;balance:string;expectedBalance:string;bookId:string;countInStatistics:boolean;timeZone:string;requestId:string})=>request<{transactionId:string}>('post','assets/adjust-balance',data),preferences:()=>request<AssetPreferences>('get','assets/preferences'),savePreferences:(data:AssetPreferences)=>request<AssetPreferences>('post','assets/preferences',data),deposits:()=>request<FixedDeposit[]>('get','assets/deposits'),saveDeposit:(data:FixedDeposit)=>request<FixedDeposit>('post','assets/deposits/save',data),closeDeposit:(id:string)=>request<boolean>('post','assets/deposits/close',{id}),syncDeposits:()=>request<number>('post','assets/deposits/sync',{}),pending:()=>request<CalendarEvent[]>('get','calendar/pending')};
export interface AssetItem {key:string;id:string;name:string;category:number;currency:string;balance:string|null;value:string|null;href:string;portfolio:boolean;hidden:boolean;unrealizedPnl:string|null;excluded?:boolean;group?:string;kind?:string}
export function assetItems(summary:WealthSummary|undefined,portfolios:InvestmentAccount[],accounts:Record<string,Account>,preferences:AssetPreferences):AssetItem[]{
 if(!summary)return [];
 const result:AssetItem[]=summary.cashAccounts.map(a=>({key:`cash:${a.id}`,id:a.id,name:a.name,category:accounts[a.id]?.category||0,excluded:!!accounts[a.id]?.assetProfile.excludeFromTotal,group:accounts[a.id]?.assetProfile.group,kind:accounts[a.id]?.assetProfile.kind,currency:a.currency,balance:a.balance,value:a.value,href:accounts[a.id]?.assetProfile.kind==='reimbursement'?`/assets/reimbursements?id=${a.id}`:[5,6].includes(accounts[a.id]?.category||0)?`/account/debt?id=${a.id}`:`/account/detail?id=${a.id}`,portfolio:false,hidden:preferences.rules[`cash:${a.id}`]?.hidden||accounts[a.id]?.hidden||false,unrealizedPnl:'0'}));
 for(const a of portfolios){const p=summary.positions.filter(p=>p.accountId===a.id&&new LedgerDecimal(p.quantity).gt(0));const value=decimalSum(p.map(p=>p.marketValue));result.push({key:`portfolio:${a.id}`,id:a.id,name:a.name,category:7,currency:'CNY',balance:value,value,href:['WALLET','EXCHANGE'].includes(a.kind)?`/crypto/account?id=${a.id}`:`/investments/ledger?accountId=${a.id}`,portfolio:true,hidden:preferences.rules[`portfolio:${a.id}`]?.hidden||false,unrealizedPnl:decimalSum(p.map(p=>p.unrealizedPnl))});}
 return result;
}
export function decimalSum(values:(string|null|undefined)[]):string|null{return values.some(v=>v==null)?null:values.reduce((sum,v)=>sum.plus(v!),new LedgerDecimal(0)).toString();}
export function assetTotals(items:AssetItem[]){const live=items.filter(a=>!a.hidden&&!a.excluded),positive=live.filter(a=>a.value!=null&&!new LedgerDecimal(a.value).lt(0)),negative=live.filter(a=>a.value!=null&&new LedgerDecimal(a.value).lt(0)),unknown=live.some(a=>a.value==null);return {assets:unknown?null:decimalSum(positive.map(a=>a.value)),liabilities:unknown?null:decimalSum(negative.map(a=>new LedgerDecimal(a.value!).abs().toString())),net:unknown?null:decimalSum(live.map(a=>a.value)),missing:live.filter(a=>a.value==null).length};}
interface NativeReminderBridge {status():string;requestPermission():void;replace(json:string):void;clear():void;test():void}
declare global {interface Window{CYLedgerReminders?:NativeReminderBridge}}
let automationPending=false,lastAutomation=0;
export async function syncAssetAutomationOnOpen(zone:string,force=false):Promise<void>{
 if(automationPending||document.hidden||(!force&&Date.now()-lastAutomation<60000))return;
 const {isUserLogined,isUserUnlocked}=await import('@/lib/userstate.ts');if(!isUserLogined()||!isUserUnlocked())return;
 automationPending=true;lastAutomation=Date.now();
 try{const n=(await assetTools.syncDeposits())+(await creditAccounts.sync());const [{useAccountsStore},{useAssetToolsStore},{useTransactionsStore}]=await Promise.all([import('@/stores/account.ts'),import('@/stores/assetTools.ts'),import('@/stores/transaction.ts')]);if(n>0)useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});const accounts=useAccountsStore(),preferences=useAssetToolsStore();await preferences.load(force);try{await accounts.loadAllAccounts({force:n>0});}catch(e){if(!e||typeof e!=='object'||!('isUpToDate' in e))throw e;}await updateNativeReminders(preferences.preferences,accounts.allAccountsMap,zone);}
 finally{automationPending=false;}
}
export function notificationStatus():string{return window.CYLedgerReminders?.status()||'browser';}
export async function updateNativeReminders(preferences:AssetPreferences,accounts:Record<string,Account>,zone:string):Promise<void>{
 const bridge=window.CYLedgerReminders;if(!bridge)return;
 if(!preferences.reminders.enabled){bridge.clear();return;}
 const r=preferences.reminders,[manual,reports,debts]=await Promise.all([assetTools.pending(),creditAccounts.reports(zone),debtAccounts.reports()]),items=mergeDebtDue(mergeCreditDue(manual,reports),debts,accounts),now=Date.now();
 const reminders=items.filter(i=>i.kind==='deposit'?r.deposit:accounts[i.accountId]?.category===3?r.credit:r.debt).map(i=>({id:i.id,at:moment.tz(i.date,'YYYY-MM-DD',zone).subtract(r.advanceDays,'day').add(r.minuteOfDay,'minute').valueOf(),title:i.id.startsWith('annual:')?'信用卡年费提醒':i.kind==='deposit'?'定期存款到期提醒':'还款提醒',text:`${i.accountName} · ${i.date}到期 · ${i.currency} ${i.amount}`})).filter(i=>i.at>now);
 bridge.replace(JSON.stringify(reminders));
}

// Computed bills follow real repayments and never create a second ledger entry.
export function mergeCreditDue(manual:CalendarEvent[],reports:CreditReport[]):CalendarEvent[]{
 const result=[...manual];
 for(const report of reports){
  for(const row of report.statements){
   const amount=new LedgerDecimal(row.remaining).plus(row.futureFees);
   if(!row.dueDate||!amount.gt(0)||manual.some(x=>x.kind==='repayment'&&x.accountId===report.accountId&&x.date===row.dueDate))continue;
   result.push({id:`credit:${report.accountId}:${row.month}`,accountId:report.accountId,accountName:report.accountName,bookId:'',currency:report.currency,kind:'repayment',date:row.dueDate,amount:amount.toFixed(2),note:'信用卡账单',completed:false});
  }
  if(report.annualDate&&!report.annualWaived&&new LedgerDecimal(report.annualFee||0).gt(0))result.push({id:`annual:${report.accountId}`,accountId:report.accountId,accountName:report.accountName,bookId:'',currency:report.currency,kind:'repayment',date:report.annualDate,amount:report.annualFee,note:'信用卡年费',completed:false});
 }
 return result.sort((a,b)=>a.date.localeCompare(b.date));
}

export function mergeDebtDue(events:CalendarEvent[],reports:DebtReport[],accounts:Record<string,Account>):CalendarEvent[]{
 const result=[...events];
 for(const report of reports){
  const account=accounts[report.accountId];
  if(!account||account.category!==5||!report.nextDate||result.some(e=>e.accountId===account.id&&e.date===report.nextDate&&e.kind==='repayment'))continue;
  const amount=new LedgerDecimal(account.balance).div(100).neg();
  if(amount.gt(0))result.push({id:`debt:${account.id}`,accountId:account.id,accountName:account.name,bookId:'',currency:account.currency,kind:'repayment',date:report.nextDate,amount:amount.toFixed(2),note:'借入待还',completed:false});
 }
 return result.sort((a,b)=>a.date.localeCompare(b.date));
}
