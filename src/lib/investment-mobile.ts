import {monetaryIncome,monetaryIncomeRevision,type MonetaryBinding} from '@/lib/monetary-income.ts';
import {computed,ref,watch} from 'vue';
import {investments,investmentError} from '@/lib/investments.ts';
import {LedgerDecimal} from '@/lib/ledger-display.ts';
import {useBooksStore} from '@/stores/books.ts';
import {useTransactionsStore} from '@/stores/transaction.ts';
import type {HoldingProfile,Instrument,InvestmentAccount,InvestmentEvent,InvestmentOrder,InvestmentPosition,WealthSummary} from '@/models/investment.ts';

export const investmentNames:Record<string,string>={OPENING:'初始持仓',BUY:'买入',SELL:'卖出',TRANSFER:'转移',INCOME:'收入',EXPENSE:'支出',ADJUST:'持仓校准'};
export const investmentTypes=[{key:'FUND',name:'基金',market:'CN_FUND'},{key:'CN',name:'股票（沪深）',market:'CN'},{key:'HK',name:'港股',market:'HK'},{key:'US',name:'美股',market:'US'},{key:'CRYPTO',name:'加密货币',market:'CRYPTO'},{key:'METAL',name:'期货 / 贵金属',market:''},{key:'OTHER',name:'自定义理财',market:''}];
export function blankHolding(accountId='',instrumentId=''):HoldingProfile{return {id:'',accountId,instrumentId,name:'',group:'',note:'',profitOffset:'0',hidden:false,excludeFromTotal:false,excludeProfit:false,bookIds:[],version:0};}
export function investmentGroup(asset?:Instrument):string{if(asset?.type==='CRYPTO')return '加密货币';if(asset?.type==='FUND')return '基金';if(asset?.type==='STOCK')return asset.market==='HK'?'港股':asset.market==='US'?'美股':'股票';return '其他理财';}
export function investmentSum(values:(string|null|undefined)[]):string|null{return values.some(v=>v==null)?null:values.reduce((a,v)=>a.plus(v!),new LedgerDecimal(0)).toString();}
export function profitClass(v:string|null|undefined):string{return v!=null?new LedgerDecimal(v).gt(0)?'inv-up':new LedgerDecimal(v).lt(0)?'inv-down':'':'';}
export function validInvestmentNumber(raw:string,name:string,positive=false,signed=false):string{if(raw.length>80||!(signed?/^-?\d+(\.\d{1,18})?$/:/^\d+(\.\d{1,18})?$/).test(raw)||positive&&!new LedgerDecimal(raw).gt(0))throw Error(`${name}请填写${positive?'大于零的':''}数字，最多18位小数`);return raw;}
export function invalidateInvestmentData():void{useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});}
export interface HoldingRow{key:string;position:InvestmentPosition;profile:HoldingProfile;asset?:Instrument;account?:InvestmentAccount;name:string;group:string;profit:string|null}
export function useInvestmentData(){
    const wealth=ref<WealthSummary>(),assets=ref<Instrument[]>([]),accounts=ref<InvestmentAccount[]>([]),profiles=ref<HoldingProfile[]>([]),events=ref<InvestmentEvent[]>([]),monetary=ref<MonetaryBinding[]>([]),orders=ref<InvestmentOrder[]>([]),loading=ref(false),error=ref(''),zone=ref('Asia/Shanghai'),books=useBooksStore();
    async function load():Promise<void>{if(loading.value)return;loading.value=true;error.value='';try{const [w,a,ac,p,e,o,s,m]=await Promise.all([investments.summary(),investments.instruments(),investments.accounts(),investments.profiles(),investments.events(),investments.orders(),investments.settings(),monetaryIncome.list(),books.loadBooks()]);monetary.value=m;wealth.value=w;assets.value=a;accounts.value=ac;profiles.value=p;events.value=e;orders.value=o;zone.value=s.timeZone||zone.value;}catch(cause){error.value=investmentError(cause);}finally{loading.value=false;}}
    watch(monetaryIncomeRevision,()=>{if(!document.hidden)void load();});
    const rows=computed<HoldingRow[]>(()=>{
        const positions=[...(wealth.value?.positions||[])];
        for(const p of profiles.value)if(!positions.some(v=>v.accountId===p.accountId&&v.instrumentId===p.instrumentId))positions.push({accountId:p.accountId,instrumentId:p.instrumentId,quantity:'0',cost:'0',costKnown:true,averageCost:null,realizedPnl:'0',marketValue:'0',unrealizedPnl:'0'});
        return positions.map(position=>{const asset=assets.value.find(a=>a.id===position.instrumentId),account=accounts.value.find(a=>a.id===position.accountId),profile=profiles.value.find(p=>p.accountId===position.accountId&&p.instrumentId===position.instrumentId)||blankHolding(position.accountId,position.instrumentId);return {key:position.accountId+':'+position.instrumentId,position,asset,account,profile,name:profile.name||asset?.name||position.instrumentId,group:profile.group||investmentGroup(asset),profit:investmentSum([position.unrealizedPnl,position.realizedPnl,profile.profitOffset])};});
    });
    return {monetary,wealth,assets,accounts,profiles,events,orders,loading,error,zone,books,load,rows};
}

let planSyncPending=false,lastPlanSync=0;
export async function syncInvestmentPlansOnOpen():Promise<void>{
 const {isUserLogined,isUserUnlocked}=await import('@/lib/userstate.ts');
 if(planSyncPending||document.hidden||!isUserLogined()||!isUserUnlocked()||Date.now()-lastPlanSync<60000)return;
 planSyncPending=true;lastPlanSync=Date.now();
 try{const result=await investments.syncPlans();if(result.created)invalidateInvestmentData();}catch{/* 定投页显示待处理原因并提供重试。 */}finally{planSyncPending=false;}
}
