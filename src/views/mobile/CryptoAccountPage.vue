<template>
 <f7-page class="cy-main-page cy-mobile-surface" @page:beforein="load">
  <f7-navbar :title="editId?'编辑账户':kind==='WALLET'?'添加加密钱包':'添加加密货币交易所'" back-link="返回" />
  <main class="cy-page-body crypto-page">
   <p v-if="error" class="crypto-error" role="alert">{{ error }}</p>
   <form @submit.prevent="save"><fieldset :disabled="busy">
    <section class="cy-panel"><h2>{{ kind==='WALLET'?'选择钱包':'选择交易所' }}</h2><div class="platform-grid"><button v-for="p in options" :key="p.id" type="button" :aria-pressed="platform===p.id" @click="selectPlatform(p)"><img :src="platformIcon(p.id)" alt="" /><span>{{ p.name }}</span><span v-if="platform===p.id" class="chosen">✓</span></button></div><label>账户名称<input v-model="name" required maxlength="64" placeholder="可自定义名称" /></label></section>
    <section class="cy-panel"><h2>持有的加密货币</h2><p v-if="editId" class="hint">持有中的币种保留；数量在交易记录中修改。</p><p v-else class="hint">选择币种，已有数量可填，没有可留空。</p><div v-for="coin in coins" :key="coin.id" class="coin-row"><label class="coin-choice"><input v-model="selected" type="checkbox" :value="coin.id" :disabled="held.includes(coin.id)" /><span><strong>{{ coin.symbol }}</strong><small>{{ coin.name }}</small></span></label><input v-if="!editId && selected.includes(coin.id)" v-model="quantities[coin.id]" class="quantity" inputmode="decimal" :aria-label="`${coin.symbol} 持有数量`" placeholder="持有数量（可留空）" /></div><CryptoCoinSearch :instruments="coins" @selected="addCoin" /></section>
    <section v-if="!editId" class="cy-panel"><BookPicker v-model="bookId" /><p class="hint">已有持仓不扣其他账户余额；未录入成本时不计算盈亏。</p></section>
    <button class="primary" :disabled="!platform || !name.trim() || busy">{{ busy?'正在保存…':editId?'保存账户':'保存账户与持仓' }}</button>
   </fieldset></form>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, reactive } from 'vue';
import type { Router } from 'framework7/types';
import { investments, investmentError } from '@/lib/investments.ts';
import {cryptoInput} from '@/lib/crypto-entry.ts';
import {LedgerDecimal} from '@/lib/ledger-display.ts';
import { cryptoPlatforms, platformIcon, type CryptoPlatform } from '@/lib/crypto-platforms.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import type { Instrument } from '@/models/investment.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
import CryptoCoinSearch from '@/components/mobile/CryptoCoinSearch.vue';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const editId=computed(()=>props.f7route.query['id']||''),held=ref<string[]>([]);
const kind=computed(()=>props.f7route.query['kind']==='WALLET'?'WALLET':'EXCHANGE');
const options=computed(()=>cryptoPlatforms.filter(p=>p.kind===kind.value));
const platform=ref(''),name=ref(''),coins=ref<Instrument[]>([]),selected=ref<string[]>([]),quantities=reactive<Record<string,string>>({}),error=ref(''),busy=ref(false),bookId=ref('');
const books=useBooksStore();let requestKey=generateRandomUUID(),lastPayload='';
function selectPlatform(p:CryptoPlatform):void{const previous=cryptoPlatforms.find(x=>x.id===platform.value)?.name;if(!name.value||name.value===previous)name.value=p.name;platform.value=p.id;}
function addCoin(coin:Instrument):void{if(!coins.value.some(c=>c.id===coin.id))coins.value.push(coin);if(!selected.value.includes(coin.id))selected.value.push(coin.id);}
async function load():Promise<void>{busy.value=true;error.value='';try{const [items]=await Promise.all([investments.instruments(),books.loadBooks()]);coins.value=items.filter(i=>i.type==='CRYPTO');bookId.value ||= books.defaultBookId;if(editId.value){const[a,s]=await Promise.all([investments.accounts(),investments.summary()]);const existing=a.find(a=>a.id===editId.value);if(!existing || existing.kind!==kind.value)throw Error('找不到这个账户');platform.value=existing.platform||'';name.value=existing.name;held.value=s.positions.filter(p=>p.accountId===editId.value && new LedgerDecimal(p.quantity).gt(0)).map(p=>p.instrumentId);selected.value=[...new Set([...(existing.instruments||[]),...held.value])];}}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
async function save():Promise<void>{if(busy.value)return;busy.value=true;error.value='';try{const input={name:name.value,kind:kind.value,platform:platform.value,bookId:bookId.value,holdings:selected.value.map(instrumentId=>({instrumentId,quantity:!quantities[instrumentId]?.trim()||/^0+(\.0+)?$/.test(quantities[instrumentId]!.trim())?'0':cryptoInput(quantities[instrumentId]!)}))};const payload=JSON.stringify(input);if(payload!==lastPayload){requestKey=generateRandomUUID();lastPayload=payload;}const account=editId.value?await investments.updateAccount({id:editId.value,name:name.value,kind:kind.value,platform:platform.value,instruments:selected.value}):await investments.createCryptoAccount(input,requestKey);useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true});if(editId.value)props.f7router.back();else props.f7router.navigate(`/crypto/account?id=${encodeURIComponent(account.id)}`,{reloadCurrent:true});}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
</script>
<style scoped>
.crypto-page{max-width:640px;margin:auto;padding-bottom:40px}fieldset{border:0;margin:0;padding:0;min-width:0}h2{font-size:17px;margin:0 0 17px}.platform-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}.platform-grid button{display:flex;align-items:center;gap:9px;min-height:62px;padding:10px;text-align:left;border:1px solid var(--cy-line);background:var(--cy-card);color:inherit;border-radius:12px;font:inherit;font-size:13px}.platform-grid button[aria-pressed=true]{border-color:var(--cy-accent);background:var(--cy-soft)}.platform-grid img{width:32px;height:32px;border-radius:8px;flex-shrink:0}.chosen{margin-left:auto;color:var(--cy-accent)}label:not(.coin-choice){display:grid;gap:10px;margin-top:20px;font-size:13px}input:not([type=checkbox]){box-sizing:border-box;width:100%;min-height:46px;padding:12px;border:1px solid var(--cy-line);border-radius:10px;background:var(--cy-card);color:inherit;font:inherit}.hint{font-size:12px;color:var(--cy-muted);line-height:1.8}.coin-row{display:flex;gap:12px;align-items:center;min-height:70px;border-bottom:1px solid var(--cy-line);padding:10px 0}.coin-choice{display:flex;align-items:center;gap:10px;flex:1;min-width:0}.coin-choice input{width:18px;height:18px;accent-color:var(--cy-accent)}.coin-choice strong{font-size:15px}.coin-choice small{display:block;color:var(--cy-muted);font-size:11px;overflow-wrap:anywhere;margin-top:5px}input.quantity{width:52%;font-size:13px}.primary{width:100%;padding:15px;border:0;border-radius:12px;background:var(--cy-accent);color:white;font:inherit}.primary:disabled{opacity:.5}.crypto-error{padding:12px;color:var(--cy-expense);font-size:13px}
</style>
