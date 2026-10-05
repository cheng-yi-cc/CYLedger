<template>
 <f7-page class="cy-main-page cy-mobile-surface" @page:beforein="load" @page:beforeout="leave">
  <f7-navbar :title="title" back-link="返回" />
  <main class="cy-page-body crypto-convert">
   <p v-if="loadError" class="error" role="alert">{{ loadError }} <button type="button" @click="load">重试</button></p>
   <form @submit.prevent="save"><fieldset :disabled="busy || !initialized">
    <section class="cy-panel"><BookPicker v-model="bookId" />
     <label v-if="mode==='opening'">持仓账户<select v-model="toAccount" required><option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
     <template v-else><label>{{ mode==='cash'?'付款账户':'转出账户' }}<select v-model="fromAccount" required><option disabled value="">选择账户</option><option v-for="a in mode==='cash'?cashAccounts:accounts" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
      <label v-if="mode!=='cash'">转出币种<select v-model="fromCoin" required><option disabled value="">暂无可转出持仓</option><option v-for="c in sourceCoins" :key="c.id" :value="c.id">{{ c.symbol }} · {{ c.name }}</option></select></label>
      <div class="available"><span>可用 {{ available }} {{ mode==='cash'?'CNY':symbol(fromCoin) }}</span><button v-if="fromCoin && mode!=='cash'" type="button" @click="amount=available">全部</button></div>
     </template>
     <label class="amount-label">{{ mode==='opening'?'持有数量':mode==='cash'?'实际付款（元）':`转出数量（${symbol(fromCoin)}）` }}<input v-model="amount" required inputmode="decimal" autocomplete="off" placeholder="0" aria-label="转出金额或数量" class="amount-input" /></label>
    </section>
    <section class="cy-panel">
     <label v-if="mode!=='opening'">{{ mode==='redeem'?'收款账户':'转入账户' }}<select v-model="toAccount" required><option disabled value="">选择账户</option><option v-for="a in destinationAccounts" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
     <label v-if="mode!=='redeem'">{{ mode==='opening'?'币种':'转入币种' }}<select v-model="toCoin" required :disabled="mode==='transfer'"><option v-for="c in targetCoins" :key="c.id" :value="c.id">{{ c.symbol }} · {{ c.name }}</option></select></label>
     <template v-if="needsQuote"><label>实际到账（{{ mode==='redeem'?'元':symbol(toCoin) }}）<input v-model="received" @input="receivedEdited=true" required inputmode="decimal" autocomplete="off" placeholder="填写实收，或使用参考换算" aria-label="实际到账" /></label>
      <div class="quote-line"><span v-if="quoteLoading" role="status">正在获取参考行情…</span><span v-else-if="quote">1 {{ mode==='cash'?'CNY':symbol(fromCoin) }} ≈ {{ rate }} {{ mode==='redeem'?'CNY':symbol(toCoin) }}</span><span v-else>可直接填写实际到账</span><button type="button" :disabled="quoteLoading || !amount" @click="refreshQuote">参考换算</button></div>
      <p v-if="quoteError" class="hint" role="status">行情暂不可用，仍可按实际成交记账。</p><p v-else-if="quote" class="hint">{{ quoteTime }} · 参考换算，可按实收修改。</p>
      <p v-if="mode==='coin' && !quote" class="hint">没有人民币参考汇率时，相关成本与盈亏保持未知。</p>
     </template>
     <p v-else-if="mode==='transfer'" class="hint">到账 {{ amount || '0' }} {{ symbol(fromCoin) }}（无手续费转移）</p>
     <details v-if="mode!=='transfer' && mode!=='redeem'" class="more-coins"><summary>查找其他币种</summary><CryptoCoinSearch :instruments="coins" @selected="addCoin" /></details>
    </section>
    <p v-if="emptyAccount" class="hint">{{ emptyAccount }} <f7-link :href="emptyAccountLink">添加账户</f7-link></p>
    <section class="cy-panel"><label>备注<input v-model="note" maxlength="300" placeholder="可留空" /></label><p v-if="mode==='opening'" class="hint">已有持仓不扣资金账户；未录入成本时不计算盈亏。</p></section>
   </fieldset><p v-if="error" ref="errorElement" class="error" role="alert">{{ error }}</p><button class="primary" :disabled="busy || !initialized || !!emptyAccount">{{ busy?'正在保存…':'保存记账' }}</button></form>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, watch, onUnmounted, nextTick } from 'vue';
import { f7 } from 'framework7-vue';
import type { Router } from 'framework7/types';
import moment from 'moment-timezone';
import { investments, investmentError } from '@/lib/investments.ts';
import { isCryptoAccount } from '@/lib/crypto-platforms.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import { cryptoInput, heldCryptoCoins, defaultCryptoCoin, receivedAfterQuote, buildCryptoEvent, type CryptoMode } from '@/lib/crypto-entry.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import type { InvestmentAccount, Instrument, WealthSummary, InvestmentConversion, InvestmentEvent } from '@/models/investment.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
import CryptoCoinSearch from '@/components/mobile/CryptoCoinSearch.vue';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const mode=computed<CryptoMode>(()=>{const value=props.f7route.query['mode'] || '';return ['cash','coin','redeem','transfer','opening'].includes(value)?value as CryptoMode:'cash';});
const title=computed(()=>({cash:'人民币买币',coin:'币币兑换',redeem:'卖币到账',transfer:'转移币种',opening:'录入已有持仓'}[mode.value]));
const needsQuote=computed(()=>['cash','coin','redeem'].includes(mode.value));
const accounts=ref<InvestmentAccount[]>([]), coins=ref<Instrument[]>([]), summary=ref<WealthSummary>();
const fromAccount=ref(''), toAccount=ref(''), fromCoin=ref(''), toCoin=ref('crypto:tether'), amount=ref(''), received=ref(''), bookId=ref(''), note=ref(''), zone=ref('Asia/Shanghai');
const quote=ref<InvestmentConversion>(), receivedEdited=ref(false), busy=ref(false), quoteLoading=ref(false), error=ref(''), loadError=ref(''), quoteError=ref(''), errorElement=ref<HTMLElement>(), initialized=ref(false);
const books=useBooksStore(); let active=true, sequence=0, timer:ReturnType<typeof setTimeout>|undefined, pending:InvestmentEvent|null=null, requestKey='', pendingSignature='';
const cashAccounts=computed(()=>summary.value?.cashAccounts.filter(a=>a.currency==='CNY') || []);
const sourceCoins=computed(()=>heldCryptoCoins(coins.value,summary.value?.positions || [],fromAccount.value));
const destinationAccounts=computed(()=>mode.value==='redeem'?cashAccounts.value:accounts.value.filter(a=>mode.value!=='transfer'||a.id!==fromAccount.value));
const targetCoins=computed(()=>mode.value==='coin'?coins.value.filter(c=>c.id!==fromCoin.value):coins.value);
const available=computed(()=>mode.value==='cash'?cashAccounts.value.find(a=>a.id===fromAccount.value)?.balance || '0':summary.value?.positions.find(p=>p.accountId===fromAccount.value&&p.instrumentId===fromCoin.value)?.quantity || '0');
const emptyAccount=computed(()=>!initialized.value?'':!accounts.value.length?'先添加一个加密账户。':mode.value==='transfer'&&!destinationAccounts.value.length?'先添加另一个钱包或交易所账户。':['cash','redeem'].includes(mode.value)&&!cashAccounts.value.length?'需要一个人民币资金账户。':'');
const emptyAccountLink=computed(()=>!accounts.value.length||mode.value==='transfer'?'/crypto/add?kind=WALLET':'/account/add');
const rate=computed(()=>quote.value?new LedgerDecimal(quote.value.fromPrice).div(quote.value.toPrice).toSignificantDigits(10).toFixed():'—');
const quoteTime=computed(()=>quote.value?moment.unix(quote.value.observedAt).tz(zone.value).format('HH:mm:ss'):'');
function symbol(id:string):string{return coins.value.find(c=>c.id===id)?.symbol || '—';}
function addCoin(coin:Instrument):void { if(!coins.value.some(c=>c.id===coin.id)) coins.value.push(coin); toCoin.value=coin.id; }
async function load():Promise<void> {
 active=true; busy.value=true; loadError.value='';
 try {
  const [a,c,s,t]=await Promise.all([investments.accounts(),investments.instruments(),investments.summary(),investments.settings(),books.loadBooks()]);
  accounts.value=a.filter(x=>isCryptoAccount(x.kind)); coins.value=c.filter(x=>x.type==='CRYPTO'); summary.value=s; zone.value=t.timeZone || zone.value;
  if(!initialized.value){
   bookId.value=books.defaultBookId;
   const id=props.f7route.query['accountId'] || accounts.value[0]?.id || '', instrument=props.f7route.query['instrumentId'] || '';
   fromAccount.value=mode.value==='cash'?cashAccounts.value[0]?.id || '':id;
   fromCoin.value=defaultCryptoCoin(heldCryptoCoins(coins.value,s.positions,id),['redeem','transfer'].includes(mode.value)?instrument:'');
   toAccount.value=mode.value==='redeem'?cashAccounts.value[0]?.id || '':mode.value==='transfer'?accounts.value.find(x=>x.id!==id)?.id || '':id;
   toCoin.value=mode.value==='transfer'?fromCoin.value:instrument || (mode.value==='coin'?'crypto:bitcoin':'crypto:tether');
  } else {
   if(mode.value==='cash'&&!fromAccount.value)fromAccount.value=cashAccounts.value[0]?.id || '';
   if(!toAccount.value)toAccount.value=destinationAccounts.value[0]?.id || '';
   fromCoin.value=defaultCryptoCoin(sourceCoins.value,fromCoin.value);
  }
  await nextTick(); initialized.value=true;
 } catch(cause) { loadError.value=investmentError(cause); } finally { busy.value=false; }
}
watch(fromAccount,()=>{
 if(!initialized.value)return; fromCoin.value=defaultCryptoCoin(sourceCoins.value,fromCoin.value);
 if(mode.value==='transfer'&&toAccount.value===fromAccount.value)toAccount.value=destinationAccounts.value[0]?.id || '';
});
watch([amount,fromCoin,toCoin,fromAccount,toAccount],()=>{
 quote.value=undefined; quoteError.value=''; sequence++; quoteLoading.value=false; clearTimeout(timer);
 if(!receivedEdited.value)received.value='';
 if(mode.value==='transfer'&&toCoin.value!==fromCoin.value)toCoin.value=fromCoin.value;
 if(mode.value==='coin'&&toCoin.value===fromCoin.value)toCoin.value=targetCoins.value[0]?.id || '';
 try { cryptoInput(amount.value,mode.value==='cash'?2:18); if(active&&needsQuote.value)timer=setTimeout(()=>void refreshQuote(),650); } catch { /* Wait for a complete input before requesting a quote. */ }
});
watch([fromCoin,toCoin],()=>{receivedEdited.value=false; received.value='';});
async function refreshQuote():Promise<void> {
 clearTimeout(timer); const version=++sequence; quote.value=undefined; quoteError.value='';
 if(!needsQuote.value || !amount.value || busy.value)return;
 quoteLoading.value=true;
 try {
  const result=await investments.conversion({fromInstrumentId:mode.value==='cash'?'':fromCoin.value,toInstrumentId:mode.value==='redeem'?'':toCoin.value,fromQuantity:cryptoInput(amount.value,mode.value==='cash'?2:18)});
  if(version===sequence && active){quote.value=result;received.value=receivedAfterQuote(received.value,receivedEdited.value,result.toQuantity);}
 } catch(cause){if(version===sequence && active)quoteError.value=investmentError(cause);}
 finally{if(version===sequence)quoteLoading.value=false;}
}
async function save():Promise<void> {
 if(busy.value || !initialized.value)return; busy.value=true; error.value=''; clearTimeout(timer); sequence++; quoteLoading.value=false;
 try {
  const draft={mode:mode.value,fromAccount:fromAccount.value,toAccount:toAccount.value,fromCoin:fromCoin.value,toCoin:toCoin.value,amount:amount.value,received:received.value,bookId:bookId.value,note:note.value};
  const signature=JSON.stringify(draft);
  if(!pending || signature!==pendingSignature){
   pending=buildCryptoEvent(draft,quote.value,Math.floor(Date.now()/1000)); pendingSignature=signature; requestKey=generateRandomUUID();
  }
  if(mode.value!=='opening' && new LedgerDecimal(pending.type==='BUY'?pending.amount:pending.quantity).gt(available.value))throw Error('转出金额或数量超过当前可用余额');
  await investments.saveEvent(pending,requestKey);
  useTransactionsStore().updateStoreInvalidState({transactionList:true,accountList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});
  if(active){f7.toast.create({text:'已保存记账',closeTimeout:1800}).open();props.f7router.back();}
 } catch(cause){
  error.value=investmentError(cause);
  if(error.value.includes('行情已过期')){pending=null;quote.value=undefined;}
  await nextTick(); errorElement.value?.scrollIntoView({block:'center'});
 } finally {busy.value=false;}
}
function leave():void{active=false;sequence++;clearTimeout(timer);quoteLoading.value=false;}
onUnmounted(leave);
</script>
<style scoped>
.crypto-convert{max-width:600px;margin:auto;padding-bottom:40px}fieldset{border:0;margin:0;padding:0;min-width:0}
label{display:grid;gap:8px;font-size:13px;margin:12px 0}label:first-child{margin-top:0}label:last-child{margin-bottom:0}
input,select{box-sizing:border-box;width:100%;min-height:46px;border:1px solid var(--cy-line);border-radius:9px;background:var(--cy-card);color:inherit;font:inherit;padding:11px}
.amount-label{margin-top:17px}.amount-input{font-size:28px;border:0;border-bottom:1px solid var(--cy-line);border-radius:0;padding:12px 0;font-variant-numeric:tabular-nums}
.available,.quote-line{display:flex;justify-content:space-between;align-items:center;gap:10px;font-size:12px;color:var(--cy-muted);overflow-wrap:anywhere}
.available button,.quote-line button{flex-shrink:0;border:0;background:none;color:var(--cy-accent);min-height:44px;padding:5px 8px;font:inherit}
.hint{font-size:12px;color:var(--cy-muted);line-height:1.7;margin:10px 0!important}.hint a{color:var(--cy-accent)}
.error{color:var(--cy-expense);font-size:13px;padding:12px 0;line-height:1.7}.primary{width:100%;min-height:48px;border:0;border-radius:11px;background:var(--cy-accent);color:white;padding:14px;font:inherit}
.primary:disabled{opacity:.5}.more-coins{margin-top:14px;font-size:12px;color:var(--cy-accent)}.more-coins summary{cursor:pointer;padding:8px 0}
</style>
