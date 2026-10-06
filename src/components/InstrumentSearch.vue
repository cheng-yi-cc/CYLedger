<template>
    <section class="cy-instrument-search" aria-label="搜索并绑定行情资产">
        <h3>{{ instrumentId ? '为此资产绑定行情' : '搜索行情资产' }}</h3>
        <p class="cy-muted">确认完整名称、市场和代码后添加。不同市场的同名代码分别保存。</p>
        <form class="cy-search-bar" @submit.prevent="search">
            <select v-model="market" aria-label="搜索市场"><option value="CRYPTO">加密货币</option><option value="CN">A 股 / 场内基金</option><option value="CN_FUND">公募基金</option><option value="HK">港股</option><option value="US">美股</option></select>
            <input v-model="query" required maxlength="64" placeholder="名称或代码，例如 BTC" aria-label="资产名称或代码" />
            <button class="cy-button" :disabled="loading || saving">{{ loading ? '搜索中…' : '搜索' }}</button>
        </form>
        <p v-if="error" role="alert" class="cy-message">{{ error }}</p>
        <p v-if="searched && !loading && !results.length && !error" class="cy-muted">没有匹配结果，可换用代码搜索或手动创建资产。</p>
        <div class="cy-search-results"><button v-for="item in results" :key="`${item.provider}:${item.providerId}`" class="cy-search-result" :aria-pressed="selected?.providerId === item.providerId && selected?.provider === item.provider" @click="selected=item"><strong>{{ item.name }} <small>{{ item.symbol }}</small></strong><span>{{ item.market }} · {{ item.currency }} · {{ item.provider }} · {{ item.providerId }}</span></button></div>
        <form v-if="selected" class="cy-binding-confirm" @submit.prevent="save">
            <p>确认选择：<strong>{{ selected.name }}</strong> · {{ selected.market }} · {{ selected.providerId }}</p>
            <label v-if="selected.type === 'STOCK' || selected.type === 'FUND'">资产分类<select v-model="selected.type"><option value="STOCK">股票</option><option value="FUND">基金</option></select></label>
            <p class="cy-muted">股票使用公开参考价，公募基金使用已公布净值；更新时间和汇率会分别展示。手动价格仍优先使用。</p>
            <button class="cy-button cy-primary" :disabled="saving">{{ saving ? '正在确认…' : instrumentId ? '确认绑定行情' : '确认添加资产' }}</button>
        </form>
    </section>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { investments, investmentError } from '@/lib/investments.ts';
import type { Instrument } from '@/models/investment.ts';
const props = defineProps<{ instrumentId?: string; initialMarket?:string }>();
const emit = defineEmits<{ saved: [instrument: Instrument] }>();
type SearchResult = Awaited<ReturnType<typeof investments.searchInstruments>>[number];
const query=ref(''),market=ref(props.initialMarket||'CRYPTO'),loading=ref(false),saving=ref(false),searched=ref(false),error=ref('');
const results=ref<SearchResult[]>([]),selected=ref<SearchResult|null>(null);
let searchVersion=0;
async function search(): Promise<void> { if (!query.value.trim()) return; const version=++searchVersion; loading.value=true; error.value=''; selected.value=null; searched.value=true; try { const found=await investments.searchInstruments(query.value.trim(),market.value); if(version===searchVersion) results.value=found; } catch(cause) { if(version===searchVersion) {error.value=investmentError(cause);results.value=[];} } finally {if(version===searchVersion)loading.value=false;} }
async function save(): Promise<void> { if (!selected.value || saving.value) return; saving.value=true;error.value='';try { const item=selected.value; const result=props.instrumentId ? await investments.bindInstrument(props.instrumentId,item) : await investments.createInstrument(item); selected.value=null;emit('saved',result); } catch(cause) {error.value=investmentError(cause);}finally{saving.value=false;} }
</script>
<style scoped>
.cy-instrument-search h3{margin:0 0 10px}.cy-search-bar{display:flex;flex-wrap:wrap;gap:8px;margin:14px 0}.cy-search-bar input{flex:1;min-width:140px}.cy-search-bar select{width:100%}.cy-search-bar input,.cy-search-bar select,.cy-binding-confirm select{padding:10px;border:1px solid var(--cy-line,#ddd);border-radius:8px;background:var(--cy-card,var(--cy-surface));color:inherit;font:inherit}.cy-search-results{max-height:330px;overflow:auto}.cy-search-result{display:flex!important;flex-direction:column;align-items:flex-start!important;text-align:left;width:100%!important;padding:12px!important;border:0;border-bottom:1px solid var(--cy-line,#ddd);background:transparent;color:inherit;gap:4px}.cy-search-result span,.cy-search-result small{font-size:11px;color:var(--cy-muted)}.cy-search-result[aria-pressed=true]{background:var(--cy-soft);box-shadow:inset 3px 0 var(--cy-accent,var(--cy-teal))}.cy-binding-confirm{padding:15px 0;display:grid;gap:12px}.cy-binding-confirm p{line-height:1.7;font-size:13px}
</style>
