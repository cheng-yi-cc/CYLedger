<template>
 <f7-page class="cy-main-page cy-mobile-surface" @page:beforein="load">
  <f7-navbar :title="original?'钱包收支记录':'钱包记账'" back-link="返回" />
  <main class="cy-page-body wallet-entry">
   <p v-if="error" class="error" role="alert">{{error}}</p>
   <p v-if="loading" class="hint">正在读取账户与持仓…</p>
   <form v-if="ready" @submit.prevent="save"><fieldset :disabled="busy || original?.voided">
    <section class="cy-panel">
     <div class="entry-types" role="group" aria-label="钱包记账类型"><button type="button" :aria-pressed="type==='EXPENSE'" @click="changeType('EXPENSE')">支出</button><button type="button" :aria-pressed="type==='INCOME'" @click="changeType('INCOME')">收入</button></div>
     <p class="account-name">{{account?.name}}</p>
     <BookPicker v-model="bookId" />
     <label>分类<select v-model="categoryId" aria-label="钱包收支分类" required><option value="" disabled>请选择分类</option><option v-for="category in categories" :key="category.id" :value="category.id">{{category.name}}</option></select></label>
     <label>货币单位<select v-model="currency" :disabled="!!original" aria-label="收支货币单位" @change="changeCurrency"><option value="USD">美元（USD）</option><option value="CNY">人民币（CNY）</option></select></label>
     <label class="amount-label">{{type==='EXPENSE'?'实际支出':'实际收入'}}（{{currency}}）<input v-model="amount" required inputmode="decimal" autocomplete="off" maxlength="32" aria-label="钱包收支金额" placeholder="0.00" @input="suggest" /></label>
     <label>时间<input v-model="dateTime" required type="datetime-local" step="1" aria-label="钱包收支时间" @change="changeDate" /></label>
     <label>备注<input v-model="note" maxlength="255" aria-label="钱包收支备注" placeholder="选填" /></label>
    </section>
    <section class="cy-panel"><h2>{{type==='EXPENSE'?'实际扣除币种':'实际收到币种'}}</h2>
     <label>{{type==='EXPENSE'?'优先使用':'收款币种'}}<select v-model="primary" required aria-label="本笔优先币种" @change="changeCoins"><option value="" disabled>请选择币种</option><option v-for="coin in coins" :key="coin.id" :value="coin.id">{{coin.symbol}} · {{coin.name}}</option></select></label>
     <label v-if="type==='EXPENSE'">不足时使用<select v-model="backup" aria-label="本笔备用币种" @change="changeCoins"><option value="">不使用第二种币</option><option v-for="coin in coins.filter(c=>c.id!==primary)" :key="coin.id" :value="coin.id">{{coin.symbol}} · {{coin.name}}</option></select></label>
     <label v-if="primary">{{symbol(primary)}} {{type==='EXPENSE'?'实扣':'实收'}}数量<input v-model="primaryQuantity" inputmode="decimal" maxlength="80" aria-label="优先币种实际数量" @input="manual=true;allocationError=''" /></label>
     <p v-if="primary && type==='EXPENSE'" class="hint">可用 {{available(primary)}} {{symbol(primary)}}</p>
     <label v-if="backup && type==='EXPENSE'">{{symbol(backup)}} 实扣数量<input v-model="backupQuantity" inputmode="decimal" maxlength="80" aria-label="备用币种实际数量" @input="manual=true;allocationError=''" /></label>
     <p v-if="backup && type==='EXPENSE'" class="hint">可用 {{available(backup)}} {{symbol(backup)}}</p>
     <p class="hint">默认按 1 枚约 1 美元预填。请以实际扣款或到账为准，可直接修改数量；手续费可计入实际金额及数量。</p>
     <p v-if="currency==='CNY'" class="hint">人民币金额按参考汇率 {{allocationRate || '暂缺'}} 估算美元币种数量；历史日期请填写实际数量。</p>
     <button class="text-button" type="button" @click="manual=false;suggest()">{{type==='EXPENSE'?'重新按顺序分配':'重新计算收款数量'}}</button>
     <p v-if="allocationError" class="error" role="alert">{{allocationError}}</p>
    </section>
    <section class="cy-panel"><h2>人民币折算</h2>
     <label v-if="currency==='USD'">1 USD 兑换人民币<input v-model="exchangeRate" required inputmode="decimal" maxlength="80" aria-label="本笔美元人民币汇率" @input="confirmRate" /></label>
     <p class="hint">{{fxSource || '请填写本笔发生时的汇率'}} · {{fxDate || '日期待确认'}}</p>
     <p v-if="fxStale && !original" class="hint">当前参考汇率已过期，请核对后保存。</p>
     <strong class="cny-preview">折合 {{currencyMoney(cnyAmount,'CNY')}}</strong>
     <p class="hint">汇率随本笔记录保存，后续行情变化不改动历史收支。</p>
    </section>
    <button type="submit" class="primary" :disabled="busy || !canSave">{{busy?'正在保存…':original?'保存修订':type==='EXPENSE'?'保存支出':'保存收入'}}</button>
   </fieldset></form>
   <p v-if="original?.voided" class="hint">此记录已撤销，持仓与账单已同步回退。</p>
   <button v-if="original && !original.voided" class="void-button" :disabled="busy" @click="confirmVoid">撤销这笔收支</button>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue';
import {f7} from 'framework7-vue';
import moment from 'moment-timezone';
import type {Router} from 'framework7/types';
import {investments,investmentError} from '@/lib/investments.ts';
import {cryptoInput} from '@/lib/crypto-entry.ts';
import {LedgerDecimal,keepUpToDate} from '@/lib/mobile-ledger.ts';
import {allocateWalletPayment,currencyMoney,suggestedPaymentCoins,usdFX,walletCurrency} from '@/lib/wallet-entry.ts';
import {generateRandomUUID} from '@/lib/misc.ts';
import {useBooksStore} from '@/stores/books.ts';
import {useTransactionsStore} from '@/stores/transaction.ts';
import {useTransactionCategoriesStore} from '@/stores/transactionCategory.ts';
import {CategoryType} from '@/core/category.ts';
import type {AssetMovement,Instrument,InvestmentAccount,InvestmentEvent,WealthSummary} from '@/models/investment.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const books=useBooksStore(),categoryStore=useTransactionCategoriesStore();
const account=ref<InvestmentAccount>(),allCoins=ref<Instrument[]>([]),summary=ref<WealthSummary>(),original=ref<InvestmentEvent>();
const ready=ref(false),loading=ref(false),busy=ref(false),error=ref(''),allocationError=ref('');
const type=ref<'INCOME'|'EXPENSE'>('EXPENSE'),currency=ref<'USD'|'CNY'>('USD');
const amount=ref(''),categoryId=ref(''),bookId=ref(''),note=ref(''),dateTime=ref(''),zone=ref('Asia/Shanghai');
const primary=ref(''),backup=ref(''),primaryQuantity=ref(''),backupQuantity=ref(''),manual=ref(false);
const exchangeRate=ref(''),fxDate=ref(''),fxSource=ref('');
let requestKey=generateRandomUUID(),lastPayload='';
const coins=computed(()=>allCoins.value.filter(c=>c.type==='CRYPTO'&&((account.value?.instruments||[]).includes(c.id)||summary.value?.positions.some(p=>p.accountId===account.value?.id&&p.instrumentId===c.id)||original.value?.instrumentId===c.id||original.value?.additionalMovements?.some(m=>m.instrumentId===c.id))));
const categories=computed(()=>Object.values(categoryStore.allTransactionCategoriesMap).filter(c=>c.parentId!=='0'&&c.visible&&c.type===(type.value==='INCOME'?CategoryType.Income:CategoryType.Expense)).map(c=>({id:c.id,name:`${categoryStore.allTransactionCategoriesMap[c.parentId]?.name||''} / ${c.name}`})));
const positions=computed(()=>{
 const items=(summary.value?.positions||[]).map(p=>({...p}));
 if(original.value?.type==='EXPENSE'&&!original.value.voided){for(const leg of [{instrumentId:original.value.instrumentId,quantity:original.value.quantity},...(original.value.additionalMovements||[])]){const p=items.find(p=>p.accountId===account.value?.id&&p.instrumentId===leg.instrumentId);if(p)p.quantity=new LedgerDecimal(p.quantity).plus(leg.quantity).toFixed();}}
 return items;
});
const fxStale=computed(()=>usdFX(summary.value)?.state==='stale');
const allocationRate=computed(()=>currency.value==='USD'?exchangeRate.value:currentDay()===moment().tz(zone.value).format('YYYY-MM-DD')?usdFX(summary.value)?.rate||'':'');
const cnyAmount=computed(()=>{try{return new LedgerDecimal(cryptoInput(amount.value,2)).mul(cryptoInput(exchangeRate.value)).toDecimalPlaces(2,LedgerDecimal.ROUND_HALF_UP).toFixed()}catch{return null}});
const canSave=computed(()=>!!amount.value&&!!categoryId.value&&!!bookId.value&&!!primary.value&&cnyAmount.value!==null&&!allocationError.value);
function symbol(id:string):string{return allCoins.value.find(c=>c.id===id)?.symbol||''}
function available(id:string):string{return positions.value.find(p=>p.accountId===account.value?.id&&p.instrumentId===id)?.quantity||'0'}
function currentDay():string{return dateTime.value.slice(0,10)}
function initializeRate():void{
 if(currency.value==='CNY'){exchangeRate.value='1';fxDate.value=currentDay();fxSource.value='人民币';return}
 const fx=usdFX(summary.value);
 if(currentDay()===moment().tz(zone.value).format('YYYY-MM-DD')&&fx){exchangeRate.value=fx.rate;fxDate.value=fx.date;fxSource.value=fx.source}
 else{exchangeRate.value='';fxDate.value=currentDay();fxSource.value=''}
}
function suggest():void{
 if(manual.value)return;
 allocationError.value='';primaryQuantity.value='';backupQuantity.value='';
 if(!amount.value||!primary.value)return;
 try{
  if(currency.value==='CNY'&&!allocationRate.value)throw Error('缺少本笔美元参考汇率，请填写实际币种数量');
  if(type.value==='INCOME'){let value=new LedgerDecimal(cryptoInput(amount.value,2));if(currency.value==='CNY')value=value.div(cryptoInput(allocationRate.value));primaryQuantity.value=value.toDecimalPlaces(18,LedgerDecimal.ROUND_DOWN).toFixed();return}
  const legs=allocateWalletPayment(amount.value,currency.value,allocationRate.value,[primary.value,backup.value],positions.value,account.value!.id);
  primaryQuantity.value=legs.find(l=>l.instrumentId===primary.value)?.quantity||'0';backupQuantity.value=legs.find(l=>l.instrumentId===backup.value)?.quantity||'0';
 }catch(e){allocationError.value=investmentError(e)}
}
function changeCoins():void{if(primary.value===backup.value)backup.value='';manual.value=false;suggest()}
function changeCurrency():void{amount.value='';manual.value=false;initializeRate();suggest()}
function changeDate():void{initializeRate();suggest()}
function confirmRate():void{fxSource.value='用户确认';fxDate.value=currentDay();suggest()}
function changeType(next:'INCOME'|'EXPENSE'):void{if(type.value===next)return;type.value=next;categoryId.value='';manual.value=false;suggest()}
function parsedMovements():AssetMovement[]{
 const values=[{instrumentId:primary.value,quantity:primaryQuantity.value},...(type.value==='EXPENSE'&&backup.value?[{instrumentId:backup.value,quantity:backupQuantity.value}]:[])];
 const legs=values.filter(v=>!/^0*(\.0+)?$/.test(v.quantity.trim())).map(v=>({instrumentId:v.instrumentId,quantity:cryptoInput(v.quantity)}));
 if(!legs.length)throw Error('请填写实际扣除或收到的币种数量');
 return legs;
}
function refreshStores():void{useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true})}
async function save():Promise<void>{
 if(busy.value)return;busy.value=true;error.value='';
 try{
  const legs=parsedMovements(),first=legs[0]!;
  const input:InvestmentEvent={id:original.value?.id||'',version:original.value?.version||0,type:type.value,accountId:account.value!.id,instrumentId:first.instrumentId,quantity:first.quantity,additionalMovements:legs.slice(1),amount:cryptoInput(amount.value,2),fee:'0',cost:null,exchangeRate:cryptoInput(exchangeRate.value),occurredAt:moment.tz(dateTime.value,zone.value).unix(),note:note.value,bookId:bookId.value,voided:false,toAccountId:'',settlementAccountId:'',settlementInstrumentId:'',cashAccountId:'',wallet:{currency:currency.value,categoryId:categoryId.value,fxDate:fxDate.value,fxSource:fxSource.value}};
  if(!input.wallet!.fxSource)throw Error('请确认本笔收支汇率');
  const payload=JSON.stringify(input);if(payload!==lastPayload){requestKey=generateRandomUUID();lastPayload=payload}
  await investments.saveEvent(input,requestKey);refreshStores();props.f7router.back();
 }catch(e){error.value=investmentError(e)}finally{busy.value=false}
}
function confirmVoid():void{f7.dialog.confirm('撤销后，这笔收支及对应币种数量会一起回退。','撤销钱包收支',()=>{void voidEntry()})}
async function voidEntry():Promise<void>{if(busy.value||!original.value)return;busy.value=true;error.value='';try{await investments.voidEvent(original.value);refreshStores();props.f7router.back()}catch(e){error.value=investmentError(e)}finally{busy.value=false}}
async function load():Promise<void>{
 if(ready.value||loading.value)return;loading.value=true;error.value='';
 try{
  const q=props.f7route.query;
  const [accounts,assets,wealth,settings,events]=await Promise.all([investments.accounts(),investments.instruments(),investments.summary(),investments.settings(),q['eventId']?investments.events():Promise.resolve([]),books.loadBooks(),categoryStore.loadAllCategories({force:false}).catch(keepUpToDate)]);
  summary.value=wealth;allCoins.value=assets;zone.value=settings.timeZone||zone.value;
  if(q['eventId']){original.value=events.find(e=>e.id===q['eventId']&&e.wallet);if(!original.value)throw Error('找不到这笔钱包收支')}
  account.value=accounts.find(a=>a.id===(original.value?.accountId||q['accountId']));if(!account.value)throw Error('找不到这个钱包账户');
  const defaults=suggestedPaymentCoins(account.value,assets);primary.value=defaults[0]||'';backup.value=defaults[1]||'';
  if(original.value){const e=original.value;type.value=e.type==='INCOME'?'INCOME':'EXPENSE';currency.value=e.wallet!.currency;amount.value=e.amount;categoryId.value=e.wallet!.categoryId;bookId.value=e.bookId||books.defaultBookId;note.value=e.note;dateTime.value=moment.unix(e.occurredAt).tz(zone.value).format('YYYY-MM-DDTHH:mm:ss');exchangeRate.value=e.exchangeRate;fxDate.value=e.wallet!.fxDate;fxSource.value=e.wallet!.fxSource;primary.value=e.instrumentId;primaryQuantity.value=e.quantity;backup.value=e.additionalMovements?.[0]?.instrumentId||'';backupQuantity.value=e.additionalMovements?.[0]?.quantity||'';manual.value=true}
  else{type.value=q['type']==='INCOME'?'INCOME':'EXPENSE';currency.value=walletCurrency(account.value);amount.value=q['amount']||'';categoryId.value=q['categoryId']||'';bookId.value=q['bookId']||books.defaultBookId;note.value=q['note']||'';dateTime.value=(q['time']?moment.unix(Number(q['time'])):moment()).tz(zone.value).format('YYYY-MM-DDTHH:mm:ss');initializeRate();suggest()}
  ready.value=true;
 }catch(e){error.value=investmentError(e)}finally{loading.value=false}
}
</script>
<style scoped>
.wallet-entry{max-width:640px;margin:auto;padding-bottom:35px}fieldset{margin:0;padding:0;border:0;min-width:0}h2{font-size:17px;margin:0 0 15px}.account-name{font-size:17px;margin:18px 0}.entry-types{display:flex;gap:8px}.entry-types button{flex:1;border:1px solid var(--cy-line);border-radius:10px;background:var(--cy-card);color:var(--cy-ink);padding:12px;font:inherit}.entry-types button[aria-pressed=true]{color:var(--cy-accent);background:var(--cy-soft);border-color:var(--cy-accent)}label{display:grid;gap:9px;font-size:13px;margin-top:18px}input,select{box-sizing:border-box;width:100%;min-height:46px;padding:11px;border:1px solid var(--cy-line);border-radius:10px;font:inherit;background:var(--cy-card);color:inherit}.amount-label input{font-size:25px}.hint{font-size:12px;color:var(--cy-muted);line-height:1.8;margin-top:10px}.error{color:var(--cy-expense);font-size:13px;line-height:1.6}.text-button{padding:10px 0;border:0;background:none;color:var(--cy-accent);font:inherit;font-size:13px}.cny-preview{display:block;font-size:20px;margin:15px 0}.primary{width:100%;border:0;border-radius:12px;padding:15px;background:var(--cy-accent);color:white;font:inherit}.primary:disabled{opacity:.5}.void-button{width:100%;margin-top:20px;padding:13px;border:1px solid var(--cy-line);border-radius:10px;background:var(--cy-card);color:var(--cy-expense);font:inherit}
</style>
