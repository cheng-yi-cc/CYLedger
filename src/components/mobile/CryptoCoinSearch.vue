<template>
    <details class="crypto-search">
        <summary>搜索更多加密货币</summary>
        <div class="search-row"><input v-model="query" aria-label="搜索加密货币" placeholder="名称或代码，例如 BTC" maxlength="64" @keydown.enter.prevent="search" /><button type="button" :disabled="busy || query.trim().length < 2" @click="search">搜索</button></div>
        <p v-if="error" role="alert">{{ error }}</p>
        <p v-if="searched && !busy && !results.length && !error">行情源中没有找到匹配币种，可在下方手动添加。</p>
        <button v-for="item in results" :key="item.provider+item.providerId" class="result" type="button" :disabled="busy" @click="choose(item)"><span><strong>{{ item.symbol }}</strong> {{ item.name }}</span><small>{{ item.providerId }}</small><span>添加</span></button>
        <button class="manual-toggle" type="button" :disabled="busy" :aria-expanded="manualOpen" @click="manualOpen = !manualOpen">{{ manualOpen ? '收起手动添加' : '找不到币种？手动添加' }}</button>
        <div v-if="manualOpen" class="manual-editor">
            <p class="hint">手动币种仅保存在你的账本，不自动获取行情。可先记录数量，再到币种详情填写每枚美元或人民币价格；未填价格时显示“暂未估值”。</p>
            <label>币种名称<input v-model="manualName" aria-label="手动币种名称" maxlength="64" placeholder="例如 Fiat24 USD（Arbitrum）" :disabled="busy" @keydown.enter.prevent="createManual" /></label>
            <label>显示代码<input v-model="manualSymbol" aria-label="手动币种代码" maxlength="24" placeholder="例如 USD24" :disabled="busy" @keydown.enter.prevent="createManual" /></label>
            <p class="hint">名称和代码请核对钱包中的网络与合约。同名币种不会自动合并。</p>
            <button class="manual-save" type="button" :disabled="busy || !manualName.trim() || !manualSymbol.trim()" @click="createManual">{{ busy ? '正在添加…' : '添加手动币种' }}</button>
        </div>
    </details>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { investments, investmentError } from '@/lib/investments.ts';
import type { Instrument, InstrumentCandidate } from '@/models/investment.ts';
const props=defineProps<{ instruments: Instrument[] }>();
const emit=defineEmits<{ selected:[item:Instrument] }>();
const query=ref(''),busy=ref(false),error=ref(''),searched=ref(false),results=ref<InstrumentCandidate[]>([]);
const manualOpen=ref(false),manualName=ref(''),manualSymbol=ref('');
async function search():Promise<void>{if(busy.value)return;busy.value=true;error.value='';searched.value=true;try{results.value=(await investments.searchInstruments(query.value.trim(),'CRYPTO')).filter(i=>i.type==='CRYPTO');}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
async function choose(item:InstrumentCandidate):Promise<void>{busy.value=true;error.value='';try{const existing=props.instruments.find(i=>(i.provider===item.provider&&i.providerId===item.providerId)||i.id===`crypto:${item.providerId}`||({'BTC':'crypto:bitcoin','ETH':'crypto:ethereum','SOL':'crypto:solana','USDT':'crypto:tether','USDC':'crypto:usd-coin'} as Record<string,string>)[item.providerId.replace(/-USD$/,'')]===i.id);const saved=existing||await investments.createInstrument(item);emit('selected',saved);results.value=[];query.value='';searched.value=false;}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
async function createManual():Promise<void>{
    if(busy.value || !manualName.value.trim() || !manualSymbol.value.trim())return;
    busy.value=true;error.value='';
    try{
        const saved=await investments.createInstrument({name:manualName.value.trim(),symbol:manualSymbol.value.trim(),type:'CRYPTO'});
        emit('selected',saved);
        manualName.value='';manualSymbol.value='';manualOpen.value=false;results.value=[];query.value='';searched.value=false;
    }catch(e){error.value=investmentError(e);}finally{busy.value=false;}
}
</script>
<style scoped>
.crypto-search{margin-top:18px;font-size:13px}.crypto-search summary{color:var(--cy-accent);cursor:pointer;min-height:32px}.search-row{display:flex;gap:8px;margin:12px 0}.search-row input{min-width:0;flex:1}.crypto-search input,.crypto-search button{font:inherit;border:1px solid var(--cy-line);border-radius:9px;padding:12px;background:var(--cy-card);color:inherit}.result{display:flex;width:100%;align-items:center;gap:8px;text-align:left;margin:7px 0}.result>span:first-child{flex:1}.result small{font-size:10px;color:var(--cy-muted);max-width:30%;overflow-wrap:anywhere}.result>span:last-child{color:var(--cy-accent)}
.crypto-search .manual-toggle{width:100%;margin-top:8px;color:var(--cy-accent)}.manual-editor{margin-top:14px;padding-top:4px;border-top:1px solid var(--cy-line)}.manual-editor label{display:grid;gap:8px;margin:16px 0}.manual-editor input{box-sizing:border-box;width:100%;min-width:0}.manual-editor .hint{color:var(--cy-muted);font-size:12px;line-height:1.8}.manual-editor .manual-save{width:100%;color:var(--cy-accent)}.crypto-search button:disabled{opacity:.5}
</style>
