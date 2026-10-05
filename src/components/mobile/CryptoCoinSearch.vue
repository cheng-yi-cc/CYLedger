<template>
    <details class="crypto-search">
        <summary>搜索更多加密货币</summary>
        <div class="search-row"><input v-model="query" aria-label="搜索加密货币" placeholder="名称或代码，例如 BTC" maxlength="64" @keydown.enter.prevent="search" /><button type="button" :disabled="busy || query.trim().length < 2" @click="search">搜索</button></div>
        <p v-if="error" role="alert">{{ error }}</p>
        <p v-if="searched && !busy && !results.length && !error">没有找到匹配币种。</p>
        <button v-for="item in results" :key="item.provider+item.providerId" class="result" type="button" :disabled="busy" @click="choose(item)"><span><strong>{{ item.symbol }}</strong> {{ item.name }}</span><small>{{ item.providerId }}</small><span>添加</span></button>
    </details>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { investments, investmentError } from '@/lib/investments.ts';
import type { Instrument, InstrumentCandidate } from '@/models/investment.ts';
const props=defineProps<{ instruments: Instrument[] }>();
const emit=defineEmits<{ selected:[item:Instrument] }>();
const query=ref(''),busy=ref(false),error=ref(''),searched=ref(false),results=ref<InstrumentCandidate[]>([]);
async function search():Promise<void>{if(busy.value)return;busy.value=true;error.value='';searched.value=true;try{results.value=(await investments.searchInstruments(query.value.trim(),'CRYPTO')).filter(i=>i.type==='CRYPTO');}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
async function choose(item:InstrumentCandidate):Promise<void>{busy.value=true;error.value='';try{const existing=props.instruments.find(i=>(i.provider===item.provider&&i.providerId===item.providerId)||i.id===`crypto:${item.providerId}`||({'BTC':'crypto:bitcoin','ETH':'crypto:ethereum','SOL':'crypto:solana','USDT':'crypto:tether','USDC':'crypto:usd-coin'} as Record<string,string>)[item.providerId.replace(/-USD$/,'')]===i.id);const saved=existing||await investments.createInstrument(item);emit('selected',saved);results.value=[];query.value='';searched.value=false;}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
</script>
<style scoped>
.crypto-search{margin-top:18px;font-size:13px}.crypto-search summary{color:var(--cy-accent);cursor:pointer;min-height:32px}.search-row{display:flex;gap:8px;margin:12px 0}.search-row input{min-width:0;flex:1}.crypto-search input,.crypto-search button{font:inherit;border:1px solid var(--cy-line);border-radius:9px;padding:12px;background:var(--cy-card);color:inherit}.result{display:flex;width:100%;align-items:center;gap:8px;text-align:left;margin:7px 0}.result>span:first-child{flex:1}.result small{font-size:10px;color:var(--cy-muted);max-width:30%;overflow-wrap:anywhere}.result>span:last-child{color:var(--cy-accent)}
</style>
